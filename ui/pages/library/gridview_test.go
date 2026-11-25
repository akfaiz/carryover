package library_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/akfaiz/carryover/ui/internal/uitest"
	"github.com/akfaiz/carryover/ui/kit"
	page "github.com/akfaiz/carryover/ui/pages/library"
)

// addGames adds n games named "Game 0000", "Game 0001", ...
func addGames(t *testing.T, n int) (*uitest.Harness, []string) {
	t.Helper()
	h := newHarness(t)
	names := make([]string, n)
	for i := range n {
		names[i] = fmt.Sprintf("Game %04d", i)
		addGame(t, h, names[i])
	}
	return h, names
}

// shown returns the games of names whose tile is built.
func shown(h *uitest.Harness, names []string) []string {
	var out []string
	for _, n := range names {
		if _, ok := h.Tester.Find(n); ok {
			out = append(out, n)
		}
	}
	return out
}

// The grid builds the rows in view and a few around them, not every tile: a
// library of hundreds of games takes what a page of a few does.
func TestGridBuildsOnlyTheRowsInView(t *testing.T) {
	h, names := addGames(t, 90)
	h.Tester.SetSize(1280, 800)
	h.Settle()

	built := shown(h, names)
	assert.NotEmpty(t, built)
	assert.Less(t, len(built), 30, "90 games, a window of two rows")
	assert.Equal(t, names[0], built[0], "the first row is the one in view")

	// Scrolled to the end, the last games are built and the first are not.
	h.Tester.Scroll(600, 500, 0, 1e6)
	h.Settle()
	built = shown(h, names)
	assert.Contains(t, built, names[len(names)-1])
	assert.NotContains(t, built, names[0])
	assert.Less(t, len(built), 30)
}

// A row of the grid is as high as its tiles, whatever the width: a row that was
// taller left gaps, a shorter one cut the tiles or overlapped the next. The
// distance between two rows is a tile and the gap between tiles.
func TestTileHeightMatchesTheLayout(t *testing.T) {
	h, names := addGames(t, 24)
	for _, w := range []int{900, 1000, 1180, 1333, 1700, 2200} {
		h.Tester.SetSize(w, 1600)
		h.Settle()
		r0, ok := h.Tester.Find(names[0])
		require.True(t, ok)
		r1, ok := h.Tester.Find(names[1])
		require.True(t, ok)
		cols := 0
		var below = r0
		for i, n := range names {
			r, ok := h.Tester.Find(n)
			if !ok {
				break
			}
			if r.Y > r0.Y+1 {
				cols, below = i, r
				break
			}
		}
		require.Positive(t, cols, "width %d: a second row is built", w)
		if cols == 1 {
			continue // one column: no tile to measure the width by
		}
		tile := r1.X - r0.X - kit.S4 // the distance between two columns is a tile and a gap
		assert.InDelta(t, page.TileHeight(tile)+kit.S4, below.Y-r0.Y, 0.6, "width %d, tile %.1f", w, tile)
	}
}

// The pad reaches tiles that are below the window: moving down builds the rows
// there, and the focus lands on the game in the same column. Every tile is two
// stops, the cover and the Play button under it.
func TestPadReachesTilesBelowTheWindow(t *testing.T) {
	h, names := addGames(t, 60)
	h.Tester.SetSize(1000, 700)
	h.Settle()
	cols := 3 // at 1000 wide

	nav := h.Shell.Navigator()
	nav.Press(kit.PadDown)
	h.Settle()
	require.True(t, h.Tester.Focused(names[0]), "the first press starts on the first game")
	for row := 1; row <= 6; row++ {
		// A tile is two stops, its cover and its Play button.
		nav.Press(kit.PadDown)
		h.Settle()
		nav.Press(kit.PadDown)
		h.Settle()
		want := names[row*cols]
		require.True(t, h.Tester.Focused(want), "row %d: focus is on %s", row, want)
	}
	nav.Press(kit.PadAccept)
	h.Settle()
	assert.Contains(t, h.Shell.Route(), "/game/", "A opens the game the pad reached")
}
