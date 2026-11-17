//go:build darwin

package notify

import (
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

// script shows its first argument as the title and its second as the text.
// They are passed as arguments, never put into the script, so no quoting can
// break it.
var script = []string{
	"-e", "on run argv",
	"-e", "display notification (item 2 of argv) with title (item 1 of argv)",
	"-e", "end run",
}

// osascript runs the command. Tests replace it.
var osascript = defaultOsascript

func defaultOsascript(args ...string) ([]byte, error) {
	return exec.Command("osascript", args...).CombinedOutput()
}

// send shows a notification through AppleScript's display notification, which
// works without a signed app bundle (macOS shows it as from Script Editor).
// app is unused: the sender can't be chosen this way.
func send(_ App, title, body string) error {
	args := append(append([]string{}, script...), title, body)
	if out, err := osascript(args...); err != nil {
		if _, ok := errors.AsType[*exec.ExitError](err); ok {
			return fmt.Errorf("osascript: %s", strings.TrimSpace(string(out)))
		}
		return err
	}
	return nil
}
