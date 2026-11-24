// Package gamemode tells whether Carryover runs in Steam Deck Game Mode
// (Steam's gamescope session), where there is no mouse, no notification
// service and no keyboard but Steam's on-screen one, or was started by Steam
// at all, and opens Steam's on-screen keyboard.
package gamemode

import (
	"context"
	"errors"
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

// SteamLaunched reports whether Steam started this process, as a non-Steam
// game (Big Picture on Windows, a shortcut on the Deck in either mode).
func SteamLaunched() bool {
	return os.Getenv("SteamGameId") != "" || os.Getenv("SteamClientLaunch") == "1"
}

// BigPicture reports whether Steam started this process from Big Picture
// (SteamTenfoot=1). Valve doesn't document the variable, so it only picks a
// default: the log header lists Steam's variables to check it.
func BigPicture() bool { return os.Getenv("SteamTenfoot") == "1" }

// KeyboardWanted reports whether Steam's on-screen keyboard is the way to type
// with a controller here: in Game Mode, or when Steam started Carryover.
func KeyboardWanted() bool { return Active() || SteamLaunched() }

// QuietInputMethod stops Steam's own GTK input method from showing the
// on-screen keyboard whenever a window gets the focus. MyGo focuses GTK's
// input context with the window, not only with a text field, and Game Mode
// exports GTK_IM_MODULE=Steam, whose module opens Steam's keyboard on that
// focus: over Carryover and over its prompts, which have no text field.
// Carryover asks for the keyboard itself (OpenKeyboard) when a text field is
// opened. An input method the user chose (fcitx, ibus) is left alone. Call it
// before the GUI starts.
func QuietInputMethod() {
	if !CanRequestKeyboard || !KeyboardWanted() {
		return
	}
	if im := os.Getenv("GTK_IM_MODULE"); im == "" || strings.EqualFold(im, "steam") {
		_ = os.Setenv("GTK_IM_MODULE", "gtk-im-context-simple")
	}
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

// OpenKeyboard asks the running Steam to show its on-screen keyboard and
// waits for the request to be handed over (keyboardCommand says how, per OS).
// It returns which command it used and, on failure, an error with what that
// command printed. The command gets this process's environment without what
// Steam sets for a game: its variables and the overlay it preloads.
func OpenKeyboard() (via string, err error) {
	name, args := keyboardCommand()
	if name == "" {
		return "", errors.New("no way to ask Steam for its keyboard here")
	}
	ctx, cancel := context.WithTimeout(context.Background(), keyboardTimeout)
	defer cancel()
	out, err := run(ctx, cleanEnv(os.Environ()), name, args...)
	if err != nil {
		cmd := strings.Join(append([]string{name}, args...), " ")
		if s := strings.TrimSpace(string(out)); s != "" {
			return name, fmt.Errorf("%s: %w: %s", cmd, err, s)
		}
		return name, fmt.Errorf("%s: %w", cmd, err)
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
