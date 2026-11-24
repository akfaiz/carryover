package game_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/akfaiz/carryover/internal/library"
	"github.com/akfaiz/carryover/services"
	"github.com/akfaiz/carryover/ui/env"
	"github.com/akfaiz/carryover/ui/internal/uitest"
	"github.com/akfaiz/carryover/ui/kit"
	"github.com/akfaiz/carryover/ui/pages/game"
	"github.com/akfaiz/carryover/ui/shell"
)

// elsewhere stands in for every page but the game's: tests only need to see
// where the game page navigated to.
type elsewhere struct{}

func (elsewhere) View(c *ui.Context) { ui.Text(c, "elsewhere") }

// newHarness shows the game page in a shell, in an isolated data dir. Any other
// route shows a stand-in page.
func newHarness(t *testing.T) *uitest.Harness {
	t.Helper()
	return uitest.New(t, shell.Config{
		Page: func(e env.Env, route string) env.Page {
			if id, ok := strings.CutPrefix(route, "/game/"); ok {
				return game.New(e, id)
			}
			return elsewhere{}
		},
		AddGame:      func(env.Env) {},
		SyncProblems: func(env.Env, int, []services.SyncResult) {},
	})
}

func askDelete(t *testing.T, h *uitest.Harness) {
	t.Helper()
	h.Click("More")
	h.Click("Delete from library…")
}

func TestGamePageDeleteAsksFirst(t *testing.T) {
	h := newHarness(t)
	g := h.AddGame("Game A", t.TempDir())
	h.Open("/")
	h.Open("/game/" + g.ID)

	askDelete(t, h)
	h.Click("Cancel") // the confirmation can be backed out of
	require.Len(t, h.Core.Library.List(), 1)
	assert.Equal(t, "/game/"+g.ID, h.Shell.Route())

	askDelete(t, h)
	h.Click("Delete")
	h.Settle()
	assert.Empty(t, h.Core.Library.List())
	assert.Equal(t, "/", h.Shell.Route(), "the game's page is gone")
}

func TestGamePageSyncAndBackup(t *testing.T) {
	h := newHarness(t)
	cloudDir := h.UseLocalCloud()
	saves := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(saves, "a.sav"), []byte("save"), 0o644))
	g := h.AddGame("Game A", saves)
	h.Open("/game/" + g.ID)

	h.Click("Sync now")
	h.Settle()
	assert.FileExists(t, filepath.Join(cloudDir, "games", g.ID, "current", "saves", "a.sav"), "the saves were uploaded")

	// Back up now stores a pinned backup.
	h.Open("/game/" + g.ID)
	h.Click("Back up now")
	h.Settle()
	backups, err := h.Shell.Sync().Backups(g.ID)
	require.NoError(t, err)
	require.Len(t, backups, 1)
	assert.Equal(t, "manual", backups[0].Kind)
	assert.True(t, backups[0].Pinned)
}

// The game page has a switcher with one button per way to play the game; the
// saves and backups of the selected one are shown.
func TestGamePageSwitchesBetweenEmulators(t *testing.T) {
	h := newHarness(t)
	g, eden, _ := h.AddEmulatorGame()
	h.Open("/game/" + g.ID)

	for _, name := range []string{"Without an emulator", "Eden", "Ryujinx"} {
		assert.Contains(t, h.Texts(), name, "a switcher button for %s", name)
	}
	gp := h.Shell.Page().(*game.Page)
	assert.Empty(t, gp.SelectedProfile(uitest.MustGame(t, h, g.ID)), "starts on the first")
	h.Click("Eden")
	assert.Equal(t, eden.ID, gp.SelectedProfile(uitest.MustGame(t, h, g.ID)))
}

// A plain game (no emulators) has no switcher.
func TestGamePageWithoutEmulatorsHasNoTags(t *testing.T) {
	h := newHarness(t)
	g := h.AddGame("Game A", t.TempDir())
	h.Open("/game/" + g.ID)
	texts := strings.Join(h.Texts(), "\n")
	assert.NotContains(t, texts, "Without an emulator")
	assert.NotContains(t, texts, "Saves of")
	assert.Contains(t, texts, "Saves")
}

