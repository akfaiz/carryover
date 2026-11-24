package kit

import "github.com/egoist/mygo/ui"

// ChoiceTileSize is the side of the square tile of one choice (a way to play,
// a kind of game to add, a storage provider).
const ChoiceTileSize = 124

// ChoiceGrid lays out n choice tiles, draw(i) drawing the i-th, in rows of as
// many as fit in width (a dialog's ContentWidth). The rows are laid out by
// hand: a wrapping row works out its height from the width it had when it was
// first laid out, which leaves a tall gap under the tiles in a dialog.
func ChoiceGrid(c *ui.Context, width float32, n int, draw func(i int)) ui.Element {
	per := max(1, int((width+S2)/(ChoiceTileSize+S2)))
	return ui.Column(c).Gap(S2).Children(func() {
		for start := 0; start < n; start += per {
			ui.Row(c).Gap(S2).Children(func() {
				for i := start; i < min(start+per, n); i++ {
					draw(i)
				}
			})
		}
	})
}
