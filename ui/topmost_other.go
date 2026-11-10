//go:build !windows

package ui

import "fyne.io/fyne/v2"

// setTopmost is a no-op outside Windows (gamescope manages focus on the Deck).
func setTopmost(fyne.Window) {}
