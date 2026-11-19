package game_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/akfaiz/carryover/services"
	"github.com/akfaiz/carryover/ui/env"
	"github.com/akfaiz/carryover/ui/internal/uitest"
	"github.com/akfaiz/carryover/ui/pages/game"
	"github.com/akfaiz/carryover/ui/shell"
)

// elsewhere stands in for every page but the game's: tests only need to see
// where the game page navigated to.
type elsewhere struct{}

func (elsewhere) View() fyne.CanvasObject { return widget.NewLabel("elsewhere") }

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

// askDelete opens the game page's delete dialog (from its "⋯" menu).
func askDelete(t *testing.T, h *uitest.Harness) {
	t.Helper()
	gp, ok := h.Shell.Page().(*game.Page)
	require.True(t, ok, "on a game page")
	g, _ := h.Shell.Store().Game(gp.GameID())
	var item *fyne.MenuItem
	for _, it := range gp.MoreMenu(g, false).Items {
		if it.Label == "Delete from library…" {
			item = it
		}
	}
	require.NotNil(t, item, "the menu offers to delete")
	item.Action()
	h.Settle()
}

func TestGamePageDeleteAsksFirst(t *testing.T) {
	h := newHarness(t)
	g := h.AddGame("Game A", t.TempDir())
	h.Open("/")
	h.Open("/game/" + g.ID)

	askDelete(t, h)
	uitest.TapOverlayButton(t, h, "Cancel") // the confirmation can be backed out of
	h.Settle()
	require.Len(t, h.Core.Library.List(), 1)
	assert.Equal(t, "/game/"+g.ID, h.Shell.Route())

	askDelete(t, h)
	uitest.TapOverlayButton(t, h, "Delete")
	h.Settle()
	assert.Empty(t, h.Core.Library.List())
	assert.Equal(t, "/", h.Shell.Route(), "back to the library")
}

// With a cloud remote, the game page syncs on demand and takes manual backups.
func TestGamePageSyncAndBackup(t *testing.T) {
	h := newHarness(t)
	cloudDir := h.UseLocalCloud()
	saves := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(saves, "a.sav"), []byte("save"), 0o644))
	g := h.AddGame("Game A", saves)
	h.Open("/game/" + g.ID)

	sync := uitest.ButtonWith(t, uitest.PageWidgets[*widget.Button](h), "Sync now")
	require.False(t, sync.Disabled(), "a cloud remote is configured")
	test.Tap(sync)
	h.Settle()
	assert.FileExists(t, filepath.Join(cloudDir, "games", g.ID, "current", "saves", "a.sav"), "the saves were uploaded")

	// Back up now stores a pinned backup.
	h.Open("/game/" + g.ID)
	test.Tap(uitest.ButtonWith(t, uitest.PageWidgets[*widget.Button](h), "Back up now"))
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

	buttons := map[string]int{}
	for _, b := range uitest.PageWidgets[*widget.Button](h) {
		buttons[b.Text]++
	}
	for _, name := range []string{"Without an emulator", "Eden", "Ryujinx"} {
		assert.Equal(t, 1, buttons[name], "a switcher button for %s", name)
	}
	assert.Equal(t, 1, buttons["Compare with cloud"], "one profile's cards at a time")
	assert.Equal(t, 1, buttons["Back up now"])

	gp := h.Shell.Page().(*game.Page)
	assert.Empty(t, gp.SelectedProfile(uitest.MustGame(t, h, g.ID)), "starts on the first")
	uitest.TapButton(t, h, "Eden")
	h.Settle()
	assert.Equal(t, eden.ID, gp.SelectedProfile(uitest.MustGame(t, h, g.ID)))
	assert.Equal(t, widget.HighImportance, uitest.ButtonWith(t, uitest.PageWidgets[*widget.Button](h), "Eden").Importance, "the selected one stands out")
}

// A plain game (no emulators) has no switcher.
func TestGamePageWithoutEmulatorsHasNoTags(t *testing.T) {
	h := newHarness(t)
	g := h.AddGame("Game A", t.TempDir())
	h.Open("/game/" + g.ID)
	texts := strings.Join(uitest.LabelTexts(h), "\n")
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
	uitest.TapButton(t, h, "Sync now")
	h.Settle()
	texts := uitest.ButtonTextsOnPage(h)
	assert.Contains(t, texts, "Keep this device's saves")
	assert.Contains(t, texts, "Keep cloud saves")
}

