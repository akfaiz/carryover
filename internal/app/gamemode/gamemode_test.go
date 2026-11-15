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

var runDefault = run

func TestActive(t *testing.T) {
	t.Setenv("XDG_CURRENT_DESKTOP", "KDE")
	t.Setenv("GAMESCOPE_WAYLAND_DISPLAY", "")
	assert.False(t, Active(), "desktop mode")

	t.Setenv("XDG_CURRENT_DESKTOP", "gamescope")
	assert.True(t, Active())

	t.Setenv("XDG_CURRENT_DESKTOP", "")
	t.Setenv("GAMESCOPE_WAYLAND_DISPLAY", "gamescope-0")
	assert.True(t, Active())
}

func TestOpenKeyboard(t *testing.T) {
	t.Setenv("SteamGameId", "123")
	t.Setenv("LD_PRELOAD", "/home/deck/.local/share/Steam/ubuntu12_64/gameoverlayrenderer.so")
	t.Setenv("HOME", "/home/deck")
	var gotName string
	var gotArgs, gotEnv []string
	run = func(_ context.Context, env []string, name string, args ...string) ([]byte, error) {
		gotName, gotArgs, gotEnv = name, args, env
		return nil, nil
	}
	lookPath = func(string) (string, error) { return "/usr/bin/steam", nil }
	t.Cleanup(func() { run, lookPath = runDefault, exec.LookPath })

	via, err := OpenKeyboard()
	require.NoError(t, err)
	assert.Equal(t, "steam", via, "Steam's own launcher hands the URL to the running client")
	assert.Equal(t, "steam", gotName)
	assert.Equal(t, []string{KeyboardURL}, gotArgs)
	assert.Contains(t, gotEnv, "HOME=/home/deck")
	for _, kv := range gotEnv {
		assert.False(t, strings.HasPrefix(kv, "SteamGameId=") || strings.HasPrefix(kv, "LD_PRELOAD="), kv)
	}

	lookPath = func(string) (string, error) { return "", errors.New("not found") }
	run = func(context.Context, []string, string, ...string) ([]byte, error) {
		return []byte("xdg-open: no method available for opening 'steam://open/keyboard'\n"), errors.New("exit status 3")
	}
	via, err = OpenKeyboard()
	assert.Equal(t, "xdg-open", via, "without steam on PATH")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no method available", "the log says why")
}
