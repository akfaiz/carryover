package ui

import (
	"image/color"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/akfaiz/carryover/internal/core"
	"github.com/akfaiz/carryover/services"
	"github.com/akfaiz/carryover/ui/internal/uitest"
	"github.com/akfaiz/carryover/ui/kit"
	editorpage "github.com/akfaiz/carryover/ui/pages/editor"
	gamepage "github.com/akfaiz/carryover/ui/pages/game"
	"github.com/akfaiz/carryover/ui/pages/settings"
	"github.com/akfaiz/carryover/ui/shell"
)

// harness is the root package's view of the shared uitest harness: the whole
// application (every page) in a shell, in an isolated data dir.
type harness struct {
	*uitest.Harness
	t *testing.T
	c *core.Core
	u *shell.Shell
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	settings.AutosaveDelay = 20 * time.Millisecond
	uh := uitest.New(t, appConfig())
	return &harness{Harness: uh, t: t, c: uh.Core, u: uh.Shell}
}

// settle runs queued UI work until things go quiet.
func (h *harness) settle() { h.t.Helper(); h.Settle() }

func (h *harness) open(route string) { h.t.Helper(); h.Open(route) }

// widgetsOf returns every widget of type T under root, in tree order.
func widgetsOf[T fyne.CanvasObject](root fyne.CanvasObject) []T { return uitest.WidgetsOf[T](root) }

// pageWidgets returns the widgets of type T in the window.
func pageWidgets[T fyne.CanvasObject](h *harness) []T { return uitest.PageWidgets[T](h.Harness) }

// overlayWidgets returns the widgets of type T in open dialogs and popups.
func overlayWidgets[T fyne.CanvasObject](h *harness) []T { return uitest.OverlayWidgets[T](h.Harness) }

func (h *harness) toasts() []string { return h.Toasts() }

// ---- Settings page ----

// ---- Cloud page ----

// ---- Game editor ----

// ---- Session prompts ----

// ---- Library and game pages ----

// Adding a game asks what kind of game it is, and opens the editor set up for it.
func TestLibraryEmptyStateOffersAddGame(t *testing.T) {
	h := newHarness(t)
	h.open("/")
	tapButton(t, h, "＋ Add your first game")
	tapChoice(t, h, "PC game")
	assert.Equal(t, "/new", h.u.Route())

	h.open("/")
	tapButton(t, h, "＋ Add your first game")
	tapChoice(t, h, "Emulated game")
	require.Equal(t, "/new/emulator", h.u.Route())
	assert.Equal(t, editorpage.KindEmulator, h.u.Page().(*editorpage.Page).Kind(), "the editor starts on the emulator cards")
}

// After saving, Back from the game's page skips the editor.
func TestEditorSaveReplacesItself(t *testing.T) {
	h := newHarness(t)
	g := h.addGame("Game A", t.TempDir())
	h.open("/")
	h.open("/game/" + g.ID)
	tapButton(t, h, "Edit")
	h.settle()
	entryByPlaceholder(t, h, "Game name").SetText("Game B")
	tapButton(t, h, "Save")
	h.settle()
	require.Equal(t, "/game/"+g.ID, h.u.Route())
	tapButton(t, h, "Back")
	h.settle()
	assert.Equal(t, "/", h.u.Route())
}

// ---- Trash ----

// A deleted game waits on the Trash page and Restore brings it back.
func TestTrashPageRestoresAGame(t *testing.T) {
	h := newHarness(t)
	g := h.addGame("Game A", t.TempDir())
	h.open("/game/" + g.ID)
	askDelete(t, h)
	tapOverlayButton(t, h, "Delete")
	h.settle()
	require.Empty(t, h.c.Library.List())

	h.open("/trash")
	tapButton(t, h, "Restore")
	h.settle()
	require.Len(t, h.c.Library.List(), 1, "restored")
	assert.Empty(t, h.c.Library.Trash())
}

