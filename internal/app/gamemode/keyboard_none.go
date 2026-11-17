//go:build windows || darwin

package gamemode

// CanRequestKeyboard is false on Windows and macOS. On Windows steam://open/keyboard opens
// Big Picture's own keyboard, not the overlay's. Behind a full-screen
// Carryover it can't be seen, and while it is open Steam sends the controller
// to it, so Carryover seems to freeze. The overlay's keyboard, which types into
// Carryover, opens with Steam's own chord (the Steam or Xbox button + X); the
// UI says so instead. macOS has neither the `steam` command nor xdg-open, and
// Steam there has no keyboard for the desktop.
const CanRequestKeyboard = false

// keyboardCommand: none here (see CanRequestKeyboard).
func keyboardCommand() (name string, args []string) { return "", nil }
