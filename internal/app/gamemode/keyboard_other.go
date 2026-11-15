//go:build !windows

package gamemode

import "os/exec"

// CanRequestKeyboard: on Linux (the Deck's Game Mode, a Steam shortcut in
// Desktop Mode) Steam shows its keyboard over Carryover when asked.
const CanRequestKeyboard = true

// lookPath finds a command. Tests replace it.
var lookPath = exec.LookPath

// keyboardCommand hands KeyboardURL to Steam's own launcher script
// (`steam <url>`, which passes it to the running client) rather than to
// xdg-open, which needs a handler for steam:// links; xdg-open only when
// there is no steam command.
func keyboardCommand() (name string, args []string) {
	if _, err := lookPath("steam"); err == nil {
		return "steam", []string{KeyboardURL}
	}
	return "xdg-open", []string{KeyboardURL}
}
