package library_test

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
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
	assert.Contains(t, h.Texts(), "Add your first game")
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

// The grid fills the width of the page: whatever the window's width, the games
// of a row reach its right edge instead of leaving an empty strip, and a wider
// window gives more columns once they fit.
func TestGridFillsTheWindowWidth(t *testing.T) {
	h := newHarness(t)
	names := make([]string, 0, 18)
	for i := range 18 {
		name := fmt.Sprintf("Game %02d", i)
		names = append(names, name)
		addGame(t, h, name)
	}
	columns := map[int]int{}
	for _, w := range []int{900, 1000, 1180, 1333, 1600, 1920, 2560} {
		h.Tester.SetSize(w, 1400)
		h.Settle()
		h.Settle()

		// Each tile's name starts at its left edge, inside the tile's padding.
		var rights []float32
		var left0 float32 = -1
		cols := 0
		var y0 float32 = -1
		for _, n := range names {
			r, ok := h.Tester.Find(n)
			if !ok {
				continue
			}
			if y0 < 0 {
				y0, left0 = r.Y, r.X
			}
			if r.Y == y0 {
				cols++
				rights = append(rights, r.X+r.W)
			}
		}
		require.Positive(t, cols, "width %d", w)
		columns[w] = cols
		edge := slices.Max(rights) + kit.S2 + 2 // the tile's padding and border
		// What is left to the window's right is the page's margin and a scroll bar.
		assert.Less(t, float32(w)-edge, kit.S8+30, "width %d: %d columns, the grid ends at %v (it starts at %v)", w, cols, edge, left0)
	}
	assert.Less(t, columns[900], columns[1920], "a wider window shows more games per row")
	assert.Less(t, columns[1920], columns[2560]+1)
}

// Every tile is the size of its cell, whatever the size of its cover picture: a
// small picture is scaled up, a big one down, and the cover keeps its 2:3 shape,
// instead of the tile following the picture.
func TestTilesAreTheSameSizeWhateverTheirCovers(t *testing.T) {
	h := newHarness(t)
	h.Tester.SetSize(1200, 1000)
	sizes := map[string][2]int{"Alpha": {40, 30}, "Bravo": {600, 900}, "Charlie": {300, 300}}
	for name, wh := range sizes {
		g := addGame(t, h, name)
		src := filepath.Join(t.TempDir(), "cover.png")
		img := image.NewRGBA(image.Rect(0, 0, wh[0], wh[1]))
		draw.Draw(img, img.Bounds(), image.NewUniform(color.RGBA{R: 200, G: 40, B: 40, A: 255}), image.Point{}, draw.Src)
		var buf bytes.Buffer
		require.NoError(t, png.Encode(&buf, img))
		require.NoError(t, os.WriteFile(src, buf.Bytes(), 0o644))
		_, err := h.Shell.Lib().SetCover(g.ID, src)
		require.NoError(t, err)
	}
	h.Shell.Store().Refresh()
	h.Settle()
	// The covers load in the background; the tiles must not change once they have.
	measure := func() (rects []ui.Rect) {
		for name := range sizes {
			r, ok := h.Tester.Find(name)
			require.True(t, ok, name)
			rects = append(rects, r)
		}
		return rects
	}
	before := measure()
	time.Sleep(300 * time.Millisecond)
	h.Settle()
	after := measure()

	for _, rects := range [][]ui.Rect{before, after} {
		for _, r := range rects {
			assert.InDelta(t, rects[0].W, r.W, 1, "the tiles are as wide as each other: %v", rects)
			assert.InDelta(t, rects[0].H, r.H, 1, "and as tall: %v", rects)
			// A 2:3 cover over its name, status and button is taller than 1.5 times the tile's width.
			assert.Greater(t, r.H, 1.5*r.W, "the cover is as tall as 2:3 makes it: %v", r)
		}
	}
	assert.InDelta(t, before[0].H, after[0].H, 1, "loading the pictures doesn't resize the tiles")
}
