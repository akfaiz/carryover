package gamemode

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

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
