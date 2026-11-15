package gamemode

import (
	"testing"

	"github.com/stretchr/testify/assert"
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

// Steam starting Carryover (Big Picture on Windows, a Deck shortcut in
// Desktop Mode) wants Steam's keyboard as much as Game Mode does.
func TestKeyboardWanted(t *testing.T) {
	t.Setenv("XDG_CURRENT_DESKTOP", "KDE")
	t.Setenv("GAMESCOPE_WAYLAND_DISPLAY", "")
	t.Setenv("SteamGameId", "")
	t.Setenv("SteamClientLaunch", "")
	assert.False(t, KeyboardWanted(), "started by hand on a desktop")
	t.Setenv("SteamGameId", "11203204647261044736")
	assert.True(t, KeyboardWanted(), "started by Steam")
	t.Setenv("SteamGameId", "")
	t.Setenv("XDG_CURRENT_DESKTOP", "gamescope")
	assert.True(t, KeyboardWanted(), "Game Mode")
}
