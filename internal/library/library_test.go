package library

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSaveAndMerge(t *testing.T) {
	dir := t.TempDir()
	pc, _ := Open(filepath.Join(dir, "pc.json"))
	deck, _ := Open(filepath.Join(dir, "deck.json"))

	g, err := pc.Save(Game{
		Name:  "Game A",
		Slots: []SaveSlot{{Key: "saves"}},
		Devices: map[string]DeviceConfig{
			"pc": {Paths: map[string]string{"saves": "{appdata}/GameA"}, Exe: `C:\Games\A\a.exe`},
		},
	}, "pc")
	require.NoError(t, err)

	// Deck receives the catalog and configures its own paths.
	changed, err := deck.MergeCatalog(pc.Catalog())
	require.NoError(t, err)
	require.True(t, changed, "merge")
	time.Sleep(2 * time.Millisecond)
	_, err = deck.UpdateDevice(g.ID, "deck", func(d *DeviceConfig) {
		d.Paths["saves"] = "{proton:shortcut}/AppData/Roaming/GameA"
	})
	require.NoError(t, err)

	// PC merges deck's catalog: keeps its own config and gains deck's.
	_, err = pc.MergeCatalog(deck.Catalog())
	require.NoError(t, err)
	got, _ := pc.Get(g.ID)
	require.Equal(t, `C:\Games\A\a.exe`, got.Devices["pc"].Exe, "merge lost device config")
	require.NotEmpty(t, got.Devices["deck"].Paths["saves"], "merge lost device config")

	// Saving on PC must not drop the deck's config.
	got.Name = "Game A (GOTY)"
	_, err = pc.Save(got, "pc")
	require.NoError(t, err)
	got, _ = pc.Get(g.ID)
	require.Contains(t, got.Devices, "deck", "deck config dropped on save")

	// Deletion propagates as a tombstone.
	require.NoError(t, pc.Delete(g.ID))
	_, err = deck.MergeCatalog(pc.Catalog())
	require.NoError(t, err)
	require.Empty(t, deck.List(), "deleted game still listed on deck")

	// Reload from disk.
	again, err := Open(filepath.Join(dir, "deck.json"))
	require.NoError(t, err, "reload")
	require.Len(t, again.Catalog().Games, 1)
}

func TestValidateSlots(t *testing.T) {
	s, _ := Open(filepath.Join(t.TempDir(), "l.json"))
	_, err := s.Save(Game{Name: "x", Slots: []SaveSlot{{Key: "a"}, {Key: "a"}}}, "d")
	require.Error(t, err, "duplicate key")
	_, err = s.Save(Game{Name: "x", Slots: []SaveSlot{{Key: "a/b"}}}, "d")
	require.Error(t, err, "invalid key")
}

// Two processes share one library file: a write by one must not drop what the
// other wrote in the meantime.
func TestPersistKeepsOtherProcessWrites(t *testing.T) {
	path := filepath.Join(t.TempDir(), "library.json")
	a, _ := Open(path)
	b, _ := Open(path)
	ga, err := a.Save(Game{Name: "A"}, "pc")
	require.NoError(t, err)
	gb, err := b.Save(Game{Name: "B"}, "pc") // b never saw A in memory
	require.NoError(t, err)
	c, _ := Open(path)
	_, err = c.Get(ga.ID)
	assert.NoError(t, err, "game A was lost")
	_, err = c.Get(gb.ID)
	assert.NoError(t, err, "game B was lost")
}