// A conflict found by "Sync now" is compared with the cloud at once, so the
// buttons to keep either side are right there.
func TestGamePageConflictShowsTheChoice(t *testing.T) {
	h := newHarness(t)
	h.UseLocalCloud()
	saves := t.TempDir()
	file := filepath.Join(saves, "a.sav")
	require.NoError(t, os.WriteFile(file, []byte("one"), 0o644))
	g := h.AddGame("Game A", saves)
	_, err := h.Shell.Sync().Sync(g.ID, "")
	require.NoError(t, err)

	// Another device uploads a change, and this one changes too.
	st, err := h.Core.States.Load(g.ID)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(file, []byte("two-local"), 0o644))
	_, err = h.Shell.Sync().Sync(g.ID, "local") // the cloud now has "two-local"
	require.NoError(t, err)
	require.NoError(t, h.Core.States.Save(g.ID, st)) // forget that sync: both sides look changed
	require.NoError(t, os.WriteFile(file, []byte("three"), 0o644))

	h.Open("/game/" + g.ID)
	h.Click("Sync now")
	h.Settle()
	assert.Contains(t, h.Texts(), "Keep this device's saves")
	assert.Contains(t, h.Texts(), "Keep cloud saves")
}

// The delete confirmation says where kept cloud saves can be found again, and
// drops the line once they are being deleted too.
func TestDeleteConfirmationExplainsKeptSaves(t *testing.T) {
	h := newHarness(t)
	g := h.AddGame("Game A", t.TempDir())
	h.Open("/game/" + g.ID)
	askDelete(t, h)
	kept := "The cloud saves stay until you delete them. Once it is gone for good, they are listed under Trash → Leftover cloud saves."
	assert.Contains(t, h.Texts(), kept)
	h.Click("Also delete its cloud saves and backups")
	assert.NotContains(t, h.Texts(), kept)
}

// The pad starts on Back, which only goes back, never on Play.
func TestGamePagePadStartsOnBack(t *testing.T) {
	h := newHarness(t)
	g := h.AddGame("Game A", t.TempDir())
	h.Open("/")
	h.Open("/game/" + g.ID)
	h.Shell.Navigator().Press(kit.PadDown)
	h.Settle()
	assert.False(t, h.Tester.Focused("Play"), "a stray A can't start the game")
	h.Shell.Navigator().Press(kit.PadAccept)
	h.Settle()
	assert.Equal(t, "/", h.Shell.Route(), "A on Back goes back")
}

// A game not set up on this device offers the setup where the saves would be,
// and no actions that can't run yet; its setup hint shows once.
func TestGamePageNotSetUp(t *testing.T) {
	h := newHarness(t)
	h.UseLocalCloud()
	g, err := h.Shell.Lib().Save(services.GameInput{
		Name:   "Game B",
		Slots:  []services.SaveSlot{{Key: "saves", Label: "Saves"}},
		Device: services.DeviceInput{Paths: map[string]string{}, LaunchMode: services.LaunchDirect},
	})
	require.NoError(t, err)
	h.Shell.Store().Refresh()
	h.Settle()
	h.Open("/game/" + g.ID)

	view := uitest.MustGame(t, h, g.ID)
	require.False(t, view.Ready)
	assert.Contains(t, h.Texts(), "Set up saves for this device")
	assert.NotContains(t, h.Texts(), "Compare with cloud")
	assert.NotContains(t, h.Texts(), "Back up now")
	n := 0
	for _, s := range h.Texts() {
		if s == view.SetupHint {
			n++
		}
	}
	assert.Equal(t, 1, n, "the setup hint shows once")

	h.Click("Set up saves for this device")
	assert.Equal(t, "/edit/"+g.ID, h.Shell.Route())
}

