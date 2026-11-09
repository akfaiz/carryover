//go:build !windows

package launcher

import (
	"os/exec"
	"syscall"
)

// configureCmd puts the game in its own process group so signals sent to
// PlayRelay's terminal don't kill it mid-save.
func configureCmd(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}
