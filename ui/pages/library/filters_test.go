package library_test

import (
	"strings"
	"testing"

	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/akfaiz/carryover/services"
	"github.com/akfaiz/carryover/ui/internal/uitest"
)

// libraryFilter is the library's filter as stored in the preferences.
func libraryFilter(h *uitest.Harness) string {
	return h.Shell.App().Preferences().StringWithFallback("library.filter", "all")
}

func setLibraryFilter(h *uitest.Harness, f string) {
	h.Shell.App().Preferences().SetString("library.filter", f)
}

// An empty search says so and offers to clear it.
func TestLibrarySearchWithoutMatches(t *testing.T) {
	h := newHarness(t)
	h.AddGame("Celeste", t.TempDir())
	h.Open("/")
	uitest.EntryByPlaceholder(t, h, "Search…").SetText("zzz")
	h.Settle()
	assert.Contains(t, strings.Join(uitest.LabelTexts(h), "\n"), "No games match")
	uitest.TapButton(t, h, "Clear search")
	h.Settle()
	assert.Equal(t, []string{"Celeste"}, tileNames(h))
}

// "Needs attention" shows only the games the user has to do something about,
// and the choice is remembered.
func TestLibraryNeedsAttentionFilter(t *testing.T) {
	h := newHarness(t)
	h.AddGame("Ready", t.TempDir())
	_, err := h.Shell.Lib().Save(services.GameInput{Name: "Not set up", Slots: []services.SaveSlot{{Key: "saves"}}})
	require.NoError(t, err)
	h.Shell.Store().Refresh()
	h.Open("/")
	assert.Len(t, tileNames(h), 2)

	uitest.TapButton(t, h, "Needs attention (1)")
	h.Settle()
	assert.Equal(t, []string{"Not set up"}, tileNames(h))
	assert.Equal(t, "attention", libraryFilter(h))

	h.Open("/settings")
	h.Open("/")
	assert.Equal(t, []string{"Not set up"}, tileNames(h), "the filter is kept")
	uitest.TapButton(t, h, "All (2)")
	h.Settle()
	assert.Len(t, tileNames(h), 2)
}

// "Ready to play" shows only the games that can be played on this device.
func TestLibraryReadyToPlayFilter(t *testing.T) {
	h := newHarness(t)
	h.AddGame("Ready", t.TempDir())
	_, err := h.Shell.Lib().Save(services.GameInput{Name: "Not set up", Slots: []services.SaveSlot{{Key: "saves"}}})
	require.NoError(t, err)
	h.Shell.Store().Refresh()
	h.Open("/")

	uitest.TapButton(t, h, "Ready to play (1)")
	h.Settle()
	assert.Equal(t, []string{"Ready"}, tileNames(h))
	assert.Equal(t, "ready", libraryFilter(h))
	uitest.TapButton(t, h, "All (2)")
	h.Settle()
	assert.Len(t, tileNames(h), 2)
}

// With no game playable here, the filter says so and offers all games.
func TestLibraryReadyToPlayFilterEmpty(t *testing.T) {
	h := newHarness(t)
	_, err := h.Shell.Lib().Save(services.GameInput{Name: "Not set up", Slots: []services.SaveSlot{{Key: "saves"}}})
	require.NoError(t, err)
	h.Shell.Store().Refresh()
	setLibraryFilter(h, "ready")
	h.Open("/")
	assert.Empty(t, tileNames(h))
	assert.Contains(t, strings.Join(uitest.LabelTexts(h), "\n"), "No game can be played on this device yet")
	uitest.TapButton(t, h, "Show all games")
	h.Settle()
	assert.Equal(t, []string{"Not set up"}, tileNames(h))
}

// With nothing needing attention the filters are hidden: there is nothing to
// filter for.
func TestLibraryHidesFiltersWhenNothingNeedsAttention(t *testing.T) {
	h := newHarness(t)
	h.AddGame("Ready", t.TempDir())
	h.Open("/")
	for _, b := range uitest.PageWidgets[*widget.Button](h) {
		assert.NotContains(t, b.Text, "Needs attention")
		assert.NotContains(t, b.Text, "All (")
	}
	assert.NotEmpty(t, uitest.PageWidgets[*widget.Select](h), "sorting is still there")

	// Turned on while a game needed attention, it stays so it can be turned off.
	setLibraryFilter(h, "attention")
	h.Open("/settings")
	h.Open("/")
	uitest.TapButton(t, h, "All (1)")
	h.Settle()
	assert.Equal(t, []string{"Ready"}, tileNames(h))
}

// A game without a path on this device shows "Set up", which opens the editor.
func TestLibraryTileSetUpOpensEditor(t *testing.T) {
	h := newHarness(t)
	g, err := h.Shell.Lib().Save(services.GameInput{Name: "Game A", Slots: []services.SaveSlot{{Key: "saves"}}})
	require.NoError(t, err)
	h.Shell.Store().Refresh()
	h.Open("/")
	uitest.TapButton(t, h, "Set up")
	h.Settle()
	assert.Equal(t, "/edit/"+g.ID, h.Shell.Route())
}

// The library tile has a single Play button; the choice is asked in a dialog.
func TestTileHasNoEmulatorDropdown(t *testing.T) {
	h := newHarness(t)
	h.AddEmulatorGame()
	h.Open("/")
	for _, b := range uitest.PageWidgets[*widget.Button](h) {
		assert.NotEqual(t, theme.MenuDropDownIcon(), b.Icon, "no dropdown next to Play")
	}
}
