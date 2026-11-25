package flows_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/akfaiz/carryover/ui/flows"
	"github.com/akfaiz/carryover/ui/internal/uitest"
	"github.com/akfaiz/carryover/ui/kit"
)

const importPrefix = "Import Game "

// importRows are n games named "Import Game 0000", "Import Game 0001", ...
func importRows(n int) []flows.ImportRow {
	rows := make([]flows.ImportRow, n)
	for i := range rows {
		rows[i] = flows.NewImportRow(fmt.Sprintf("%s%04d", importPrefix, i), "", fmt.Sprintf("/new/steam/%d", i))
	}
	return rows
}

// built returns the games of the import dialog whose row is built, once each.
func built(h *uitest.Harness) []string {
	var out []string
	seen := map[string]bool{}
	for _, tx := range h.Texts() {
		if strings.HasPrefix(tx, importPrefix) && !seen[tx] {
			seen[tx] = true
			out = append(out, tx)
		}
	}
	return out
}

// A launcher's library can be hundreds of games: the dialog builds the rows in
// view, not all of them, and the end of the list is still reached.
func TestImportBuildsOnlyTheRowsInView(t *testing.T) {
	h := newHarness(t)
	flows.ShowImportRows(h.Shell, importRows(500))
	h.Settle()

	shown := built(h)
	require.NotEmpty(t, shown)
	assert.Less(t, len(shown), 30, "500 games, a list of a few rows high")
	assert.Equal(t, importPrefix+"0000", shown[0])

	h.Tester.Scroll(500, 400, 0, 1e6)
	h.Settle()
	shown = built(h)
	assert.Contains(t, shown, importPrefix+"0499", "the end of the list")
	assert.NotContains(t, shown, importPrefix+"0000")
	assert.Less(t, len(shown), 30)
}

// The filter narrows the list to the games that match, whatever their place in
// the launcher's library, and picking one opens that game, not the one at the
// same place in the whole list.
func TestImportFilterThenPick(t *testing.T) {
	h := newHarness(t)
	flows.ShowImportRows(h.Shell, importRows(500))
	h.Settle()

	require.NoError(t, h.Tester.Click("Filter…"))
	h.Tester.Type("0420")
	h.Settle()
	assert.Equal(t, []string{importPrefix + "0420"}, built(h))

	h.Click(importPrefix + "0420")
	assert.Equal(t, "/new/steam/420", h.Shell.Route())
	assert.False(t, h.Shell.Dialogs().Open(), "picking a game closes the dialog")
}

// The filter is not case sensitive, and a filter that nothing matches leaves the
// list empty.
func TestImportFilterIgnoresCaseAndMayMatchNothing(t *testing.T) {
	h := newHarness(t)
	flows.ShowImportRows(h.Shell, importRows(30))
	h.Settle()
	require.NoError(t, h.Tester.Click("Filter…"))
	h.Tester.Type("IMPORT GAME 002")
	h.Settle()
	shown := built(h)
	require.NotEmpty(t, shown)
	assert.Equal(t, importPrefix+"0020", shown[0])
	for _, name := range shown {
		assert.True(t, strings.HasPrefix(name, importPrefix+"002"), "%s matches 002", name)
	}

	h.Tester.Type("x")
	h.Settle()
	assert.Empty(t, built(h))
}

// A game the library already has opens its page.
func TestImportOfAGameTheLibraryHasOpensItsPage(t *testing.T) {
	h := newHarness(t)
	flows.ShowImportRows(h.Shell, []flows.ImportRow{
		flows.NewImportRow("Celeste", "abc", "/new/steam/1"),
		flows.NewImportRow("Hades", "", "/new/steam/2"),
	})
	h.Settle()
	assert.Contains(t, h.Texts(), "In library")
	h.Click("Celeste")
	assert.Equal(t, "/game/abc", h.Shell.Route())
}

func TestImportWithNoGamesSaysSo(t *testing.T) {
	h := newHarness(t)
	flows.ShowImportRows(h.Shell, nil)
	h.Settle()
	assert.Contains(t, h.Texts(), "None.")
	assert.Empty(t, built(h))
}

// The pad moves through the list a row at a time, also past the rows that were
// not built until it got near them.
func TestImportPadWalksUpTheList(t *testing.T) {
	h := newHarness(t)
	flows.ShowImportRows(h.Shell, importRows(200))
	h.Settle()
	h.Tester.Scroll(500, 400, 0, 3000) // well below the first rows
	h.Settle()

	focusedRow := func() int {
		for _, name := range built(h) {
			if h.Tester.Focused(name) {
				var i int
				_, err := fmt.Sscanf(strings.TrimPrefix(name, importPrefix), "%d", &i)
				require.NoError(t, err)
				return i
			}
		}
		return -1
	}
	nav := h.Shell.Navigator()
	nav.Press(kit.PadUp) // from Close, the control the dialog starts on
	h.Settle()
	row := focusedRow()
	require.Positive(t, row, "the pad reached a row of the list")
	for step := 1; step <= 12; step++ {
		nav.Press(kit.PadUp)
		h.Settle()
		require.Equal(t, row-1, focusedRow(), "step %d: one row up", step)
		row--
	}
}
