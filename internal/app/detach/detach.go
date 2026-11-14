// Package detach starts a process that outlives the process tree it was
// started from. Steam ends a game by killing its whole process tree (on Linux,
// everything under its "reaper" subreaper), which includes Carryover's
// "run -- %command%" wrapper; a detached helper survives that and can finish
// the session.
package detach

import (
	"os"
	"os/exec"
	"strings"
)

// Start starts exe with args, detached from this process's tree, and does
// not wait for it. The process may not inherit this process's environment
// (systemd-run, WMI): pass what it needs as arguments. Where it does inherit
// it, Steam's variables are left out (see withoutSteam).
func Start(exe string, args []string) error {
	return start(exe, args)
}

// plain starts the process directly, set up by configure, without waiting for it.
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

// withoutSteam drops the variables Steam sets for a game (SteamGameId,
// SteamAppId, SteamClientLaunch, STEAM_COMPAT_*, ...): the detached process is
// not part of the game, and Steam may use them to find the game's processes
// when it stops it.
func withoutSteam(env []string) []string {
	out := env[:0:0]
	for _, kv := range env {
		name, _, _ := strings.Cut(kv, "=")
		if strings.HasPrefix(strings.ToLower(name), "steam") {
			continue
		}
		out = append(out, kv)
	}
	return out
}
