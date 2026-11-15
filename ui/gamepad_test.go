package ui

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/widget"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/akfaiz/carryover/internal/ui/kit"
)

func TestSectionOf(t *testing.T) {
	for route, want := range map[string]string{
		"/": "/", "/game/x": "/", "/edit/x": "/", "/new/emulator": "/",
		"/emulators": "/emulators", "/cloud": "/cloud", "/settings": "/settings", "/trash": "/trash",
	} {
		assert.Equal(t, want, sectionOf(route), route)
	}
}

// LB and RB step through the sidebar; B goes back from a game to the library.
func TestGamepadSectionsAndBack(t *testing.T) {
	h := newHarness(t)
	g := h.addGame("Celeste", t.TempDir())
	h.settle()

	h.u.nav.Apply(kit.PadTabNext)
	assert.Equal(t, "/emulators", h.u.route)
	h.u.nav.Apply(kit.PadTabNext)
	assert.Equal(t, "/cloud", h.u.route)
	h.u.nav.Apply(kit.PadTabPrev)
	h.u.nav.Apply(kit.PadTabPrev)
	assert.Equal(t, "/", h.u.route)
	h.u.nav.Apply(kit.PadTabPrev)
	assert.Equal(t, "/settings", h.u.route, "goes round")

	h.open("/")
	h.open("/game/" + g.ID)
	h.u.nav.Apply(kit.PadBack)
	assert.Equal(t, "/", h.u.route)
	h.u.nav.Apply(kit.PadBack)
	assert.Equal(t, "/", h.u.route, "B on the library stays there")
}

// The library starts a gamepad on the first game; A opens it, and its page
// starts on Play.
func TestGamepadStartsOnTheFirstGame(t *testing.T) {
	h := newHarness(t)
	h.addGame("Hades", t.TempDir())
	h.addGame("Celeste", t.TempDir())
	h.open("/")

	h.u.nav.FocusDefault()
	tiles := pageWidgets[*gameTile](h)
	require.NotEmpty(t, tiles)
	var first *gameTile
	for _, tile := range tiles {
		if tile.g.Name == "Celeste" {
			first = tile
		}
	}
	require.NotNil(t, first)
	require.Same(t, first.open, h.u.win.Canvas().Focused(), "sorted by name: Celeste first")

	h.u.nav.Apply(kit.PadAccept)
	h.settle()
	require.Equal(t, "/game/"+first.g.ID, h.u.route)
	h.u.nav.FocusDefault()
	b, ok := h.u.win.Canvas().Focused().(*widget.Button)
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
		h.u.nav.FocusDefault()
		f := h.u.win.Canvas().Focused()
		require.NotNil(t, f, route)
		for path, b := range h.u.navBtns {
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
	require.NotEmpty(t, h.u.win.Canvas().Overlays().List())
	h.u.nav.Apply(kit.PadBack)
	assert.Empty(t, h.u.win.Canvas().Overlays().List())
	assert.Equal(t, "/settings", h.u.route)
}

// With nothing focused, the arrow keys reach the window and pick a control.
func TestArrowKeysPickAControl(t *testing.T) {
	h := newHarness(t)
	h.addGame("Celeste", t.TempDir())
	h.open("/")
	c := h.u.win.Canvas()
	c.Unfocus()
	c.OnTypedKey()(&fyne.KeyEvent{Name: fyne.KeyDown})
	assert.IsType(t, &kit.Tappable{}, c.Focused())
}
