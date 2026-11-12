package services

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/akfaiz/carryover/internal/saves/emumanifest"
	"github.com/akfaiz/carryover/internal/saves/ryujinx"
	"github.com/akfaiz/carryover/internal/saves/yuzu"
)

func TestCleanRomName(t *testing.T) {
	for in, want := range map[string]string{
		"Super Mario Odyssey [0100000000010000][v0].nsp":                 "Super Mario Odyssey",
		`C:\Roms\Switch\Metroid Prime (USA) (Rev 1).iso`:                 "Metroid Prime",
		"/roms/Final_Fantasy_VII_v1.0.2.xci":                             "Final Fantasy VII",
		"The Legend of Zelda - Tears of the Kingdom [0100] [v1.2.0].nsp": "The Legend of Zelda - Tears of the Kingdom",
		"Pokemon Y (Europe) (En,Fr,De,Es,It).3ds":                        "Pokemon Y",
		"Game.name.with.dots.nsp":                                        "Game.name.with.dots",
		"":                                                               "",
	} {
		assert.Equal(t, want, cleanRomName(in), in)
	}
}

func TestRomIDs(t *testing.T) {
	assert.Equal(t, []string{"0100000000010000"}, romIDs("Mario [0100000000010000][v0].nsp"))
	assert.Equal(t, []string{"SLUS-12345"}, romIDs("Game [SLUS-12345].iso"))
	assert.Equal(t, []string{"01007EF00011E000"}, romIDs("01007EF00011E000.nsp"))
	assert.Empty(t, romIDs("Plain Game (USA).iso"))
	assert.Empty(t, romIDs("Game [v1.0.2].nsp"), "a version is not an ID")
}

// quiet stops the background library upload a save triggers, which would
// otherwise race with the test's temp dir being deleted.
func quiet(t *testing.T, e syncEnv) {
	t.Helper()
	st := e.c.Settings.Get()
	st.SyncLibrary = false
	require.NoError(t, e.c.Settings.Update(st))
}

const switchSample = `---
"Super Mario Odyssey (Switch)":
  files:
    "<root>/nand/user/save/0000000000000000/*/0100000000010000":
      tags:
        - save
    "<root>/nand/user/save/cache/0100000000010000":
      tags:
        - save
"Super Mario Odyssey Demo (Switch)":
  files:
    "<root>/nand/user/save/0000000000000000/*/0100000000010001":
      tags:
        - save
`

