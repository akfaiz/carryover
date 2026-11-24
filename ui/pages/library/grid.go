package library

// The games' grid fills the width of the page: it has as many columns as fit
// tiles of at least TileMin, and the tiles share the room that is left, so a
// wider window makes them wider (and, once another column fits, adds one)
// instead of leaving an empty strip at the right.
const (
	// TileMin is the narrowest a tile gets: a 180 DIP cover and the padding
	// around it.
	TileMin float32 = 196
	// TileMax is the widest a tile gets. Two columns never reach it, so only a
	// lone column (a window narrower than the app allows) is ever capped.
	TileMax float32 = 304
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
