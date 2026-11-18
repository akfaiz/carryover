package ui

import (
	"os"
	"path/filepath"
	"testing"

	"fyne.io/fyne/v2/widget"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/akfaiz/carryover/internal/core"
	"github.com/akfaiz/carryover/internal/stores/steam"
)

// Every kind of library art Carryover's own shortcut gets is embedded.
func TestSelfSteamArt(t *testing.T) {
	art := selfSteamArt()
	for _, kind := range core.SelfArtKinds {
		assert.NotEmpty(t, art[kind], kind)
	}
}

// Settings adds Carryover itself to Steam, then offers to update or remove it,
// and the card stays usable with a gamepad.
func TestSettingsAddsCarryoverToSteam(t *testing.T) {
	root := t.TempDir()
	t.Setenv("CARRYOVER_STEAM_ROOT", root)
	const user = "12345"
	require.NoError(t, os.MkdirAll(filepath.Join(root, "userdata", user), 0o755))
	h := newHarness(t)
	h.open("/settings")
	h.settle()

	tapButton(t, h, "Add Carryover to Steam")
	h.settle()
	list, err := steam.ListShortcuts(root, user)
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Equal(t, "Carryover", list[0].AppName)
	_, err = os.Stat(filepath.Join(steam.GridDir(root, user), filepath.Base(list[0].Icon)))
	require.NoError(t, err, "icon")
	require.NotEmpty(t, h.toasts())
	assert.Contains(t, h.toasts()[0], "Added to Steam") // with "restart Steam" while it runs

	// The button now updates the shortcut, and Remove is reachable with the D-pad.
	h.u.Navigator().FocusDefault()
	got := reachable(h, h.u.Window().Canvas().Focused())
	for _, text := range []string{"Update Steam shortcut", "Remove"} {
		b := buttonWith(t, pageWidgets[*widget.Button](h), text)
		assert.True(t, got[b], "%s not reachable with the D-pad", text)
	}

	tapButton(t, h, "Remove")
	h.settle()
	list, _ = steam.ListShortcuts(root, user)
	assert.Empty(t, list)
	buttonWith(t, pageWidgets[*widget.Button](h), "Add Carryover to Steam")
}