// The delete confirmation says where kept cloud saves can be found again, and
// drops the line once they are being deleted too.
func TestDeleteConfirmationExplainsKeptSaves(t *testing.T) {
	h := newHarness(t)
	g := h.AddGame("Game A", t.TempDir())
	h.Open("/game/" + g.ID)
	askDelete(t, h)
	assert.Contains(t, uitest.OverlayText(h), "Leftover cloud saves")
	test.Tap(uitest.OverlayWidgets[*widget.Check](h)[0])
	h.Settle()
	assert.NotContains(t, uitest.OverlayText(h), "Leftover cloud saves")
}

// The Steam shortcut of a game with emulators can be for the whole game or for
// any one of its emulators, not only the one shown on the page.
func TestSteamShortcutTargetsListEveryEmulator(t *testing.T) {
	t.Setenv("CARRYOVER_STEAM_ROOT", t.TempDir())
	h := newHarness(t)
	g := h.AddGame("Game A", t.TempDir())
	var ids []string
	for _, name := range []string{"Eden", "Ryujinx"} {
		ev, err := h.Shell.Emus().Save(services.EmulatorSetup{Name: name, Exe: name})
		require.NoError(t, err)
		ids = append(ids, ev.ID)
	}
	var in []services.GameEmulatorInput
	for _, id := range ids {
		in = append(in, services.GameEmulatorInput{EmulatorID: id, Slots: g.Slots, Rom: "g.nsp", Paths: map[string]string{"saves": filepath.Join(t.TempDir(), id)}})
	}
	_, err := h.Shell.Lib().Save(services.GameInput{
		ID: g.ID, Name: g.Name, Slots: g.Slots, Device: services.DeviceInput{Paths: g.Device.Paths, Exe: g.Device.Exe}, Emulators: in,
	})
	require.NoError(t, err)
	h.Shell.Store().Refresh()
	h.Settle()

	h.Open("/game/" + g.ID)
	var shortcut *widget.Select
	for _, s := range uitest.WidgetsOf[*widget.Select](h.Shell.Page().View()) {
		for _, o := range s.Options {
			if o == "Only Eden" {
				shortcut = s
			}
		}
	}
	require.NotNil(t, shortcut, "no shortcut target selector")
	assert.Equal(t, []string{
		"The whole game: asks which emulator to use when it starts", "Only Eden", "Only Ryujinx",
	}, shortcut.Options)
	assert.Equal(t, shortcut.Options[0], shortcut.Selected, "one shortcut for the whole game is the default")
}

// A backup can be unpinned, restored and deleted from the game page.
func TestGamePageBackupActions(t *testing.T) {
	h := newHarness(t)
	h.UseLocalCloud()
	saves := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(saves, "a.sav"), []byte("save"), 0o644))
	g := h.AddGame("Game A", saves)
	h.Open("/game/" + g.ID)
	test.Tap(uitest.ButtonWith(t, uitest.PageWidgets[*widget.Button](h), "Sync now"))
	h.Settle()
	h.Open("/game/" + g.ID)
	test.Tap(uitest.ButtonWith(t, uitest.PageWidgets[*widget.Button](h), "Back up now"))
	h.Settle()
	buttons := func() []*widget.Button { return uitest.PageWidgets[*widget.Button](h) }

	test.Tap(uitest.ButtonWith(t, buttons(), "Unpin"))
	h.Settle()
	backups, err := h.Shell.Sync().Backups(g.ID)
	require.NoError(t, err)
	require.Len(t, backups, 1)
	assert.False(t, backups[0].Pinned)

	test.Tap(uitest.ButtonWith(t, buttons(), "Restore"))
	h.Settle()
	uitest.TapOverlayButton(t, h, "Restore")
	h.Settle()
	assert.Contains(t, h.Toasts(), "Backup restored and uploaded")

	// delete (the icon-only button): the one with no text
	var del *widget.Button
	for _, b := range buttons() {
		if b.Text == "" && b.Icon != nil {
			del = b
		}
	}
	if del != nil {
		test.Tap(del)
		h.Settle()
		uitest.TapOverlayButton(t, h, "Delete")
		h.Settle()
	}
}

func TestGamePageSyncNowAndRedraw(t *testing.T) {
	h := newHarness(t)
	h.UseLocalCloud()
	saves := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(saves, "a.sav"), []byte("save"), 0o644))
	g := h.AddGame("Game A", saves)
	h.Open("/game/" + g.ID)
	gp, ok := h.Shell.Page().(*game.Page)
	require.True(t, ok)
	assert.NotNil(t, gp.DefaultFocus())
	gp.Redraw()
	gp.SyncNow()
	h.Settle()
	gv := uitest.MustGame(t, h, g.ID)
	assert.NotEmpty(t, gv.Status.LastSyncAt)
}
