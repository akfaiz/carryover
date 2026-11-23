package library

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A device that changes only its own setup, before it has merged a rename and
// a removed save slot made on another device, must not undo them.
func TestDeviceOnlySaveKeepsOtherDevicesSharedChanges(t *testing.T) {
	dir := t.TempDir()
	pc, err := Open(filepath.Join(dir, "pc.json"))
	require.NoError(t, err)
	phone, err := Open(filepath.Join(dir, "phone.json"))
	require.NoError(t, err)

	g, err := pc.Save(Game{
		Name:    "Game",
		Slots:   []SaveSlot{{Key: "saves"}, {Key: "states"}},
		Devices: map[string]DeviceConfig{"pc": {Paths: map[string]string{"saves": "C:/saves"}}},
	}, "pc")
	require.NoError(t, err)
	_, err = phone.MergeCatalog(pc.Catalog())
	require.NoError(t, err)

	// The PC renames the game and removes a slot.
	time.Sleep(2 * time.Millisecond)
	renamed, err := pc.Get(g.ID)
	require.NoError(t, err)
	renamed.Name, renamed.Slots = "Game (GOTY)", []SaveSlot{{Key: "saves"}}
	_, err = pc.Save(renamed, "pc")
	require.NoError(t, err)

	// Later, the phone, which hasn't seen that, sets its own path through a
	// whole-game save (as the Android app does).
	time.Sleep(2 * time.Millisecond)
	mine, err := phone.Get(g.ID)
	require.NoError(t, err)
	before := mine.UpdatedAt
	mine.Devices["phone"] = DeviceConfig{Paths: map[string]string{"saves": "/sdcard/saves"}}
	saved, err := phone.Save(mine, "phone")
	require.NoError(t, err)
	assert.True(t, saved.UpdatedAt.Equal(before), "only the phone's entry changed")

	for _, s := range []*Store{pc, phone} {
		_, err = s.MergeCatalog(pc.Catalog())
		require.NoError(t, err)
		_, err = s.MergeCatalog(phone.Catalog())
		require.NoError(t, err)
		got, err := s.Get(g.ID)
		require.NoError(t, err)
		assert.Equal(t, "Game (GOTY)", got.Name)
		assert.Equal(t, []SaveSlot{{Key: "saves"}}, got.Slots)
		assert.Equal(t, "/sdcard/saves", got.Devices["phone"].Paths["saves"])
		assert.Equal(t, "C:/saves", got.Devices["pc"].Paths["saves"])
	}
}

func TestSharedChangeStampsTheGame(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "lib.json"))
	require.NoError(t, err)
	g, err := s.Save(Game{Name: "Game", Slots: []SaveSlot{{Key: "saves"}}}, "pc")
	require.NoError(t, err)
	time.Sleep(2 * time.Millisecond)

	// Empty and missing lists are the same settings.
	g.Slots[0].Include = []string{}
	same, err := s.Save(g, "pc")
	require.NoError(t, err)
	assert.True(t, same.UpdatedAt.Equal(g.UpdatedAt))

	g.Name = "Renamed"
	renamed, err := s.Save(g, "pc")
	require.NoError(t, err)
	assert.True(t, renamed.UpdatedAt.After(g.UpdatedAt))

	e, err := s.SaveEmulator(Emulator{Name: "Eden", Devices: map[string]EmulatorDevice{"pc": {Exe: "eden"}}}, "pc")
	require.NoError(t, err)
	time.Sleep(2 * time.Millisecond)
	e.Devices["pc"] = EmulatorDevice{Exe: "eden2"}
	e2, err := s.SaveEmulator(e, "pc")
	require.NoError(t, err)
	assert.True(t, e2.UpdatedAt.Equal(e.UpdatedAt), "only this device's setup changed")
	assert.True(t, e2.Devices["pc"].UpdatedAt.After(e.Devices["pc"].UpdatedAt))
	e2.Preset = "switch"
	e3, err := s.SaveEmulator(e2, "pc")
	require.NoError(t, err)
	assert.True(t, e3.UpdatedAt.After(e2.UpdatedAt))
}

// The catalog leaves out empty fields and the save slots of an emulator that
// shares another's cloud save; both come back when it is read.
func TestCatalogIsCompact(t *testing.T) {
	path := filepath.Join(t.TempDir(), "lib.json")
	s, err := Open(path)
	require.NoError(t, err)
	slots := []SaveSlot{{Key: "account", Label: "Account save"}}
	g, err := s.Save(Game{
		Name: "Unicorn Overlord",
		Emulators: []GameEmulator{
			{EmulatorID: "eden", Slots: slots},
			{EmulatorID: "ryujinx", Slots: slots, ShareWith: "eden"},
		},
		Devices: map[string]DeviceConfig{"phone": {
			DeviceName: "Phone",
			Emulators:  map[string]EmulatorConfig{"eden": {Rom: "uo.nsp", Paths: map[string]string{"account": "/saves"}}},
		}},
	}, "phone")
	require.NoError(t, err)
	ryu, ok := g.Emulator("ryujinx")
	require.True(t, ok)
	assert.Equal(t, slots, ryu.Slots, "in memory a sharing emulator has the other's slots")

	raw, err := os.ReadFile(path)
	require.NoError(t, err)
	var disk struct {
		Games []map[string]any `json:"games"`
	}
	require.NoError(t, json.Unmarshal(raw, &disk))
	require.Len(t, disk.Games, 1)
	stored := disk.Games[0]
	assert.NotContains(t, stored, "cover")
	assert.NotContains(t, stored, "slots")
	ems := stored["emulators"].([]any)
	assert.Contains(t, ems[0], "slots")
	assert.NotContains(t, ems[1], "slots", "the sharing emulator's slots are eden's")
	dev := stored["devices"].(map[string]any)["phone"].(map[string]any)
	for _, empty := range []string{"exe", "args", "workDir", "env", "waitProcess", "steamAppId", "paths"} {
		assert.NotContains(t, dev, empty)
	}
	assert.Equal(t, "Phone", dev["deviceName"], "kept for older clients")

	again, err := Open(path)
	require.NoError(t, err)
	g2, err := again.Get(g.ID)
	require.NoError(t, err)
	ryu, _ = g2.Emulator("ryujinx")
	assert.Equal(t, slots, ryu.Slots)
	assert.NotNil(t, g2.Devices["phone"].Paths, "maps come back empty, not nil")
	assert.Equal(t, LaunchDirect, g2.Devices["phone"].LaunchMode)

	// A catalog written by an older version, with the copy, reads the same.
	old := again.Catalog()
	old.Games[0].Emulators[1].Slots = []SaveSlot{{Key: "stale"}}
	other, err := Open(filepath.Join(t.TempDir(), "other.json"))
	require.NoError(t, err)
	_, err = other.MergeCatalog(old)
	require.NoError(t, err)
	g3, err := other.Get(g.ID)
	require.NoError(t, err)
	ryu, _ = g3.Emulator("ryujinx")
	assert.Equal(t, slots, ryu.Slots, "the stale copy is replaced by eden's slots")
}
