//go:build !windows

package detach

import (
	"log"
	"os"
	"os/exec"
	"strings"
	"syscall"
)

// start runs the process as a transient systemd user service when systemd-run
// is available: it is then a child of the user's systemd instance, outside
// Steam's reaper, which kills every descendant it has. Otherwise (no systemd,
// Flatpak Steam's sandbox) it falls back to a new session, which survives a
// signal to the process group but not necessarily Steam's reaper.
func start(exe string, args []string) error {
	if run, err := exec.LookPath("systemd-run"); err == nil {
		sdArgs := []string{"--user", "--collect", "--quiet", "--no-block", "--", exe}
		sdArgs = append(sdArgs, args...)
		cmd := exec.Command(run, sdArgs...)
		out, err := cmd.CombinedOutput()
		if err == nil {
			return nil
		}
		log.Printf("systemd-run: %v: %s; starting it in a new session instead", err, strings.TrimSpace(string(out)))
	}
	return plain(exe, args, func(cmd *exec.Cmd) {
		cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	})
}

// Where says how this process was started, for the log: as a systemd user
// service (outside Steam's reaper) or not.
func Where() string {
	if os.Getenv("INVOCATION_ID") != "" {
		return "systemd service"
	}
	return "not a systemd service"
}
