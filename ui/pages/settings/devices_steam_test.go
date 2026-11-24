package settings_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/akfaiz/carryover/internal/library"
	"github.com/akfaiz/carryover/services"
	"github.com/akfaiz/carryover/ui/env"
	"github.com/akfaiz/carryover/ui/internal/uitest"
	"github.com/akfaiz/carryover/ui/pages/settings"
	"github.com/akfaiz/carryover/ui/shell"
)

func newHarnessWith(t *testing.T, opts settings.Options) *uitest.Harness {
	t.Helper()
	settings.AutosaveDelay = 20 * time.Millisecond
	h := uitest.New(t, shell.Config{
		Page:         func(e env.Env, _ string) env.Page { return settings.New(e, opts) },
		AddGame:      func(env.Env) {},
		SyncProblems: func(env.Env, int, []services.SyncResult) {},
	})
	h.Tester.SetSize(1000, 2400)
	h.Open("/settings")
	return h
}

// withDevice makes the library hold another device's settings for a game.
func withDevice(t *testing.T, h *uitest.Harness, gameID, id, name string) {
	t.Helper()
	_, err := h.Core.Library.UpdateDevice(gameID, id, func(d *library.DeviceConfig) {
		d.DeviceName = name
		d.Paths["saves"] = "/elsewhere"
	})
	require.NoError(t, err)
}

func settle(h *uitest.Harness) {
	for range 3 {
		h.Settle()
		time.Sleep(10 * time.Millisecond)
	}
}

func TestOtherDevicesCanBeForgotten(t *testing.T) {
	h := newHarnessWith(t, settings.Options{})
	g := h.AddGame("Game A", t.TempDir())
	withDevice(t, h, g.ID, "deck", "Steam Deck")
	withDevice(t, h, g.ID, "nameless", "")
	h.Open("/settings")
	settle(h)
	assert.Contains(t, h.Texts(), "Steam Deck")
	assert.Contains(t, h.Texts(), "Unnamed device")
	assert.NotContains(t, h.Texts(), "No other devices.")

	require.NoError(t, h.Tester.Click("Forget…"))
	settle(h)
	assert.Contains(t, h.Texts(), "Forget device")
	h.Click("Forget")
	settle(h)
	assert.Contains(t, h.Texts(), "Device forgotten")
}

func TestAnEarlierInstallCanBeTakenOver(t *testing.T) {
	h := newHarnessWith(t, settings.Options{})
	g := h.AddGame("Game A", t.TempDir())
	withDevice(t, h, g.ID, "old", "Phone")
	h.Open("/settings")
	settle(h)

	require.NoError(t, h.Tester.Click("This is me…"))
	settle(h)
	assert.Contains(t, h.Texts(), "Use the settings of Phone here")
	h.Click("This is me")
	settle(h)
	assert.Contains(t, h.Texts(), "This device now has the settings of Phone.")
}

// Carryover's own Steam card: nothing without Steam, and with it one button to
// add and one to remove the shortcut.
func TestSteamCardWithoutSteam(t *testing.T) {
	t.Setenv("CARRYOVER_STEAM_ROOT", "")
	t.Setenv("HOME", t.TempDir())
	h := newHarnessWith(t, settings.Options{SteamArt: func() map[string][]byte { return nil }})
	settle(h)
	assert.Contains(t, h.Texts(), "Steam")
}

func TestSteamCardAddsAndRemovesTheShortcut(t *testing.T) {
	root := t.TempDir()
	for _, id := range []string{"1111", "2222"} {
		require.NoError(t, os.MkdirAll(filepath.Join(root, "userdata", id, "config"), 0o755))
	}
	t.Setenv("CARRYOVER_STEAM_ROOT", root)
	h := newHarnessWith(t, settings.Options{SteamArt: func() map[string][]byte {
		return map[string][]byte{"icon": {1, 2, 3}}
	}})
	settle(h)
	for _, want := range []string{"Add Carryover to Steam", "Steam account"} {
		assert.Contains(t, h.Texts(), want)
	}

	h.Click("Add Carryover to Steam")
	settle(h)
	assert.Contains(t, h.Texts(), "In Steam on this device")
	assert.Contains(t, h.Texts(), "Update Steam shortcut")

	h.Click("Remove")
	settle(h)
	assert.NotContains(t, h.Texts(), "In Steam on this device")
	assert.Contains(t, h.Texts(), "Add Carryover to Steam")
}
