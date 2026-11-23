package core

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/akfaiz/carryover/internal/app/appdir"
	"github.com/akfaiz/carryover/internal/library"
)

// withOldInstall adds the settings of an earlier install of this device
// ("old-phone") to the game.
func withOldInstall(t *testing.T, c *Core, gameID string) {
	t.Helper()
	_, err := c.Library.UpdateDevice(gameID, "old-phone", func(d *library.DeviceConfig) {
		d.DeviceName = "Phone"
		d.Paths["saves"] = "/old/saves"
	})
	require.NoError(t, err)
}

func TestReuseDevice(t *testing.T) {
	c, id, saves, _ := newPlayable(t)
	withOldInstall(t, c, id)
	before := c.DeviceID()
	// This install was set up again after the old one.
	_, err := c.Library.UpdateDevice(id, before, func(d *library.DeviceConfig) { d.Args = "again" })
	require.NoError(t, err)

	require.NoError(t, c.ReuseDevice("old-phone"))
	assert.Equal(t, before, c.DeviceID(), "this device keeps its own id")
	g := mustFind(t, c, id)
	assert.Equal(t, saves, g.Devices[before].Paths["saves"], "this install's newer settings are kept")
	assert.NotContains(t, g.Devices, "old-phone")

	// It survives the library sync.
	require.NoError(t, c.SyncLibrary(context.Background()))
	assert.False(t, c.Library.HasDevice("old-phone"))
	job, err := c.Job(mustFind(t, c, id))
	require.NoError(t, err)
	assert.NotEmpty(t, job.Slots, "the game is still set up here")
}

func TestReuseDeviceRefused(t *testing.T) {
	c, id, _, _ := newPlayable(t)
	assert.ErrorIs(t, c.ReuseDevice(c.DeviceID()), ErrThisDevice)
	assert.ErrorIs(t, c.ReuseDevice("nobody"), library.ErrUnknownDevice)

	withOldInstall(t, c, id)
	marker := filepath.Join(c.Dirs.StateDir(), id+sessionExt)
	require.NoError(t, os.MkdirAll(c.Dirs.StateDir(), 0o755))
	require.NoError(t, os.WriteFile(marker, []byte("{}"), 0o644))
	own := c.DeviceID()
	assert.ErrorIs(t, c.ReuseDevice("old-phone"), ErrSessionsOpen)
	assert.Equal(t, own, c.DeviceID())
}

func TestForgetDevice(t *testing.T) {
	c, id, _, cloudDir := newPlayable(t)
	withOldInstall(t, c, id)
	require.NoError(t, c.SyncLibrary(context.Background()))

	assert.ErrorIs(t, c.ForgetDevice(c.DeviceID()), ErrThisDevice)
	require.NoError(t, c.ForgetDevice("old-phone"))
	require.NoError(t, c.SyncLibrary(context.Background()))
	assert.NotContains(t, mustFind(t, c, id).Devices, "old-phone")

	// The cloud copy has the marker, so other devices drop it too.
	other, err := library.Open(filepath.Join(t.TempDir(), "library.json"))
	require.NoError(t, err)
	var cat library.Catalog
	found, err := appdir.ReadJSON(filepath.Join(cloudDir, "library.json"), &cat)
	require.NoError(t, err)
	require.True(t, found, "cloud catalog")
	assert.False(t, cat.Devices["old-phone"].ForgottenAt.IsZero())
	assert.NotEmpty(t, cat.Devices[c.DeviceID()].Platform, "this device registered itself")
	_, err = other.MergeCatalog(cat)
	require.NoError(t, err)
	assert.False(t, other.HasDevice("old-phone"))
}
