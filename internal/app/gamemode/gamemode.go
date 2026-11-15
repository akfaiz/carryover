// Package gamemode tells whether Carryover runs in Steam Deck Game Mode
// (Steam's gamescope session), where there is no mouse, no notification
// service and no keyboard but Steam's on-screen one, and opens that keyboard.
package gamemode

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// Active reports whether the process runs inside gamescope, Steam's Game Mode
// compositor.
func Active() bool {
	return os.Getenv("XDG_CURRENT_DESKTOP") == "gamescope" || os.Getenv("GAMESCOPE_WAYLAND_DISPLAY") != ""
}

// KeyboardURL asks Steam to show its on-screen keyboard.
const KeyboardURL = "steam://open/keyboard"

// keyboardTimeout bounds the wait for the command that hands the URL to Steam;
// it returns as soon as the running Steam has it.
const keyboardTimeout = 10 * time.Second

// run runs a command and returns what it printed. Tests replace it.
var run = func(ctx context.Context, env []string, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Env = env
	return cmd.CombinedOutput()
}

// lookPath finds a command. Tests replace it.
var lookPath = exec.LookPath

// OpenKeyboard asks the running Steam to show its on-screen keyboard and
// waits for the request to be handed over. It returns which command it used
// and, on failure, an error with what that command printed.
//
// The URL goes to Steam's own launcher script (`steam <url>`, which passes it
// to the running client) rather than through xdg-open, which needs a handler
// for steam:// links. The command gets this process's environment without
// what Steam sets for a game: its variables and the overlay it preloads.
func OpenKeyboard() (via string, err error) {
	name := "steam"
	if _, err := lookPath(name); err != nil {
		name = "xdg-open"
	}
	ctx, cancel := context.WithTimeout(context.Background(), keyboardTimeout)
	defer cancel()
	out, err := run(ctx, cleanEnv(os.Environ()), name, KeyboardURL)
	if err != nil {
		if s := strings.TrimSpace(string(out)); s != "" {
			return name, fmt.Errorf("%s %s: %w: %s", name, KeyboardURL, err, s)
		}
		return name, fmt.Errorf("%s %s: %w", name, KeyboardURL, err)
	}
	return name, nil
}

// cleanEnv drops the variables Steam sets for a game (SteamGameId,
// SteamAppId, STEAM_COMPAT_*, ...) and LD_PRELOAD, which loads Steam's
// overlay into everything a game starts.
func cleanEnv(env []string) []string {
	out := env[:0:0]
	for _, kv := range env {
		name, _, _ := strings.Cut(kv, "=")
		if strings.HasPrefix(strings.ToLower(name), "steam") || name == "LD_PRELOAD" {
			continue
		}
		out = append(out, kv)
	}
	return out
}
