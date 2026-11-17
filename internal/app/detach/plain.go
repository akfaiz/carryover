//go:build !darwin

package detach

import (
	"os"
	"os/exec"
)

// plain starts the process directly, set up by configure, without waiting for
// it. macOS doesn't use it: there a shell starts the process (see start).
func plain(exe string, args []string, configure func(*exec.Cmd)) error {
	cmd := exec.Command(exe, args...)
	cmd.Env = withoutSteam(os.Environ())
	if configure != nil {
		configure(cmd)
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	return cmd.Process.Release()
}
