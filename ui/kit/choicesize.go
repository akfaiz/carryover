package kit

// ChoiceTileSize is the side of the square tile of one way to play, and
// ChoicesPerRow how many fit in a row of the dialog. The rows are laid out by
// hand: a wrapping grid works out its height from the width it had when it was
// first laid out, which leaves a tall gap under the tiles in a dialog.
const (
	ChoiceTileSize = 124
	ChoicesPerRow  = 4
)
