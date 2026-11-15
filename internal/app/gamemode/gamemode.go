// Package gamemode tells whether Carryover runs in Steam Deck Game Mode
// (Steam's gamescope session), where there is no mouse, no notification
// service and no keyboard but Steam's on-screen one.
package gamemode

import "os"

// Active reports whether the process runs inside gamescope, Steam's Game Mode
// compositor.
func Active() bool {
	return os.Getenv("XDG_CURRENT_DESKTOP") == "gamescope" || os.Getenv("GAMESCOPE_WAYLAND_DISPLAY") != ""
}

// KeyboardURL opens Steam's on-screen keyboard when opened in Game Mode.
const KeyboardURL = "steam://open/keyboard"
