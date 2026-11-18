package ui

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/widget"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/akfaiz/carryover/ui/kit"
	libpage "github.com/akfaiz/carryover/ui/pages/library"
)

// The library starts a gamepad on the first game; A opens it, and its page
// starts on Back, not Play: an A pressed once more must not start the game.
// Play is just below.
func TestGamepadStartsOnTheFirstGame(t *testing.T) {
	h := newHarness(t)
	h.addGame("Hades", t.TempDir())
	h.addGame("Celeste", t.TempDir())
	h.open("/")

	h.u.Navigator().FocusDefault()
	tiles := pageWidgets[*libpage.Tile](h)
	require.NotEmpty(t, tiles)
	var first *libpage.Tile
	for _, tile := range tiles {
		if tile.Game().Name == "Celeste" {
			first = tile
		}
	}
	require.NotNil(t, first)
	require.Same(t, first.Open(), h.u.Window().Canvas().Focused(), "sorted by name: Celeste first")

	h.u.Navigator().Apply(kit.PadAccept)
	h.settle()
	require.Equal(t, "/game/"+first.Game().ID, h.u.Route())
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
