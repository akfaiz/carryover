package library_test

import (
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/akfaiz/carryover/services"
	"github.com/akfaiz/carryover/ui/env"
	"github.com/akfaiz/carryover/ui/internal/uitest"
	"github.com/akfaiz/carryover/ui/kit"
	page "github.com/akfaiz/carryover/ui/pages/library"
	"github.com/akfaiz/carryover/ui/shell"
)

// newHarness shows only the library page in a shell: the page is tested
// without the rest of the application.
func newHarness(t *testing.T) *uitest.Harness {
	t.Helper()
	h := uitest.New(t, shell.Config{
		Page:         func(e env.Env, _ string) env.Page { return page.New(e) },
		AddGame:      func(env.Env) {},
		SyncProblems: func(env.Env, int, []services.SyncResult) {},
	})
	h.Settle()
	return h
}

func addGame(t *testing.T, h *uitest.Harness, name string) services.GameView {
	t.Helper()
	g, err := h.Shell.Lib().Save(services.GameInput{
		Name:  name,
		Slots: []services.SaveSlot{{Key: "saves", Label: "Saves"}},
		Device: services.DeviceInput{
			Paths: map[string]string{"saves": t.TempDir()}, Exe: "/x/game.exe", LaunchMode: services.LaunchDirect,
		},
	})
	require.NoError(t, err)
	h.Shell.Store().Refresh()
	h.Settle()
	return g
}

// order returns the names among texts in the order they are drawn, once each
// (a tile's name is its text and its accessible name).
func order(h *uitest.Harness, names ...string) []string {
	var out []string
	for _, tx := range h.Texts() {
		for _, n := range names {
			if tx == n && !slices.Contains(out, n) {
				out = append(out, n)
			}
		}
	}
	return out
}

func typeSearch(t *testing.T, h *uitest.Harness, text string) {
	t.Helper()
	require.NoError(t, h.Tester.Click("Search…"))
	h.Tester.Type(text)
	h.Settle()
}

func TestEmptyLibraryOffersTheFirstGame(t *testing.T) {
	h := newHarness(t)
	assert.Contains(t, h.Texts(), "No games yet")
	assert.Contains(t, h.Texts(), "＋ Add your first game")
	assert.Contains(t, h.Texts(), "Set up cloud storage", "no cloud yet")
	h.Click("Set up cloud storage")
	assert.Equal(t, "/cloud", h.Shell.Route())
}

// The grid follows the library, the search and the order.
func TestPageShowsSearchesAndSortsGames(t *testing.T) {
	h := newHarness(t)
	addGame(t, h, "Hades")
	addGame(t, h, "Celeste")
	assert.Equal(t, []string{"Celeste", "Hades"}, order(h, "Hades", "Celeste"), "sorted by name")

	typeSearch(t, h, "hades")
	assert.Equal(t, []string{"Hades"}, order(h, "Hades", "Celeste"))

	typeSearch(t, h, "zelda")
	assert.Empty(t, order(h, "Hades", "Celeste"))
	assert.Contains(t, h.Texts(), "No games match “zelda”.")
	h.Click("Clear search")
	assert.Len(t, order(h, "Hades", "Celeste"), 2, "Clear search shows the games again")
}

// A tile opens its game; Play on a game that isn't set up here sets it up.
func TestPageTileOpensTheGame(t *testing.T) {
	h := newHarness(t)
	g := addGame(t, h, "Game A")
	h.Click("Game A")
	assert.Equal(t, "/game/"+g.ID, h.Shell.Route())
}

func TestPageSetUpGoesToTheEditor(t *testing.T) {
	h := newHarness(t)
	g, err := h.Shell.Lib().Save(services.GameInput{Name: "Needs setup", Slots: []services.SaveSlot{{Key: "saves", Label: "Saves"}}})
	require.NoError(t, err)
	h.Shell.Store().Refresh()
	h.Settle()
	assert.Contains(t, h.Texts(), "Needs setup")
	h.Click("Set up")
	assert.Equal(t, "/edit/"+g.ID, h.Shell.Route())
}

// Sorting and filtering are remembered in the preferences.
func TestPageSortChoiceIsRemembered(t *testing.T) {
	h := newHarness(t)
	addGame(t, h, "Hades")
	addGame(t, h, "Celeste")
	h.Click("Sort by name")
	h.Click("Sort by last sync")
	assert.Equal(t, "recent", h.Shell.Prefs().StringWithFallback("library.sort", ""))
}

// The pad: the first press starts on the first game, and A on a tile opens it.
func TestPadStartsOnTheFirstGameAndOpensIt(t *testing.T) {
	h := newHarness(t)
	g := addGame(t, h, "Celeste")
	addGame(t, h, "Hades")

	nav := h.Shell.Navigator()
	nav.Press(kit.PadDown)
	h.Settle()
	assert.True(t, h.Tester.Focused("Celeste"), "the page's default is its first game, not the sidebar")
	nav.Press(kit.PadAccept)
	h.Settle()
	assert.Equal(t, "/game/"+g.ID, h.Shell.Route())
}
