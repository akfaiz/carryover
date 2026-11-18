package ui

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/akfaiz/carryover/ui/kit"
	"github.com/akfaiz/carryover/ui/shell"
)

// LB and RB step through the sidebar; B goes back from a game to the library.
func TestGamepadSectionsAndBack(t *testing.T) {
	h := newHarness(t)
	g := h.addGame("Celeste", t.TempDir())
	h.settle()

	h.u.Navigator().Apply(kit.PadTabNext)
	assert.Equal(t, "/emulators", h.u.Route())
	h.u.Navigator().Apply(kit.PadTabNext)
	assert.Equal(t, "/cloud", h.u.Route())
	h.u.Navigator().Apply(kit.PadTabPrev)
	h.u.Navigator().Apply(kit.PadTabPrev)
	assert.Equal(t, "/", h.u.Route())
	h.u.Navigator().Apply(kit.PadTabPrev)
	assert.Equal(t, "/settings", h.u.Route(), "goes round")

	h.open("/")
	h.open("/game/" + g.ID)
	h.u.Navigator().Apply(kit.PadBack)
	assert.Equal(t, "/", h.u.Route())
	h.u.Navigator().Apply(kit.PadBack)
	assert.Equal(t, "/", h.u.Route(), "B on the library stays there")
}

// The library starts a gamepad on the first game; A opens it, and its page
// starts on Back, not Play: an A pressed once more must not start the game.
// Play is just below.
func TestGamepadStartsOnTheFirstGame(t *testing.T) {
	h := newHarness(t)
	h.addGame("Hades", t.TempDir())
	h.addGame("Celeste", t.TempDir())
	h.open("/")

	h.u.Navigator().FocusDefault()
	tiles := pageWidgets[*gameTile](h)
	require.NotEmpty(t, tiles)
	var first *gameTile
	for _, tile := range tiles {
		if tile.g.Name == "Celeste" {
			first = tile
		}
	}
	require.NotNil(t, first)
	require.Same(t, first.open, h.u.Window().Canvas().Focused(), "sorted by name: Celeste first")

	h.u.Navigator().Apply(kit.PadAccept)
	h.settle()
	require.Equal(t, "/game/"+first.g.ID, h.u.Route())
	h.u.Navigator().FocusDefault()
	b, ok := h.u.Window().Canvas().Focused().(*widget.Button)
	require.True(t, ok)
	assert.Equal(t, "Back", b.Text)
	h.u.Navigator().Apply(kit.PadDown)
	b, ok = h.u.Window().Canvas().Focused().(*widget.Button)
	require.True(t, ok)
	assert.Equal(t, "Play", b.Text)
}

// On a page without a start of its own, the gamepad starts on the page's first
// control, not on the sidebar's Library button (the window's top left); a page
// with no controls starts on its own sidebar entry.
func TestGamepadStartsOnThePageNotTheSidebar(t *testing.T) {
	h := newHarness(t)
	for _, route := range []string{"/emulators", "/cloud", "/trash", "/settings"} {
		h.open(route)
		h.u.Navigator().FocusDefault()
		f := h.u.Window().Canvas().Focused()
		require.NotNil(t, f, route)
		for path, b := range h.u.NavButtons() {
			if path != route {
				assert.NotSame(t, b, f, "%s: focus went to the sidebar's %s", route, path)
			}
		}
	}
}

// A "Sync all" problem list and other dialogs close with B, not the page.
func TestGamepadClosesDialogs(t *testing.T) {
	h := newHarness(t)
	h.open("/settings")
	showAddGame(h.u)
	require.NotEmpty(t, h.u.Window().Canvas().Overlays().List())
	h.u.Navigator().Apply(kit.PadBack)
	assert.Empty(t, h.u.Window().Canvas().Overlays().List())
	assert.Equal(t, "/settings", h.u.Route())
}

// With nothing focused, the arrow keys reach the window and pick a control.
func TestArrowKeysPickAControl(t *testing.T) {
	h := newHarness(t)
	h.addGame("Celeste", t.TempDir())
	h.open("/")
	c := h.u.Window().Canvas()
	c.Unfocus()
	c.OnTypedKey()(&fyne.KeyEvent{Name: fyne.KeyDown})
	assert.IsType(t, &kit.Tappable{}, c.Focused())
}

// The View button switches full screen and remembers it, apart for Carryover
// started by Steam and started by hand.
func TestGamepadViewSwitchesFullScreen(t *testing.T) {
	t.Setenv("SteamGameId", "")
	t.Setenv("SteamClientLaunch", "")
	t.Setenv("XDG_CURRENT_DESKTOP", "")
	t.Setenv("GAMESCOPE_WAYLAND_DISPLAY", "")
	h := newHarness(t)
	prefs := h.u.App().Preferences()
	require.False(t, h.u.Window().FullScreen())
	h.u.Navigator().Apply(kit.PadView)
	assert.True(t, h.u.Window().FullScreen())
	assert.True(t, prefs.Bool(shell.PrefFullScreen))
	assert.False(t, prefs.Bool(shell.PrefFullScreenSteam), "started by hand: Steam's choice stays")
	h.u.Navigator().Apply(kit.PadView)
	assert.False(t, h.u.Window().FullScreen())
	assert.False(t, prefs.Bool(shell.PrefFullScreen))

	t.Setenv("SteamGameId", "123")
	t.Setenv("SteamTenfoot", "1")
	h.u.Window().SetFullScreen(false)
	h.u.ApplyFullScreen()
	assert.True(t, h.u.Window().FullScreen(), "from Big Picture it starts full screen")
	h.u.ToggleFullScreen()
	h.u.ApplyFullScreen()
	assert.False(t, h.u.Window().FullScreen(), "until switched off")
}

// exitButton is the sidebar's Exit button, or nil.
func exitButton(h *harness) *widget.Button {
	for _, b := range widgetsOf[*widget.Button](h.u.Sidebar()) {
		if b.Text == "Exit" {
			return b
		}
	}
	return nil
}

// Exit is in the sidebar where the window's close button can't be reached:
// full screen, Game Mode, started by Steam. It closes like the window's close
// button: asking first while a sync runs.
func TestExitButton(t *testing.T) {
	t.Setenv("SteamGameId", "")
	t.Setenv("SteamClientLaunch", "")
	t.Setenv("XDG_CURRENT_DESKTOP", "")
	t.Setenv("GAMESCOPE_WAYLAND_DISPLAY", "")
	h := newHarness(t)
	h.settle()
	assert.Nil(t, exitButton(h), "a desktop window has its own close button")

	h.u.ToggleFullScreen()
	exit := exitButton(h)
	require.NotNil(t, exit, "full screen")
	assert.Contains(t, kit.Focusables(h.u.Window().Content()), fyne.Focusable(exit), "the D-pad reaches it")

	h.u.Store().SyncAll.Running = true
	test.Tap(exit)
	assert.Contains(t, overlayText(h), "A game is still running or a sync is in progress.", "asks while a sync runs")
	h.u.Navigator().Apply(kit.PadBack)
	assert.Empty(t, h.u.Window().Canvas().Overlays().List(), "B keeps Carryover open")
	h.u.Store().SyncAll.Running = false

	h.u.ToggleFullScreen()
	assert.Nil(t, exitButton(h), "back in a window")
	t.Setenv("SteamGameId", "123")
	h.u.RefreshSidebar()
	assert.NotNil(t, exitButton(h), "started by Steam")
}
