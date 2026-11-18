package shell

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSectionOf(t *testing.T) {
	for route, want := range map[string]string{
		"/": "/", "/game/x": "/", "/edit/x": "/", "/new/emulator": "/",
		"/emulators": "/emulators", "/cloud": "/cloud", "/settings": "/settings", "/trash": "/trash",
	} {
		assert.Equal(t, want, sectionOf(route), route)
	}
}

// The gamepad is read where Steam hands it over as one; started by hand on a
// desktop, Steam's Desktop Layout already makes it a mouse and keys.
func TestPadWanted(t *testing.T) {
	t.Setenv("SteamGameId", "")
	t.Setenv("SteamClientLaunch", "")
	t.Setenv("XDG_CURRENT_DESKTOP", "")
	t.Setenv("GAMESCOPE_WAYLAND_DISPLAY", "")
	assert.False(t, padWanted(), "started by hand")
	t.Setenv("SteamGameId", "123")
	assert.True(t, padWanted(), "started by Steam")
	t.Setenv("SteamGameId", "")
	t.Setenv("XDG_CURRENT_DESKTOP", "gamescope")
	assert.True(t, padWanted(), "Game Mode")
}