// Ticking "also delete the cloud saves" changes the confirmation, and the
// choice decides what is deleted.
func TestDeleteConfirmationCheckbox(t *testing.T) {
	h := newHarness(t)
	cloud := h.useLocalCloud()
	saves := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(saves, "a.sav"), []byte("save"), 0o644))
	g := h.addGame("Game A", saves)
	_, err := h.u.Sync().SyncAll()
	require.NoError(t, err)
	folder := filepath.Join(cloud, "games", g.ID)
	require.DirExists(t, folder)

	// Trash page.
	require.NoError(t, h.u.Lib().Delete(g.ID))
	h.open("/trash")
	tapButton(t, h, "Delete permanently…")
	h.settle()
	checks := overlayWidgets[*widget.Check](h)
	require.Len(t, checks, 1)
	test.Tap(checks[0])
	h.settle()
	assert.True(t, checks[0].Checked)
	assert.Contains(t, overlayText(h), "together with its cloud saves", "the message says what will go")
	tapOverlayButton(t, h, "Delete")
	h.settle()
	assert.NoDirExists(t, folder, "the cloud saves went too")

	// Game page: the same dialog.
	g2 := h.addGame("Game B", saves)
	h.open("/game/" + g2.ID)
	askDelete(t, h)
	checks = overlayWidgets[*widget.Check](h)
	require.Len(t, checks, 1)
	test.Tap(checks[0])
	h.settle()
	assert.Contains(t, overlayText(h), "permanently on all devices")
}

// The sidebar counts what can be restored from the trash. Leftover cloud
// saves are counted on the Trash page itself.
func TestSidebarTrashCount(t *testing.T) {
	h := newHarness(t)
	h.useLocalCloud()
	saves := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(saves, "a.sav"), []byte("save"), 0o644))
	g := h.addGame("Game A", saves)
	_, err := h.u.Sync().SyncAll()
	require.NoError(t, err)
	h.settle()
	assert.Equal(t, "Trash", h.u.NavButtons()["/trash"].Text)

	h.open("/game/" + g.ID)
	askDelete(t, h)
	tapOverlayButton(t, h, "Delete") // to the trash, saves kept
	h.settle()
	h.u.Store().Refresh()
	h.settle()
	assert.Equal(t, "Trash (1)", h.u.NavButtons()["/trash"].Text, "one game in the trash")

	h.open("/trash")
	tapButton(t, h, "Delete permanently…")
	h.settle()
	tapOverlayButton(t, h, "Delete") // gone for good, saves kept
	h.settle()
	assert.Equal(t, "Trash", h.u.NavButtons()["/trash"].Text, "nothing left to restore")
	assert.Contains(t, strings.Join(labelTexts(h), "\n"), "Leftover cloud saves (1)", "its cloud saves are a leftover now")

	tapButton(t, h, "Delete…")
	h.settle()
	tapOverlayButton(t, h, "Delete")
	h.settle()
	assert.NotContains(t, strings.Join(labelTexts(h), "\n"), "Leftover cloud saves (1)")
}

// A toast on screen while the theme changes is repainted with the new palette:
// its text follows the theme, so an old background would make it unreadable.
func TestToastFollowsThemeChange(t *testing.T) {
	h := newHarness(t)
	t.Cleanup(func() { kit.SetPalette(kit.ThemeDark) })
	h.u.ApplyTheme(kit.ThemeDark)

	h.u.Toast("Settings saved", kit.ToastOK)
	require.Len(t, h.u.Toasts().BGs, 1)
	var bg *kit.ToastBG
	for _, b := range h.u.Toasts().BGs {
		bg = b
	}
	dark := bg.Rect.FillColor
	assert.Equal(t, kit.ColBg3, dark)

	h.u.ApplyTheme(kit.ThemeLight)
	assert.NotEqual(t, dark, bg.Rect.FillColor, "the background changed with the theme")
	assert.Equal(t, kit.ColBg3, bg.Rect.FillColor, "it uses the light palette now")
	assert.Equal(t, kit.ColOK.R, bg.Rect.StrokeColor.(color.NRGBA).R, "the accent follows the palette too")
}

