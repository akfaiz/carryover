package services

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/akfaiz/carryover/internal/library"
)

// withDevice adds another device's settings to the game.
func withDevice(t *testing.T, e syncEnv, id, name string) {
	t.Helper()
	_, err := e.c.Library.UpdateDevice(e.id, id, func(d *library.DeviceConfig) {
		d.DeviceName = name
		d.Paths["saves"] = "/elsewhere"
	})
	require.NoError(t, err)
}

func TestDevicesMarksEarlierInstalls(t *testing.T) {
	e := newSyncEnv(t)
	st := e.c.Settings.Get()
	st.DeviceName = "Phone"
	_, err := e.svc.SaveSettings(st)
	require.NoError(t, err)
	withDevice(t, e, "deck", "steamdeck")
	withDevice(t, e, "old", "phone")

	devs := e.svc.Devices()
	require.Len(t, devs, 3)
	assert.True(t, devs[0].This)
	assert.Equal(t, e.c.DeviceID(), devs[0].ID)
	assert.Equal(t, 1, devs[0].Games)
	assert.Equal(t, "old", devs[1].ID, "a device of the same name comes first")
	assert.True(t, devs[1].SameName)
	assert.False(t, devs[2].SameName)
}

func TestForgetAndReuseDevice(t *testing.T) {
	e := newSyncEnv(t)
	withDevice(t, e, "deck", "steamdeck")
	withDevice(t, e, "old", "Phone")
	own := e.c.DeviceID()

	require.ErrorIs(t, e.svc.ForgetDevice(own), ErrThisDevice)
	require.NoError(t, e.svc.ForgetDevice("deck"))
	require.ErrorIs(t, e.svc.ForgetDevice("deck"), ErrUnknownDevice)

	require.NoError(t, e.svc.ReuseDevice("old"))
	assert.Equal(t, "old", e.c.DeviceID())
	devs := e.svc.Devices()
	require.Len(t, devs, 1, "the earlier id is gone")
	assert.Equal(t, "old", devs[0].ID)
	assert.Equal(t, 1, devs[0].Games)
	g, err := e.c.Library.Get(e.id)
	require.NoError(t, err)
	assert.Equal(t, "/elsewhere", g.Devices["old"].Paths["saves"], "the old install's settings were changed last")
}
