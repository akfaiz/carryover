package mobile

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/rclone/rclone/fs/config"
	"github.com/rclone/rclone/lib/oauthutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/akfaiz/carryover/internal/app/appdir"
	"github.com/akfaiz/carryover/internal/cloudsync/cloud"
	"github.com/akfaiz/carryover/internal/cloudsync/localfs"
	"github.com/akfaiz/carryover/internal/cloudsync/syncer"
	"github.com/akfaiz/carryover/internal/core"
	"github.com/akfaiz/carryover/internal/library"
	"github.com/akfaiz/carryover/services"
)

type fakeHost struct {
	mu      sync.Mutex
	urls    []string
	events  map[string][]string
	onEvent func(kind, payload string) // also called for each event, if set
}

func (h *fakeHost) OpenURL(url string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.urls = append(h.urls, url)
}

func (h *fakeHost) Event(kind, payload string) {
	h.mu.Lock()
	if h.events == nil {
		h.events = map[string][]string{}
	}
	h.events[kind] = append(h.events[kind], payload)
	on := h.onEvent
	h.mu.Unlock()
	if on != nil {
		on(kind, payload)
	}
}

func (h *fakeHost) setOnEvent(f func(kind, payload string)) {
	h.mu.Lock()
	h.onEvent = f
	h.mu.Unlock()
}

func (h *fakeHost) count(kind string) int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.events[kind])
}

// decode returns a function that checks a facade call's error and decodes
// its JSON: decode[T](t)(Call()).
func decode[T any](t *testing.T) func(string, error) T {
	return func(s string, err error) T {
		t.Helper()
		require.NoError(t, err)
		var v T
		require.NoError(t, json.Unmarshal([]byte(s), &v), s)
		return v
	}
}

