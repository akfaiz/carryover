package library_test

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/akfaiz/carryover/internal/library"
	"github.com/akfaiz/carryover/services"
	"github.com/akfaiz/carryover/ui/env"
	"github.com/akfaiz/carryover/ui/internal/uitest"
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
		Slots: []library.SaveSlot{{Key: "saves", Label: "Saves"}},
		Device: services.DeviceInput{
			Paths: map[string]string{"saves": t.TempDir()}, Exe: "/x/game.exe", LaunchMode: library.LaunchDirect,
		},
	})
	require.NoError(t, err)
	h.Shell.Store().Refresh()
	return g
}

func tileNames(h *uitest.Harness) []string {
	var out []string
	for _, tile := range uitest.PageWidgets[*page.Tile](h) {
		out = append(out, tile.Game().Name)
	}
	return out
}

func search(t *testing.T, h *uitest.Harness, text string) {
	t.Helper()
	for _, e := range uitest.PageWidgets[*widget.Entry](h) {
		if e.PlaceHolder == "Search…" {
			e.SetText(text)
			h.Settle()
			return
		}
	}
	require.Fail(t, "search field not found")
}

// The grid follows the library, the search and the order.
func TestPageShowsSearchesAndSortsGames(t *testing.T) {
	h := newHarness(t)
	addGame(t, h, "Hades")
	addGame(t, h, "Celeste")
	h.Settle()
	assert.Equal(t, []string{"Celeste", "Hades"}, tileNames(h), "sorted by name")

	search(t, h, "hades")
	assert.Equal(t, []string{"Hades"}, tileNames(h))

	search(t, h, "zelda")
	assert.Empty(t, tileNames(h))
	assert.NotEmpty(t, uitest.PageWidgets[*widget.Button](h), "a Clear search button explains the empty grid")
}

// Tiles are kept and updated in place: a refresh doesn't build them again, so
// covers aren't reloaded and focus survives.
func TestPageKeepsItsTiles(t *testing.T) {
	h := newHarness(t)
	addGame(t, h, "Game A")
	h.Settle()
	before := uitest.PageWidgets[*page.Tile](h)
	require.Len(t, before, 1)

	h.Shell.Store().Refresh()
	h.Settle()
	after := uitest.PageWidgets[*page.Tile](h)
	require.Len(t, after, 1)
	assert.Same(t, before[0], after[0])
}

// A tile opens its game.
func TestPageTileOpensTheGame(t *testing.T) {
	h := newHarness(t)
	g := addGame(t, h, "Game A")
	h.Settle()
	tiles := uitest.PageWidgets[*page.Tile](h)
	require.Len(t, tiles, 1)
	test.Tap(tiles[0].Open())
	h.Settle()
	assert.Equal(t, "/game/"+g.ID, h.Shell.Route())
}

// The filter row gets its height when the games load after the page was first
// drawn (on startup the page is drawn before the library is read), so it
// doesn't overlap the header or the grid.
func TestPageRowsGetTheirHeightWhenGamesLoad(t *testing.T) {
	h := newHarness(t)
	h.Open("/")
	p, ok := h.Shell.Page().(*page.Page)
	require.True(t, ok)
	require.Zero(t, p.ChipsHost().MinSize().Height, "no games yet: no row")

	addGame(t, h, "Game A")
	h.Settle()
	h.Shell.Window().Resize(fyne.NewSize(1000, 700))
	row := p.ChipsHost()
	require.Positive(t, row.MinSize().Height)
	assert.Equal(t, row.MinSize().Height, row.Size().Height, "laid out with its height")
	assert.GreaterOrEqual(t, p.GridHost().Position().Y, row.Position().Y+row.Size().Height, "the grid starts below it")
}