func TestSuggestSavesForSwitchEmulator(t *testing.T) {
	e := newSyncEnv(t)
	quiet(t, e)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(switchSample)) }))
	defer srv.Close()
	svc := NewEmulatorService(e.c, NewBus())
	svc.store = &emumanifest.Store{Dir: t.TempDir(), URL: srv.URL + "/"}

	root := t.TempDir()
	const pcProfile = "AD2ECA295FA7CD7B990C2688B94AFD57"
	save := filepath.Join(root, "nand", "user", "save", "0000000000000000", pcProfile, "0100000000010000")
	require.NoError(t, os.MkdirAll(save, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(save, "progress.bin"), []byte("x"), 0o644))
	writeProfiles(t, root, pcProfile)

	// Without a type nothing can be looked up.
	em, err := svc.Save(EmulatorSetup{Name: "Eden", Exe: "eden", DataDir: root})
	require.NoError(t, err)
	_, err = svc.SuggestSaves(em.ID, EmuSuggestInput{GameName: "Super Mario Odyssey"})
	require.ErrorContains(t, err, "emulator type")

	_, err = svc.Save(EmulatorSetup{ID: em.ID, Name: "Eden", Exe: "eden", DataDir: root, Preset: "no-such-type"})
	require.Error(t, err)
	em, err = svc.Save(EmulatorSetup{ID: em.ID, Name: "Eden", Exe: "eden", DataDir: root, Preset: "switch"})
	require.NoError(t, err)
	assert.Equal(t, "switch", em.Preset)
	assert.Equal(t, root, em.Device.DataDir)

	// The title ID in the ROM name finds exactly the game, not the demo.
	res, err := svc.SuggestSaves(em.ID, EmuSuggestInput{Rom: "Odyssey [0100000000010000][v0].nsp"})
	require.NoError(t, err)
	require.Len(t, res.Suggestions, 2, "the account save and the device save")
	rootSlash := strings.ReplaceAll(root, `\`, "/")
	acct, dev := res.Suggestions[0], res.Suggestions[1]
	assert.Equal(t, "Super Mario Odyssey (account save)", acct.Game)
	assert.Equal(t, kindAccount, acct.Kind)
	assert.Equal(t, rootSlash+"/nand/user/save/0000000000000000/"+pcProfile+"/0100000000010000", acct.Path,
		"the exact folder of this device's profile, so the profile ID stays out of the cloud layout")
	assert.Empty(t, acct.Include, "no filters: the title folder is stored flat")
	assert.True(t, acct.Exists)
	assert.Equal(t, kindDevice, dev.Kind)
	assert.Equal(t, rootSlash+"/nand/user/save/0000000000000000/00000000000000000000000000000000/0100000000010000", dev.Path)
	assert.False(t, dev.Exists, "this game has no device save here")
	assert.Empty(t, res.Note)

	// By name only: both the game and its demo match.
	res, err = svc.SuggestSaves(em.ID, EmuSuggestInput{GameName: "super mario odyssey"})
	require.NoError(t, err)
	assert.Len(t, res.Suggestions, 4)
	assert.Equal(t, "Super Mario Odyssey (account save)", res.Suggestions[0].Game, "the exact name comes first")

	res, err = svc.SuggestSaves(em.ID, EmuSuggestInput{GameName: "unknown game"})
	require.NoError(t, err)
	assert.Empty(t, res.Suggestions)

	// The manifest is cached now, so a ROM's title ID gives the game's real name offline.
	assert.Equal(t, "Super Mario Odyssey", svc.GuessRomName(em.ID, "x [0100000000010000].nsp"))
	assert.Equal(t, "Some Game", svc.GuessRomName(em.ID, "Some_Game (USA).nsp"))
	assert.Equal(t, "Some Game", svc.GuessRomName("no-such-emulator", "Some_Game (USA).nsp"))
}

func TestDefaultDataDirAndPresets(t *testing.T) {
	e := newSyncEnv(t)
	svc := NewEmulatorService(e.c, nil)
	assert.NotEmpty(t, svc.Presets())
	assert.Empty(t, svc.DefaultDataDir("nope", "x"))
	assert.NotEmpty(t, svc.DefaultDataDir("ryujinx", "Ryujinx"))
}

// An emulator without per-game saves (Ryujinx) offers its shared locations
// whatever the game is called, and says they hold every game's saves.
func TestSuggestSavesForEmulatorWithoutPerGameSaves(t *testing.T) {
	e := newSyncEnv(t)
	quiet(t, e)
	const ryujinx = `---
"Ryujinx (all saves)":
  files:
    "<root>/bis/user/save":
      tags:
        - save
    "<root>/bis/user/saveMeta":
      tags:
        - save
    "<root>/system/Profiles.json":
      tags:
        - save
`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(ryujinx)) }))
	defer srv.Close()
	svc := NewEmulatorService(e.c, NewBus())
	svc.store = &emumanifest.Store{Dir: t.TempDir(), URL: srv.URL + "/"}

	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "bis", "user", "save"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "bis", "user", "save", "x"), []byte("1"), 0o644))
	em, err := svc.Save(EmulatorSetup{Name: "Ryujinx", Exe: "ryujinx", Preset: "ryujinx", DataDir: root})
	require.NoError(t, err)

	res, err := svc.SuggestSaves(em.ID, EmuSuggestInput{GameName: "Mario Kart 8 Deluxe", Rom: "mk8d.nsp"})
	require.NoError(t, err)
	require.Len(t, res.Suggestions, 3, "the game's name doesn't matter")
	assert.False(t, res.PerGame, "so they are not filled in automatically")
	assert.NotEmpty(t, res.Note)
	byPath := map[string]EmuSuggestion{}
	for _, s := range res.Suggestions {
		byPath[strings.TrimPrefix(s.Path, strings.ReplaceAll(root, `\`, "/")+"/")] = s
	}
	assert.True(t, byPath["bis/user/save"].Exists)
	assert.False(t, byPath["bis/user/saveMeta"].Exists)
	assert.Equal(t, []string{"Profiles.json"}, byPath["system"].Include, "a file becomes a filter on its folder")
}

// Ryujinx's numbered save folders are found through its index, so one game's
// saves can be synced instead of all of them.
func TestSuggestRyujinxSavesForOneGame(t *testing.T) {
	e := newSyncEnv(t)
	quiet(t, e)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(switchSample)) }))
	defer srv.Close()
	svc := NewEmulatorService(e.c, NewBus())
	svc.store = &emumanifest.Store{Dir: t.TempDir(), URL: srv.URL + "/"}

	root := t.TempDir()
	index := filepath.Join(root, "bis", "system", "save", "8000000000000000", "0")
	require.NoError(t, os.MkdirAll(index, 0o755))
	const odyssey = 0x0100000000010000
	require.NoError(t, os.WriteFile(filepath.Join(index, "imkvdb.arc"),
		ryujinx.EncodeIndex([]ryujinx.Entry{{ProgramID: odyssey, Type: 1, SaveDataID: 3}}), 0o644))
	save := filepath.Join(root, "bis", "user", "save", "0000000000000003", "0")
	require.NoError(t, os.MkdirAll(save, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(save, "progress.bin"), []byte("x"), 0o644))

	em, err := svc.Save(EmulatorSetup{Name: "Ryujinx", Exe: "ryujinx", Preset: "ryujinx", DataDir: root})
	require.NoError(t, err)

	// The title ID comes from the ROM's name, or, failing that, from the Switch manifest by game name.
	for _, in := range []EmuSuggestInput{
		{Rom: "Odyssey [0100000000010000][v0].nsp"},
		{GameName: "Super Mario Odyssey"},
	} {
		res, err := svc.SuggestSaves(em.ID, in)
		require.NoError(t, err)
		require.Len(t, res.Suggestions, 1, "the game has an account save only: %+v", in)
		sg := res.Suggestions[0]
		assert.Equal(t, "{ryujinx:0100000000010000:account}", sg.Path)
		assert.Equal(t, kindAccount, sg.Kind, "the same kind names as Eden's locations, so they match up when shared")
		assert.Empty(t, sg.Include)
		assert.True(t, sg.Exists)
		assert.True(t, res.PerGame, "so it may be filled in automatically")
		assert.Empty(t, res.Note)
	}

	// The suggested token resolves to that game's folder only, in the emulator's data folder.
	v, err := NewLibraryService(e.c, NewBus(), nil).Get(e.id)
	require.NoError(t, err)
	_, err = NewLibraryService(e.c, NewBus(), nil).Save(GameInput{
		ID: v.ID, Name: v.Name, Slots: v.Slots, Device: DeviceInput{Paths: v.Device.Paths, Exe: v.Device.Exe},
		Emulators: []GameEmulatorInput{{
			EmulatorID: em.ID, Slots: v.Slots, Rom: "x.nsp", Paths: map[string]string{"saves": "{ryujinx:0100000000010000:account}"},
		}},
	})
	require.NoError(t, err)
	g, err := e.c.Library.Get(e.id)
	require.NoError(t, err)
	job, err := e.c.JobFor(g, em.ID)
	require.NoError(t, err)
	assert.Equal(t, save, job.Slots[0].LocalPath)

	// A game Ryujinx hasn't saved yet: both kinds are offered, with a note.
	res, err := svc.SuggestSaves(em.ID, EmuSuggestInput{Rom: "New [0100AAAAAAAAA000].nsp"})
	require.NoError(t, err)
	assert.Len(t, res.Suggestions, 2)
	assert.NotEmpty(t, res.Note)
	for _, s := range res.Suggestions {
		assert.False(t, s.Exists)
		assert.NotEmpty(t, s.Kind)
	}
}

// writeProfiles writes the emulator's profile list (the profiles it has).
func writeProfiles(t *testing.T, dataDir string, ids ...string) {
	t.Helper()
	dir := filepath.Join(dataDir, "nand", "system", "save", "8000000000000010", "su", "avators")
	require.NoError(t, os.MkdirAll(dir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "profiles.dat"), yuzu.EncodeProfiles(ids), 0o644))
}

// A Switch game's account save is found under this device's own profile, never
// under another device's profile (an "orphaned profile" in Eden).
func TestSuggestYuzuProfiles(t *testing.T) {
	const (
		pcProfile   = "AD2ECA295FA7CD7B990C2688B94AFD57"
		deckProfile = "0123456789ABCDEF0123456789ABCDEF"
		title       = "0100000000010000"
	)
	e := newSyncEnv(t)
	quiet(t, e)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(switchSample)) }))
	defer srv.Close()
	svc := NewEmulatorService(e.c, NewBus())
	svc.store = &emumanifest.Store{Dir: t.TempDir(), URL: srv.URL + "/"}
	saveDir := func(root, profile string) {
		require.NoError(t, os.MkdirAll(filepath.Join(root, "nand", "user", "save", "0000000000000000", profile, title), 0o755))
	}
	n := 0
	suggest := func(root string) EmuSuggestResult {
		n++
		em, err := svc.Save(EmulatorSetup{Name: fmt.Sprintf("Eden %d", n), Exe: "eden", Preset: "switch", DataDir: root})
		require.NoError(t, err)
		res, err := svc.SuggestSaves(em.ID, EmuSuggestInput{Rom: "x [" + title + "].nsp"})
		require.NoError(t, err)
		return res
	}
	accounts := func(res EmuSuggestResult) (out []EmuSuggestion) {
		for _, s := range res.Suggestions {
			if s.Kind == kindAccount {
				out = append(out, s)
			}
		}
		return out
	}

	t.Run("a fresh device uses its own profile before the game has a save", func(t *testing.T) {
		root := t.TempDir()
		writeProfiles(t, root, deckProfile)
		a := accounts(suggest(root))
		require.Len(t, a, 1)
		assert.Contains(t, a[0].Path, "/"+deckProfile+"/"+title)
		assert.False(t, a[0].Exists)
	})

	t.Run("a profile from another device is an orphan and is not suggested", func(t *testing.T) {
		root := t.TempDir()
		writeProfiles(t, root, deckProfile)
		saveDir(root, pcProfile) // brought here by an earlier sync
		res := suggest(root)
		a := accounts(res)
		require.Len(t, a, 1)
		assert.Contains(t, a[0].Path, "/"+deckProfile+"/")
		assert.NotContains(t, a[0].Path, pcProfile)
		assert.Contains(t, res.Note, "doesn't have")
		assert.Contains(t, res.Note, pcProfile[:8])
	})

	t.Run("without a profile list the profile folders on disk are used", func(t *testing.T) {
		root := t.TempDir()
		saveDir(root, pcProfile)
		a := accounts(suggest(root))
		require.Len(t, a, 1)
		assert.Contains(t, a[0].Path, "/"+pcProfile+"/")
		assert.True(t, a[0].Exists)
	})

	t.Run("no profile at all: only the device save, with a note", func(t *testing.T) {
		res := suggest(t.TempDir())
		assert.Empty(t, accounts(res))
		require.Len(t, res.Suggestions, 1)
		assert.Equal(t, kindDevice, res.Suggestions[0].Kind)
		assert.Contains(t, res.Note, "no user profile")
	})

	t.Run("several profiles are all offered", func(t *testing.T) {
		root := t.TempDir()
		writeProfiles(t, root, pcProfile, deckProfile)
		a := accounts(suggest(root))
		require.Len(t, a, 2)
		assert.Contains(t, a[0].Game, "profile "+pcProfile[:8])
	})
}