// The facade runs a whole first-use flow: open a fresh data dir, point it at a
// cloud folder, add a game and push its saves; the catalog reaches the cloud
// and events reach the host. Init is once per process, so it is one test.
func TestFlow(t *testing.T) {
	root := t.TempDir()
	data, cache, home := filepath.Join(root, "data"), filepath.Join(root, "cache"), filepath.Join(root, "home")
	for _, k := range []string{appdir.EnvHome, "TMPDIR", "XDG_CACHE_HOME", "XDG_CONFIG_HOME", "HOME"} {
		t.Setenv(k, os.Getenv(k)) // restored after the test; Init sets them
	}
	_, err := Games()
	require.Error(t, err, "calls before Init")

	host := &fakeHost{}
	require.NoError(t, Init(data, cache, home, "Test Phone", host))
	t.Cleanup(func() {
		WaitBackground() // catalog uploads write into data
		closeLog()       // Windows can't remove an open file
	})
	require.NoError(t, Init(data, cache, home, "Other", host), "Init again with the same dir")
	require.Error(t, Init(filepath.Join(root, "other"), cache, home, "", host), "Init with another dir")

	st := decode[appdir.Settings](t)(Settings())
	assert.Equal(t, "Test Phone", st.DeviceName, "first start names the device")
	assert.NotEmpty(t, st.DeviceID)
	assert.DirExists(t, filepath.Join(cache, "tmp"))
	if runtime.GOOS != "windows" { // Windows takes it from TMP
		assert.Equal(t, filepath.Join(cache, "tmp"), os.TempDir())
	}

	// OAuth providers open the authorisation page through the host.
	require.NoError(t, oauthutil.OpenURL("https://example.com/auth"))
	assert.Equal(t, []string{"https://example.com/auth"}, host.urls)

	config.FileSetValue("cloud", "type", "local")
	cloudDir := filepath.ToSlash(filepath.Join(root, "cloud"))
	st.Remote, st.RemoteRoot, st.SyncLibrary = "cloud", cloudDir, true
	st.DeviceID = "changed"
	st = decode[appdir.Settings](t)(SaveSettings(mustJSON(t, st)))
	assert.Equal(t, "cloud", st.Remote)
	require.NoError(t, TestRemote("cloud", cloudDir))

	// The library settings the app toggles: covers kept on this device, and the library not shared.
	st.CoversLocalOnly = true
	assert.True(t, decode[appdir.Settings](t)(SaveSettings(mustJSON(t, st))).CoversLocalOnly)
	st.CoversLocalOnly, st.SyncLibrary = false, false
	shared := decode[appdir.Settings](t)(SaveSettings(mustJSON(t, st)))
	assert.False(t, shared.CoversLocalOnly, "turned on again")
	assert.False(t, shared.SyncLibrary, "not shared")
	st.SyncLibrary = true
	assert.True(t, decode[appdir.Settings](t)(SaveSettings(mustJSON(t, st))).SyncLibrary)

	saves := filepath.Join(home, "RetroArch", "saves")
	require.NoError(t, os.MkdirAll(saves, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(saves, "game.srm"), []byte("sram"), 0o644))
	g := decode[services.GameView](t)(SaveGame(mustJSON(t, services.GameInput{
		Name:   "Game A",
		Slots:  []library.SaveSlot{{Key: "saves"}},
		Device: services.DeviceInput{Paths: map[string]string{"saves": saves}, Exe: "x"},
	})))
	require.True(t, g.Ready, g.SetupHint)
	if runtime.GOOS != "windows" { // Windows takes the home dir from USERPROFILE
		p, err := PreviewPath(g.ID, "{home}/RetroArch/saves")
		require.NoError(t, err)
		assert.Equal(t, saves, p, "{home} is the home dir given to Init")
	}

	plan := decode[services.PlanView](t)(Plan(g.ID))
	assert.Equal(t, "push", plan.Action)
	out := decode[SyncOutcome](t)(Sync(g.ID, ""))
	assert.Equal(t, SyncOutcome{Action: "push"}, out)
	assert.Positive(t, host.count(EventSync), "sync progress events")
	assert.Positive(t, host.count(EventLibrary), "library events")

	all := decode[SyncAllOutcome](t)(SyncAll())
	require.Len(t, all.Results, 1)
	assert.Equal(t, "none", all.Results[0].Action)
	assert.Positive(t, host.count(EventSyncAll))

	games := decode[[]services.GameView](t)(Games())
	require.Len(t, games, 1)
	assert.Equal(t, "none", games[0].Status.LastAction, "SyncAll found nothing to do")
	assert.NotEmpty(t, games[0].Status.LastSyncAt)

	// Backups: none yet; a manual backup is pinned, can be unpinned, restored
	// over changed saves and deleted.
	none := decode[[]syncer.Backup](t)(Backups(g.ID))
	assert.Empty(t, none)
	made := decode[SyncOutcome](t)(BackupNow(g.ID))
	assert.Equal(t, SyncOutcome{Action: "none"}, made)
	listed := decode[[]syncer.Backup](t)(Backups(g.ID))
	require.Len(t, listed, 1)
	assert.Equal(t, "manual", listed[0].Kind)
	assert.True(t, listed[0].Pinned)
	require.NoError(t, SetBackupPinned(g.ID, listed[0].Name, false))
	listed = decode[[]syncer.Backup](t)(Backups(g.ID))
	assert.False(t, listed[0].Pinned)
	require.NoError(t, os.WriteFile(filepath.Join(saves, "game.srm"), []byte("changed"), 0o644))
	undone := decode[SyncOutcome](t)(RestoreBackup(g.ID, listed[0].Name))
	assert.Equal(t, SyncOutcome{Action: "none"}, undone)
	back, err := os.ReadFile(filepath.Join(saves, "game.srm"))
	require.NoError(t, err)
	assert.Equal(t, "sram", string(back), "the backup replaced the changed save")
	_, err = RestoreBackup(g.ID, "nope.zip")
	require.Error(t, err, "unknown backup")
	require.NoError(t, DeleteBackup(g.ID, listed[0].Name))
	for _, b := range decode[[]syncer.Backup](t)(Backups(g.ID)) {
		assert.NotEqual(t, listed[0].Name, b.Name, "deleted")
	}
	_, err = Backups("nope")
	require.NoError(t, err, "a game without backups has none")

	// SetPaths sets one profile's paths and keeps the rest of the game.
	s, err := get()
	require.NoError(t, err)
	emu, err := s.core.Library.SaveEmulator(library.Emulator{Name: "RetroArch"}, s.core.DeviceID())
	require.NoError(t, err)
	g = decode[services.GameView](t)(SaveGame(mustJSON(t, func() services.GameInput {
		in := inputOf(g)
		in.Emulators = []services.GameEmulatorInput{{EmulatorID: emu.ID, Slots: []library.SaveSlot{{Key: "srm"}}}}
		return in
	}())))
	require.Len(t, g.Emulators, 1)
	assert.False(t, g.Emulators[0].SetUp)
	g = decode[services.GameView](t)(SetPaths(g.Emulators[0].Key, mustJSON(t, map[string]string{"srm": saves})))
	require.Len(t, g.Emulators, 1)
	assert.True(t, g.Emulators[0].SetUp)
	assert.Equal(t, saves, g.Emulators[0].ResolvedPaths["srm"])
	assert.Equal(t, saves, g.ResolvedPaths["saves"], "native paths kept")
	assert.Equal(t, "x", g.Device.Exe, "launch config kept")
	g = decode[services.GameView](t)(SetPaths(g.ID, `{"saves":""}`))
	assert.Empty(t, g.Device.Paths["saves"], `"" clears a path`)
	assert.Equal(t, saves, g.Emulators[0].ResolvedPaths["srm"], "emulator paths kept")
	_, err = SetPaths(g.ID+".nope", `{}`)
	require.Error(t, err, "unknown emulator")

	// A play session the app runs: another device holds the cloud lock, so
	// BeginSession asks through a prompt event, answered with Respond.
	key := g.Emulators[0].Key
	lockFile := filepath.Join(root, "cloud", "games", key, "lock.json")
	require.NoError(t, os.MkdirAll(filepath.Dir(lockFile), 0o755))
	require.NoError(t, os.WriteFile(lockFile, []byte(`{"device":"other","deviceName":"PC","since":"`+time.Now().UTC().Format(time.RFC3339)+`"}`), 0o644))
	host.setOnEvent(func(kind, payload string) {
		if kind == EventPrompt {
			var p services.Prompt
			assert.NoError(t, json.Unmarshal([]byte(payload), &p))
			assert.Equal(t, "lock", p.Kind)
			assert.True(t, Respond(p.ID, "yes"))
		}
	})
	begun := decode[BeginOutcome](t)(BeginSession(key, ""))
	host.setOnEvent(nil)
	assert.Equal(t, emu.ID, begun.Emulator)
	assert.Equal(t, key, begun.Key)
	assert.False(t, begun.Cancelled)
	assert.True(t, begun.Synced)
	assert.True(t, SessionOpen(key))
	open := decode[[]core.OpenSession](t)(OpenSessions())
	assert.Equal(t, []core.OpenSession{{CloudKey: key, Key: key, Game: g.ID, Emulator: emu.ID}}, open)
	assert.Equal(t, 1, host.count(EventPrompt))
	assert.Positive(t, host.count(EventStatus))
	games = decode[[]services.GameView](t)(Games())
	assert.True(t, games[0].Emulators[0].Status.Running, "an open session shows as running")
	_, err = Sync(key, "")
	require.Error(t, err, "manual sync while playing")
	_, err = BeginSession(key, "")
	require.Error(t, err, "second start of an open session")

	require.NoError(t, os.WriteFile(filepath.Join(saves, "game2.srm"), []byte("played"), 0o644))
	ended := decode[EndOutcome](t)(EndSession(key))
	require.NotNil(t, ended.Notice)
	assert.Equal(t, "uploaded", string(ended.Notice.Kind))
	assert.False(t, ended.Offline)
	assert.False(t, SessionOpen(key))
	assert.Equal(t, "[]", must(OpenSessions()))
	assert.NoFileExists(t, lockFile, "cloud lock released")

	// Android launch settings live in this device's existing fields.
	ev := decode[services.EmulatorView](t)(SaveEmulator(mustJSON(t, services.EmulatorSetup{
		ID: emu.ID, Name: "RetroArch", Exe: "com.retroarch", Args: "-e ROM {rom}",
	})))
	assert.True(t, ev.SetUp)
	assert.Equal(t, "com.retroarch", ev.Device.Exe)
	g = decode[services.GameView](t)(SetRom(key, " content://x/game.sfc "))
	assert.Equal(t, "content://x/game.sfc", g.Emulators[0].Rom)
	assert.True(t, g.Emulators[0].Installed, "an emulator with an app counts as installed")
	assert.Equal(t, saves, g.Emulators[0].ResolvedPaths["srm"], "paths kept")
	_, err = SetRom(g.ID, "x")
	require.Error(t, err, "the native profile has no game file")
	g = decode[services.GameView](t)(SetNativeApp(g.ID, "com.example.game"))
	assert.Equal(t, "com.example.game", g.Device.Exe)
	assert.Equal(t, "content://x/game.sfc", g.Emulators[0].Rom, "emulator setup kept")

	// Save locations: a new one gets a key from its name, an old one keeps its
	// key and filters, and a removed one takes this device's path with it.
	g = decode[services.GameView](t)(SetSlots(key, mustJSON(t, []library.SaveSlot{
		{Key: "srm", Label: "Battery saves", Include: []string{"*.srm"}},
		{Label: "Save states"},
	})))
	require.Len(t, g.Emulators[0].Slots, 2)
	assert.Equal(t, library.SaveSlot{Key: "srm", Label: "Battery saves", Include: []string{"*.srm"}}, g.Emulators[0].Slots[0])
	assert.Equal(t, "save-states", g.Emulators[0].Slots[1].Key)
	assert.Equal(t, saves, g.Emulators[0].ResolvedPaths["srm"], "paths kept")
	assert.False(t, g.Emulators[0].SetUp && g.Emulators[0].Ready, "the new location has no folder yet")
	g = decode[services.GameView](t)(SetPaths(key, mustJSON(t, map[string]string{"save-states": saves})))
	assert.Equal(t, saves, g.Emulators[0].ResolvedPaths["save-states"])
	g = decode[services.GameView](t)(SetSlots(key, mustJSON(t, []library.SaveSlot{{Key: "save-states"}, {Key: "srm", Label: "SRM"}})))
	assert.Equal(t, []string{"save-states", "srm"}, []string{g.Emulators[0].Slots[0].Key, g.Emulators[0].Slots[1].Key}, "order follows the list")
	g = decode[services.GameView](t)(SetSlots(key, mustJSON(t, []library.SaveSlot{{Key: "srm"}})))
	require.Len(t, g.Emulators[0].Slots, 1)
	assert.NotContains(t, g.Emulators[0].Paths, "save-states", "the removed location's path is gone")
	assert.Equal(t, saves, g.Emulators[0].Paths["srm"])
	for name, slots := range map[string][]library.SaveSlot{
		"empty list":   {},
		"unknown key":  {{Key: "nope"}},
		"new, no name": {{Key: "srm"}, {Label: " "}},
		"listed twice": {{Key: "srm"}, {Key: "srm"}},
	} {
		_, err = SetSlots(key, mustJSON(t, slots))
		require.Error(t, err, name)
	}
	_, err = SetSlots(g.ID+".nope", mustJSON(t, []library.SaveSlot{{Label: "x"}}))
	require.Error(t, err, "unknown emulator")
	g = decode[services.GameView](t)(SetSlots(g.ID, mustJSON(t, []library.SaveSlot{{Key: "saves"}, {Label: "Game state"}})))
	require.Len(t, g.Slots, 2, "the game's own saves can have several too")
	assert.Equal(t, "game-state", g.Slots[1].Key)
	g = decode[services.GameView](t)(SetSlots(g.ID, mustJSON(t, []library.SaveSlot{{Key: "saves"}})))
	require.Len(t, g.Slots, 1)

	// A cover picked on the phone is copied into Carryover's data.
	img := filepath.Join(root, "cover.png")
	require.NoError(t, os.WriteFile(img, []byte("\x89PNG\r\n\x1a\nfake"), 0o644))
	g = decode[services.GameView](t)(SetCover(g.ID, img))
	assert.FileExists(t, g.CoverPath)
	assert.NotEqual(t, img, g.CoverPath, "copied, not referenced")
	_, err = SetCover(g.ID, filepath.Join(root, "cover.gif"))
	require.Error(t, err, "only PNG, JPEG or WebP")

	// Removing the emulator from this device clears only this device's setup.
	require.NoError(t, UnsetEmulator(emu.ID))
	emus := decode[[]services.EmulatorView](t)(Emulators())
	require.Len(t, emus, 1)
	assert.False(t, emus[0].SetUp)
	assert.Empty(t, emus[0].Device.Exe)
	g = decode[services.GameView](t)(Game(g.ID))
	require.Len(t, g.Emulators, 1, "the game keeps its emulator")
	assert.False(t, g.Emulators[0].Installed)
	require.Error(t, UnsetEmulator("nope"))
	ev = decode[services.EmulatorView](t)(SaveEmulator(mustJSON(t, services.EmulatorSetup{
		ID: emu.ID, Name: "RetroArch", Exe: "com.retroarch", Args: "-e ROM {rom}",
	})))
	assert.True(t, ev.SetUp, "set up again")
	prompts := decode[[]services.Prompt](t)(Prompts())
	assert.Empty(t, prompts)

	// A save folder granted through the Storage Access Framework: the app's
	// Storage reaches it, uploads and downloads go through it.
	saf := &fakeStorage{dir: filepath.Join(root, "saf")}
	SetStorage(saf)
	t.Cleanup(func() { SetStorage(nil) })
	safFile := filepath.Join(saf.dir, "auth", "tree%3Ax", "saves", "s.sav")
	require.NoError(t, os.MkdirAll(filepath.Dir(safFile), 0o755))
	require.NoError(t, os.WriteFile(safFile, []byte("from saf"), 0o644))
	g = decode[services.GameView](t)(SetPaths(key, `{"srm":":saf:auth/tree%3Ax/saves"}`))
	assert.Equal(t, ":saf:auth/tree%3Ax/saves", g.Emulators[0].ResolvedPaths["srm"])
	out = decode[SyncOutcome](t)(Sync(key, ""))
	assert.Equal(t, "push", out.Action)
	assert.FileExists(t, filepath.Join(root, "cloud", "games", key, "current", "srm", "s.sav"))
	require.NoError(t, os.Remove(safFile))
	out = decode[SyncOutcome](t)(Sync(key, "cloud"))
	assert.Equal(t, "pull", out.Action)
	b, err := os.ReadFile(safFile)
	require.NoError(t, err)
	assert.Equal(t, "from saf", string(b), "pulled back into the SAF folder")

	// Editing on the phone: another emulator for the game, sharing the first
	// one's saves (Lemon next to Eden), then removed again; a rename.
	lemon := decode[services.EmulatorView](t)(SaveEmulator(mustJSON(t, services.EmulatorSetup{Name: "Lemon", Exe: "dev.lemon"})))
	_, err = AddGameEmulator(g.ID, lemon.ID, "nope")
	require.Error(t, err, "sharing with an emulator the game doesn't have")
	g = decode[services.GameView](t)(AddGameEmulator(g.ID, lemon.ID, emu.ID))
	require.Len(t, g.Emulators, 2)
	assert.Equal(t, emu.ID, g.Emulators[1].ShareWith)
	assert.Equal(t, g.Emulators[0].Slots, g.Emulators[1].Slots, "a sharer has the shared emulator's slots")
	assert.Equal(t, ":saf:auth/tree%3Ax/saves", g.Emulators[0].ResolvedPaths["srm"], "the other emulator's setup kept")
	_, err = AddGameEmulator(g.ID, lemon.ID, "")
	require.Error(t, err, "added twice")
	_, err = RemoveGameEmulator(g.ID, emu.ID)
	require.Error(t, err, "Lemon uses its saves")
	g = decode[services.GameView](t)(RemoveGameEmulator(g.ID, lemon.ID))
	require.Len(t, g.Emulators, 1)
	g = decode[services.GameView](t)(AddGameEmulator(g.ID, lemon.ID, ""))
	assert.Equal(t, []library.SaveSlot{savesSlot}, g.Emulators[1].Slots, "saves of its own")
	g = decode[services.GameView](t)(RenameGame(g.ID, "  Game B "))
	assert.Equal(t, "Game B", g.Name)
	assert.Len(t, g.Emulators, 2, "renaming keeps the emulators")
	_, err = RenameGame(g.ID, " ")
	require.Error(t, err)

	// A game added on the phone: an emulator, its game file and save folder.
	added := decode[services.GameView](t)(AddEmulatorGame("Zelda", lemon.ID, "content://x/zelda.nsp", saves))
	require.Len(t, added.Emulators, 1)
	assert.Empty(t, added.Slots, "no saves of its own")
	assert.Equal(t, "content://x/zelda.nsp", added.Emulators[0].Rom)
	assert.Equal(t, saves, added.Emulators[0].ResolvedPaths["saves"])
	_, err = RemoveGameEmulator(added.ID, lemon.ID)
	require.Error(t, err, "the game's only emulator")
	_, err = AddEmulatorGame(" ", lemon.ID, "", "")
	require.Error(t, err, "no name")
	assert.Equal(t, "Super Mario World", GuessRomName(lemon.ID, "Super Mario World (USA).sfc"))

	// The trash: a deleted game can be restored, or deleted for good with or
	// without its cloud saves; the saves of a game deleted without them are left over.
	tr := decode[TrashView](t)(Trash())
	assert.Equal(t, services.TrashDays, tr.Days)
	assert.Empty(t, tr.Games)
	assert.Empty(t, tr.Emulators)

	names := func() []string {
		var out []string
		for _, g := range decode[[]services.GameView](t)(Games()) {
			out = append(out, g.Name)
		}
		return out
	}
	withSaves := func(name, dirName string) (services.GameView, string) {
		dir := filepath.Join(home, dirName)
		require.NoError(t, os.MkdirAll(dir, 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "s.sav"), []byte(name), 0o644))
		g := decode[services.GameView](t)(AddEmulatorGame(name, lemon.ID, "", dir))
		k := g.Emulators[0].Key
		assert.Equal(t, "push", decode[SyncOutcome](t)(Sync(k, "")).Action)
		cloud := filepath.Join(root, "cloud", "games", k)
		require.DirExists(t, cloud, "its saves are in the cloud")
		return g, cloud
	}

	doomed, doomedCloud := withSaves("Doomed", "doomed")
	require.NoError(t, DeleteGame(doomed.ID))
	assert.NotContains(t, names(), "Doomed", "deleted games leave the library")
	tr = decode[TrashView](t)(Trash())
	require.Len(t, tr.Games, 1)
	assert.Equal(t, doomed.ID, tr.Games[0].ID)
	assert.Equal(t, "Doomed", tr.Games[0].Name)
	assert.Equal(t, 1, tr.Games[0].Emulators)
	assert.Equal(t, services.TrashDays*24, int(tr.Games[0].ExpiresAt.Sub(tr.Games[0].DeletedAt).Hours()))
	assert.DirExists(t, doomedCloud, "deleting keeps the cloud saves")

	require.NoError(t, RestoreGame(doomed.ID))
	assert.Contains(t, names(), "Doomed")
	assert.Empty(t, decode[TrashView](t)(Trash()).Games)
	require.NoError(t, RestoreGame(doomed.ID), "restoring a game that is not deleted changes nothing")
	require.Error(t, RestoreGame("nope"))
	require.Error(t, DeleteGame("nope"))

	// Deleted for good with its cloud saves: they go too.
	require.NoError(t, DeleteGame(doomed.ID))
	require.NoError(t, DeleteGameForever(doomed.ID, true))
	assert.NoDirExists(t, doomedCloud)
	assert.Empty(t, decode[TrashView](t)(Trash()).Games, "gone for good, not in the trash")
	assert.NotContains(t, names(), "Doomed")

	// Deleted for good without them: they stay, as a leftover that can be
	// deleted, or brought back as a game.
	keeper, keeperCloud := withSaves("Keeper", "keeper")
	require.NoError(t, DeleteGameForever(keeper.ID, false), "a live game can be deleted for good")
	assert.DirExists(t, keeperCloud)
	left := decode[[]services.LeftoverView](t)(Leftovers())
	require.Len(t, left, 1)
	assert.Equal(t, "Keeper", left[0].GameName)
	assert.Equal(t, keeper.Emulators[0].Key, left[0].Key)
	assert.Positive(t, left[0].Files)
	require.NoError(t, DeleteLeftover(left[0].Key))
	assert.NoDirExists(t, keeperCloud)
	assert.Empty(t, decode[[]services.LeftoverView](t)(Leftovers()))

	again, againCloud := withSaves("Again", "again")
	require.NoError(t, DeleteGameForever(again.ID, false))
	left = decode[[]services.LeftoverView](t)(Leftovers())
	require.Len(t, left, 1)
	restored, err := RestoreLeftover(left[0].Key)
	require.NoError(t, err)
	assert.Contains(t, names(), "Again", "the leftover is a game again")
	assert.NotEmpty(t, restored)
	assert.DirExists(t, againCloud)
	assert.Empty(t, decode[[]services.LeftoverView](t)(Leftovers()))
	_, err = RestoreLeftover("nope")
	require.Error(t, err)
	require.Error(t, DeleteLeftover("../outside"), "only folders of games")

	// An emulator: to the trash and back, then for good.
	temp := decode[services.EmulatorView](t)(SaveEmulator(mustJSON(t, services.EmulatorSetup{Name: "Temp Emu", Exe: "dev.temp"})))
	require.NoError(t, DeleteEmulator(temp.ID))
	tr = decode[TrashView](t)(Trash())
	require.Len(t, tr.Emulators, 1)
	assert.Equal(t, "Temp Emu", tr.Emulators[0].Name)
	assert.Empty(t, tr.Games)
	require.NoError(t, RestoreEmulator(temp.ID))
	assert.Empty(t, decode[TrashView](t)(Trash()).Emulators)
	require.NoError(t, DeleteEmulator(temp.ID))
	require.NoError(t, DeleteEmulatorForever(temp.ID, false))
	assert.Empty(t, decode[TrashView](t)(Trash()).Emulators, "gone for good")
	require.Error(t, RestoreEmulator(temp.ID))
	require.Error(t, DeleteEmulator("nope"))

	// SteamGridDB artwork: the wrappers refuse before any request is made.
	require.Error(t, TestSteamGridKey("  "), "an empty key")
	_, err = SearchGridGames("zelda")
	require.ErrorContains(t, err, "API key", "no key in the settings")
	_, err = GridArt("cover", 12)
	require.ErrorContains(t, err, "API key")
	_, err = GridArt("wide", 12)
	require.ErrorContains(t, err, "unknown artwork kind", "the app picks covers and backgrounds")
	_, err = GridArt("cover", 0)
	require.Error(t, err, "a SteamGridDB game must be chosen first")
	_, err = SetCoverFromURL(g.ID, "https://example.com/cover.png")
	require.ErrorContains(t, err, "not a SteamGridDB image URL", "only SteamGridDB's hosts are downloaded")
	_, err = SetBackgroundFromURL(g.ID, "http://cdn2.steamgriddb.com/hero.png")
	require.ErrorContains(t, err, "not a SteamGridDB image URL", "https only")
	_, err = SetCoverFromURL("nope", "https://cdn2.steamgriddb.com/cover.png")
	require.Error(t, err, "unknown game")
	cleared := decode[services.GameView](t)(ClearBackground(g.ID))
	assert.Empty(t, cleared.Art["hero"].Path, "a game with no background keeps none")
	_, err = ClearBackground("nope")
	require.Error(t, err)

	// Save suggestions from an emulator's manifest, under a data folder the user granted
	// (the in-memory FS stands in for the Storage Access Framework).
	mem := localfs.NewMem()
	localfs.Register(localfs.SAF, mem)
	t.Cleanup(func() { localfs.Register(localfs.SAF, nil) })
	dataDir := localfs.SAF + "test.docs/primary%3ADolphin"
	require.NoError(t, localfs.MkdirAll(dataDir))
	w, err := localfs.Create(localfs.Join(dataDir, "GC/USA/Card A/luigi.gci"))
	require.NoError(t, err)
	_, _ = w.Write([]byte("save"))
	require.NoError(t, w.Close())
	require.NoError(t, os.MkdirAll(s.core.Dirs.ManifestsDir(), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(s.core.Dirs.ManifestsDir(), "dolphin.yaml"), []byte(`---
"Luigi's Mansion (GameCube)":
  files:
    "<root>/GC/USA/Card A/*.gci":
      tags:
        - save
    "<root>/StateSaves/GLME01*.sav":
      tags:
        - save
`), 0o644))
	dolphin, err := s.emus.Save(services.EmulatorSetup{Name: "Dolphin", Exe: "org.dolphinemu.dolphinemu", Preset: "dolphin", DataDir: dataDir})
	require.NoError(t, err)
	luigi := decode[services.GameView](t)(AddEmulatorGame("Luigi's Mansion", dolphin.ID, "", ""))
	sugKey := luigi.Emulators[0].Key

	found := decode[services.EmuSuggestResult](t)(SuggestSaves(sugKey, ""))
	require.Len(t, found.Suggestions, 2)
	assert.True(t, found.PerGame)
	assert.Equal(t, dataDir+"/GC/USA/Card A", found.Suggestions[0].Path)
	assert.Equal(t, []string{"*.gci", "*.gci/**"}, found.Suggestions[0].Include, "files, and folders by that name")
	assert.True(t, found.Suggestions[0].Exists, "the card is there, in the granted folder")
	assert.False(t, found.Suggestions[1].Exists, "no save states yet")

	// Autofill takes the one that is there, with its filter; a second run changes nothing.
	filled := decode[AutofillSaveResult](t)(AutofillSaves(sugKey, ""))
	assert.Equal(t, 1, filled.Applied)
	luigi = decode[services.GameView](t)(Game(luigi.ID))
	assert.Equal(t, dataDir+"/GC/USA/Card A", luigi.Emulators[0].Paths["saves"])
	assert.Equal(t, []string{"*.gci", "*.gci/**"}, luigi.Emulators[0].Slots[0].Include)
	assert.Zero(t, decode[AutofillSaveResult](t)(AutofillSaves(sugKey, "")).Applied, "the profile has a folder now")

	// Taking another suggestion adds a location.
	extra, err := json.Marshal(found.Suggestions[1])
	require.NoError(t, err)
	luigi = decode[services.GameView](t)(UseSaveSuggestion(sugKey, string(extra)))
	require.Len(t, luigi.Emulators[0].Slots, 2)
	assert.Equal(t, dataDir+"/StateSaves", luigi.Emulators[0].Paths["saves-2"])
	assert.Equal(t, []string{"GLME01*.sav", "GLME01*.sav/**"}, luigi.Emulators[0].Slots[1].Include)
	_, err = UseSaveSuggestion(sugKey, "{")
	require.Error(t, err, "not a suggestion")
	_, err = SuggestSaves(g.ID, "")
	require.ErrorContains(t, err, "emulator profile", "the game's own saves have no emulator")
	_, err = SuggestSaves(key, "")
	require.ErrorContains(t, err, "emulator type", "an emulator of no known type")

	// An rclone.conf the user picked: the supported accounts are copied, a same-named one is replaced.
	imp := decode[cloud.ImportResult](t)(ImportRcloneConfig("[nas]\ntype = smb\nhost = nas.local\n\n[bucket]\ntype = s3\n"))
	assert.Equal(t, []string{"nas"}, imp.Imported)
	assert.Equal(t, []string{"bucket (s3)"}, imp.Skipped)
	assert.Empty(t, imp.Replaced)
	imp = decode[cloud.ImportResult](t)(ImportRcloneConfig("[nas]\ntype = smb\nhost = other.local\n"))
	assert.Equal(t, []string{"nas"}, imp.Replaced)
	remotes := decode[[]cloud.Remote](t)(Remotes())
	assert.True(t, slices.ContainsFunc(remotes, func(r cloud.Remote) bool { return r.Name == "nas" }), "it is an account now")
	_, err = ImportRcloneConfig("  \n")
	require.Error(t, err, "an empty file")
	_, err = ImportRcloneConfig("RCLONE_ENCRYPT_V0:\nabc")
	require.ErrorContains(t, err, "encrypted")

	WaitBackground()
	assert.FileExists(t, filepath.Join(root, "cloud", "library.json"), "catalog uploaded")
	assert.False(t, Cancel(g.ID), "nothing running")
	log, err := LogFile()
	require.NoError(t, err)
	assert.FileExists(t, log)
}

func must(s string, err error) string {
	if err != nil {
		panic(err)
	}
	return s
}

func mustJSON(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	require.NoError(t, err)
	return string(b)
}
