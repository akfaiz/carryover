//go:build !windows && !(darwin && cgo)

package kit

import "fyne.io/fyne/v2"

// SetTopmost is a no-op on Linux (gamescope manages focus on the Deck).
func SetTopmost(fyne.Window) {}
