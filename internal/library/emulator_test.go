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

func TestEmulatorMergeAcrossDevices(t *testing.T) {
	dir := t.TempDir()
	pc, _ := Open(filepath.Join(dir, "pc.json"))
	deck, _ := Open(filepath.Join(dir, "deck.json"))

	e, err := pc.SaveEmulator(Emulator{
		Name:    "Eden",
		Devices: map[string]EmulatorDevice{"pc": {Exe: `C:\eden\eden.exe`, Args: "-g {rom}"}},
	}, "pc")
	require.NoError(t, err)

	changed, err := deck.MergeCatalog(pc.Catalog())
	require.NoError(t, err)
	require.True(t, changed)
	time.Sleep(2 * time.Millisecond)
	_, err = deck.SaveEmulator(Emulator{
		ID: e.ID, Name: "Eden",
		Devices: map[string]EmulatorDevice{"deck": {Exe: "/opt/eden"}},
	}, "deck")
	require.NoError(t, err)

	_, err = pc.MergeCatalog(deck.Catalog())
	require.NoError(t, err)
	got, err := pc.Emulator(e.ID)
	require.NoError(t, err)
	assert.Equal(t, `C:\eden\eden.exe`, got.Devices["pc"].Exe)
	assert.Equal(t, "/opt/eden", got.Devices["deck"].Exe)

	found, err := pc.FindEmulator("eden")
	require.NoError(t, err)
	assert.Equal(t, e.ID, found.ID)

	require.NoError(t, pc.DeleteEmulator(e.ID))
	_, err = deck.MergeCatalog(pc.Catalog())
	require.NoError(t, err)
	assert.Empty(t, deck.Emulators())
}

func TestGameEmulatorProfiles(t *testing.T) {
	s, _ := Open(filepath.Join(t.TempDir(), "l.json"))
	g, err := s.Save(Game{
		Name: "Zelda",
		Emulators: []GameEmulator{
			{EmulatorID: "eden", Slots: []SaveSlot{{Key: "saves", Include: []string{"*.sav"}}}},
			{EmulatorID: "ryu", Slots: []SaveSlot{{Key: "saves"}}},
		},
		Devices: map[string]DeviceConfig{"pc": {
			Emulators: map[string]EmulatorConfig{"eden": {Rom: "z.nsp", Paths: map[string]string{"saves": "/e"}}},
		}},
	}, "pc")
	require.NoError(t, err)

	got, err := s.Get(g.ID)
	require.NoError(t, err)
	// Mutating a returned copy must not touch the store.
	got.Emulators[0].Slots[0].Include[0] = "changed"
	got.Devices["pc"].Emulators["eden"].Paths["saves"] = "changed"
	again, _ := s.Get(g.ID)
	assert.Equal(t, "*.sav", again.Emulators[0].Slots[0].Include[0])
	assert.Equal(t, "/e", again.Devices["pc"].Emulators["eden"].Paths["saves"])

	_, ok := again.Emulator("ryu")
	assert.True(t, ok)

	_, err = s.Save(Game{Name: "x", Emulators: []GameEmulator{{EmulatorID: "a"}, {EmulatorID: "a"}}}, "pc")
	require.Error(t, err, "duplicate emulator")
	_, err = s.Save(Game{Name: "x", Emulators: []GameEmulator{{EmulatorID: "a.b"}}}, "pc")
	require.Error(t, err, "emulator id must be file-name safe")
}

func TestNewerCatalogIsNeverRewritten(t *testing.T) {
	path := filepath.Join(t.TempDir(), "l.json")
	data, _ := json.Marshal(Catalog{Version: CatalogVersion + 1})
	require.NoError(t, os.WriteFile(path, data, 0o600))
	_, err := Open(path)
	require.ErrorIs(t, err, ErrNewerCatalog)

	s, _ := Open(filepath.Join(t.TempDir(), "other.json"))
	_, err = s.MergeCatalog(Catalog{Version: CatalogVersion + 1})
	require.ErrorIs(t, err, ErrNewerCatalog)
}