// ---- Game with several emulators ----

// Play on a game with several emulators opens the "which emulator?" dialog
// (no countdown: the user pressed Play), and cancelling plays nothing.
func TestPlayAsksWhichEmulatorInADialog(t *testing.T) {
	h := newHarness(t)
	h.addEmulatorGame()
	h.open("/")
	tapButton(t, h, "Play")
	require.Eventually(t, func() bool {
		h.settle()
		return len(overlayButtons(h)) > 0
	}, 10*time.Second, 50*time.Millisecond)
	var names []string
	for _, c := range overlayWidgets[*kit.ChoiceTile](h) {
		names = append(names, c.Name)
	}
	assert.ElementsMatch(t, []string{"Without an emulator", "Eden", "Ryujinx"}, names, "a square tile for each way to play")
	stop := buttonWith(t, overlayButtons(h), "Stop countdown")
	assert.False(t, stop.Visible(), "nothing starts by itself, so there is no countdown to stop")

	tapOverlayButton(t, h, "Cancel")
	h.settle()
	require.Eventually(t, func() bool { h.settle(); return len(overlayButtons(h)) == 0 }, 10*time.Second, 50*time.Millisecond)
}

// "Sync all" belongs to the window: it keeps going, and is reported, when the
// library page is left while it runs.
func TestSyncAllOutlivesTheLibraryPage(t *testing.T) {
	h := newHarness(t)
	cloudDir := h.useLocalCloud()
	saves := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(saves, "a.sav"), []byte("save"), 0o644))
	g := h.addGame("Game A", saves)
	h.open("/")
	tapButton(t, h, "Sync all")
	h.u.Navigate("/settings")
	h.settle()
	assert.False(t, h.u.Store().SyncAll.Running, "finished")
	assert.FileExists(t, filepath.Join(cloudDir, "games", g.ID, "current", "saves", "a.sav"))
	assert.Contains(t, h.toasts(), "Synced 1 game")
}

// A running "Sync all" shows in the sidebar on every page but the library,
// which shows its own progress bar.
func TestSyncAllInSidebarExceptOnTheLibrary(t *testing.T) {
	h := newHarness(t)
	h.u.Store().SyncAll.Running = true
	h.u.Store().SyncAll.Progress = services.SyncAllProgress{Done: 1, Total: 3}
	sidebarSync := func() bool {
		for _, b := range widgetsOf[*widget.Button](h.u.Activity()) {
			if b.Text == "Syncing 1 of 3" {
				return true
			}
		}
		return false
	}
	h.open("/")
	assert.False(t, sidebarSync(), "not on the library")
	h.open("/settings")
	assert.True(t, sidebarSync(), "on other pages")
	h.open("/")
	assert.False(t, sidebarSync())
}

// askDelete opens the game page's delete dialog (from its "⋯" menu).
func askDelete(t *testing.T, h *harness) {
	t.Helper()
	gp, ok := h.u.Page().(*gamepage.Page)
	require.True(t, ok, "on a game page")
	g, _ := h.u.Store().Game(gp.GameID())
	var item *fyne.MenuItem
	for _, it := range gp.MoreMenu(g, false).Items {
		if it.Label == "Delete from library…" {
			item = it
		}
	}
	require.NotNil(t, item, "the menu offers to delete")
	item.Action()
	h.settle()
}

// Game page → editor → back: the router keeps the history across the two pages.
func TestGamePageNavigation(t *testing.T) {
	h := newHarness(t)
	g := h.addGame("Game A", t.TempDir())
	h.open("/")
	h.open("/game/" + g.ID)

	tapButton(t, h, "Edit")
	h.settle()
	assert.Equal(t, "/edit/"+g.ID, h.u.Route())
	tapButton(t, h, "Cancel") // nothing changed: leaves without asking
	h.settle()
	assert.Equal(t, "/game/"+g.ID, h.u.Route())

	tapButton(t, h, "Back")
	h.settle()
	assert.Equal(t, "/", h.u.Route())
}
