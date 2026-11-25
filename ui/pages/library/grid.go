package library

import "github.com/akfaiz/carryover/ui/kit"

// The games' grid fills the width of the page: it has as many columns as fit
// tiles of at least TileMin, and the tiles share the room that is left, so a
// wider window makes them wider (and, once another column fits, adds one)
// instead of leaving an empty strip at the right.
const (
	// TileMin is the narrowest a tile gets: a 152 DIP cover and the padding
	// around it.
	TileMin float32 = 168
	// TileMax is the widest a tile gets. Two columns never reach it (they stay
	// under (3*TileMin+16)/2), so only a lone column (a window narrower than the
	// app allows) is ever capped.
	TileMax float32 = 264
	// CoverRatio is a cover's height over its width.
	CoverRatio float32 = 1.5
)

// GridColumns is how many columns a grid width DIPs wide has, with gap DIPs
// between them: as many as fit tiles of TileMin. Without a width (zero, before
// the first layout) it is one.
func GridColumns(width, gap float32) int {
	if width <= 0 {
		return 1
	}
	return max(1, int((width+gap)/(TileMin+gap)))
}

// The grid is a ui.GridView, which builds only the rows in view. It lays its
// items out its own way: a cell is a box with a border, cells are the theme's
// gap apart (a unit of two), and a row has half a gap of room above and below.
// A tile is drawn inside its cell with a margin that makes up the difference to
// the room tiles had when the grid was a plain ui.Grid: kit.S4 between them.
const (
	// cellBorder is the width of the border of a GridView cell, clear unless
	// the cell is chosen.
	cellBorder float32 = 2
	// tileChrome is the height of a tile besides its cover: the button's
	// padding, the name's two lines, the badge row, the gaps and the Play
	// button. TestTileHeight checks it against the layout.
	tileChrome float32 = 139.27
	// tilePad is the padding between the edge of the tile's button and its cover.
	tilePad = kit.S2
)

// cellMargin is the margin a tile has in its cell for tiles to be kit.S4 apart
// when the grid's gap is gap.
func cellMargin(gap float32) float32 { return max(0, (kit.S4-gap-2*cellBorder)/2) }

// cellExtra is the width, and the height, a cell has around its tile.
func cellExtra(gap float32) float32 { return 2 * (cellBorder + cellMargin(gap)) }

// rowInset is the room above the first row and below the last of the grid, from
// the edge of its content to the edge of a tile.
func rowInset(gap float32) float32 { return gap/2 + cellBorder + cellMargin(gap) }

// TileWidth is the width of a tile of a grid whose rows are width DIPs wide
// with gap between its cells: the cells share the width, as the columns
// GridColumns counts, and a lone column's tile stops at TileMax.
func TileWidth(width, gap float32) float32 {
	extra := cellExtra(gap)
	cols := GridColumns(width-extra, kit.S4)
	return min((width-float32(cols-1)*gap)/float32(cols)-extra, TileMax)
}

// TileHeight is the height of a tile width DIPs wide: the cover has CoverRatio
// of the width inside the button's padding, and the rest does not depend on it.
func TileHeight(width float32) float32 { return CoverRatio*(width-2*tilePad) + tileChrome }
