//go:build !windows

package gamemode

import (
	"context"
	"errors"
	"os/exec"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// On Linux the URL goes to Steam's own launcher, xdg-open only without one.
func TestKeyboardCommand(t *testing.T) {
	t.Cleanup(func() { lookPath = exec.LookPath })
	lookPath = func(string) (string, error) { return "/usr/bin/steam", nil }
	name, args := keyboardCommand()
	assert.Equal(t, "steam", name)
	assert.Equal(t, []string{KeyboardURL}, args)

	lookPath = func(string) (string, error) { return "", errors.New("not found") }
	name, args = keyboardCommand()
	assert.Equal(t, "xdg-open", name)
	assert.Equal(t, []string{KeyboardURL}, args)
}

// The keyboard is asked for without Steam's game variables and overlay, and a
// failure says what the command printed.
func TestOpenKeyboard(t *testing.T) {
	t.Setenv("SteamGameId", "123")
	t.Setenv("LD_PRELOAD", "/home/deck/.local/share/Steam/ubuntu12_64/gameoverlayrenderer.so")
	t.Setenv("HOME", "/home/deck")
	var gotEnv []string
	run = func(_ context.Context, env []string, _ string, _ ...string) ([]byte, error) {
		gotEnv = env
		return nil, nil
	}
	t.Cleanup(func() { run = runDefault })

	_, err := OpenKeyboard()
	require.NoError(t, err)
	assert.Contains(t, gotEnv, "HOME=/home/deck")
	for _, kv := range gotEnv {
		assert.False(t, strings.HasPrefix(kv, "SteamGameId=") || strings.HasPrefix(kv, "LD_PRELOAD="), kv)
	}

	run = func(context.Context, []string, string, ...string) ([]byte, error) {
		return []byte("no handler for steam://\n"), errors.New("exit status 3")
	}
	_, err = OpenKeyboard()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no handler", "the log says why")
	assert.Contains(t, err.Error(), KeyboardURL)
}
