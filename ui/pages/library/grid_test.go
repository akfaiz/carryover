package library

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// A tile is never narrower than TileMin, and the tiles of a row share the whole
// width: the more room, the wider they get, until another column fits.
func TestGridColumnsFillTheWidth(t *testing.T) {
	const gap = 16
	for width := float32(400); width <= 3000; width += 7 {
		cols := GridColumns(width, gap)
		tile := (width - float32(cols-1)*gap) / float32(cols)
		assert.GreaterOrEqual(t, tile, TileMin, "width %v", width)
		if cols > 1 {
			assert.LessOrEqual(t, tile, TileMax, "a tile never gets wider than the most, width %v", width)
		}
		// One more column would not have fit.
		assert.Less(t, width, float32(cols+1)*TileMin+float32(cols)*gap, "width %v", width)
	}
}

func TestGridColumnsEdges(t *testing.T) {
	assert.Equal(t, 1, GridColumns(0, 16), "no width yet")
	assert.Equal(t, 1, GridColumns(100, 16), "a width narrower than a tile")
	assert.Equal(t, 1, GridColumns(TileMin, 16))
	assert.Equal(t, 1, GridColumns(2*TileMin+15, 16), "two tiles of the least width don't quite fit")
	assert.Equal(t, 2, GridColumns(2*TileMin+16, 16), "two tiles of the least width fit exactly")
}
