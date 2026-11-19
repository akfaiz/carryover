package gamemode

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBigPicture(t *testing.T) {
	t.Setenv("SteamTenfoot", "")
	assert.False(t, BigPicture())
	t.Setenv("SteamTenfoot", "1")
	assert.True(t, BigPicture())
	t.Setenv("SteamTenfoot", "0")
	assert.False(t, BigPicture(), "only 1 counts")
}

// What Steam sets for a game must not reach the command that talks to Steam,
// and nothing else may be lost.
func TestCleanEnv(t *testing.T) {
	in := []string{
		"PATH=/usr/bin", "HOME=/home/deck", "LD_PRELOAD=gameoverlayrenderer.so",
		"SteamGameId=123", "STEAM_COMPAT_DATA_PATH=/x", "steamappid=1", "DISPLAY=:0", "NOEQUALS",
	}
	got := cleanEnv(in)
	assert.Equal(t, []string{"PATH=/usr/bin", "HOME=/home/deck", "DISPLAY=:0", "NOEQUALS"}, got)
	assert.Len(t, in, 8, "the input is left alone")
	assert.Empty(t, cleanEnv(nil))
}

// Where there is no command to ask Steam (Windows, macOS), it says so instead
// of running anything.
func TestOpenKeyboardWithoutACommand(t *testing.T) {
	if name, _ := keyboardCommand(); name != "" {
		t.Skip("this platform has a keyboard command")
	}
	via, err := OpenKeyboard()
	assert.Empty(t, via)
	assert.EqualError(t, err, "no way to ask Steam for its keyboard here")
}