// With emulators, the Saves card doesn't repeat the header's setup hint.
func TestGamePageSetupHintNotRepeated(t *testing.T) {
	h := newHarness(t)
	eden, err := h.Shell.Emus().Save(services.EmulatorSetup{Name: "Eden", Exe: "eden"})
	require.NoError(t, err)
	slots := []services.SaveSlot{{Key: "saves", Label: "Saves"}}
	g, err := h.Shell.Lib().Save(services.GameInput{
		Name: "Game C", Slots: slots,
		Device:    services.DeviceInput{Paths: map[string]string{}, LaunchMode: services.LaunchDirect},
		Emulators: []services.GameEmulatorInput{{EmulatorID: eden.ID, Slots: slots}},
	})
	require.NoError(t, err)
	h.Shell.Store().Refresh()
	h.Settle()
	h.Open("/game/" + g.ID)

	view := uitest.MustGame(t, h, g.ID)
	require.False(t, view.Ready)
	require.NotEmpty(t, view.SetupHint)
	n := 0
	for _, s := range h.Texts() {
		if s == view.SetupHint {
			n++
		}
	}
	assert.Equal(t, 1, n, "the hint shows in the header only")
}

// A save location's filters show under its folder, the first few of each; a
// location without filters shows none.
func TestGamePageShowsSlotFilters(t *testing.T) {
	h := newHarness(t)
	g, err := h.Shell.Lib().Save(services.GameInput{
		Name: "Game F",
		Slots: []services.SaveSlot{
			{Key: "saves", Label: "Saves", Include: []string{"*.sav"}, Exclude: []string{"*.log", "*.tmp", "cache/**", "shaders/**"}},
			{Key: "config", Label: "Config"},
		},
		Device: services.DeviceInput{
			Paths: map[string]string{"saves": t.TempDir(), "config": t.TempDir()}, Exe: "/x/game.exe", LaunchMode: services.LaunchDirect,
		},
	})
	require.NoError(t, err)
	h.Shell.Store().Refresh()
	h.Settle()
	h.Open("/game/" + g.ID)

	assert.Contains(t, h.Texts(), "Only *.sav · Skips *.log, *.tmp, cache/** +1 more")
	n := 0
	for _, s := range h.Texts() {
		if strings.HasPrefix(s, "Only ") || strings.HasPrefix(s, "Skips ") {
			n++
		}
	}
	assert.Equal(t, 1, n, "only the location with filters says which files sync")
}

// An emulator game lists other devices as a PC game does: in the Other devices
// card, with the shown emulator's save paths there.
func TestGamePageOtherDevicesOfAnEmulator(t *testing.T) {
	h := newHarness(t)
	me := h.Core.DeviceID()
	eden, err := h.Shell.Emus().Save(services.EmulatorSetup{Name: "Eden", Exe: "eden"})
	require.NoError(t, err)
	g, err := h.Core.Library.Save(library.Game{
		Name:      "Game E",
		Emulators: []library.GameEmulator{{EmulatorID: eden.ID, Slots: []library.SaveSlot{{Key: "account", Label: "Account save"}}}},
		Devices: map[string]library.DeviceConfig{
			me:      {Emulators: map[string]library.EmulatorConfig{eden.ID: {Rom: "g.nsp", Paths: map[string]string{"account": t.TempDir()}}}},
			"deck1": {DeviceName: "Deck", Emulators: map[string]library.EmulatorConfig{eden.ID: {Paths: map[string]string{"account": "/deck/eden"}}}},
			"pc2":   {DeviceName: "Laptop"},
		},
	}, me)
	require.NoError(t, err)
	h.Shell.Store().Refresh()
	h.Settle()
	h.Open("/game/" + g.ID)

	assert.Contains(t, h.Texts(), "Other devices")
	var shown []string // without the invisible break points of a path
	for _, s := range h.Texts() {
		shown = append(shown, strings.ReplaceAll(s, "\u200b", ""))
	}
	assert.Contains(t, shown, "Account save: /deck/eden")
	assert.Contains(t, h.Texts(), "No save location set there", "Laptop doesn't play it with Eden")
	for _, s := range h.Texts() {
		assert.False(t, strings.HasPrefix(s, "Also on"), "no second way of saying it in the header")
	}
}
