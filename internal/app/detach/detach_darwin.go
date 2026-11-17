package detach

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"
)

// start runs the process so that launchd adopts it at once. Steam on macOS
// stops a game by signalling every process it started, one by one, so a new
// session (enough against a signal to the process group) isn't: the watcher
// was still the wrapper's child and got the signal too. macOS has no
// systemd-run, so a shell starts it in the background and exits; the process
// is then no longer a descendant of anything Steam started.
func start(exe string, args []string) error {
	shArgs := append([]string{"-c", `"$@" </dev/null >/dev/null 2>&1 &`, "sh", exe}, args...)
	cmd := exec.Command("/bin/sh", shArgs...)
	cmd.Env = withoutSteam(os.Environ())
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("start %s: %w: %s", exe, err, out)
	}
	return nil
}

// Where says how this process was started, for the log: adopted by launchd
// (parent 1, outside what Steam stops) or still under another process.
func Where() string {
	if p := os.Getppid(); p != 1 {
		return fmt.Sprintf("parent %d, not launchd", p)
	}
	return "adopted by launchd"
}
