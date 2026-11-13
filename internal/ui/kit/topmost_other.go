//go:build !windows

package kit

import "fyne.io/fyne/v2"

// SetTopmost is a no-op outside Windows (gamescope manages focus on the Deck).
func SetTopmost(fyne.Window) {}
