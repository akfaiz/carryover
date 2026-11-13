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

// A deleted game sits in the trash with its settings until it is restored or
// expires; a restore reaches the other devices; what expired is stripped and
// stays that way when an older copy is merged in.
func TestTrashRestoreAndExpire(t *testing.T) {
	dir := t.TempDir()
	pc, _ := Open(filepath.Join(dir, "pc.json"))
	deck, _ := Open(filepath.Join(dir, "deck.json"))
	g, err := pc.Save(Game{
		Name: "Zelda", Cover: "c.png", Slots: []SaveSlot{{Key: "saves"}},
		Devices: map[string]DeviceConfig{"pc": {Paths: map[string]string{"saves": "/e"}}},
	}, "pc")
	require.NoError(t, err)
	_, err = deck.MergeCatalog(pc.Catalog())
	require.NoError(t, err)

	time.Sleep(2 * time.Millisecond)
	require.NoError(t, pc.Delete(g.ID))
	require.NoError(t, pc.Delete(g.ID), "deleting twice changes nothing")
	_, err = pc.Get(g.ID)
	require.ErrorIs(t, err, ErrNotFound)
	trash := pc.Trash()
	require.Len(t, trash, 1)
	assert.Equal(t, "/e", trash[0].Devices["pc"].Paths["saves"], "settings are kept in the trash")
	assert.False(t, trash[0].DeletedAt.IsZero())

	// The deletion reaches the deck, with its settings, so the deck can restore it too.
	_, err = deck.MergeCatalog(pc.Catalog())
	require.NoError(t, err)
	assert.Empty(t, deck.List())
	assert.Len(t, deck.Trash(), 1)

	time.Sleep(2 * time.Millisecond)
	back, err := deck.Restore(g.ID)
	require.NoError(t, err)
	assert.False(t, back.Deleted)
	assert.Equal(t, "/e", back.Devices["pc"].Paths["saves"])
	assert.Empty(t, deck.Trash())
	_, err = pc.MergeCatalog(deck.Catalog())
	require.NoError(t, err)
	_, err = pc.Get(g.ID)
	require.NoError(t, err, "the restore reached the other device")

	// Past the retention the settings are removed for good.
	require.NoError(t, pc.Delete(g.ID))
	none, err := pc.ExpireTrash(time.Now().Add(TrashRetention - time.Hour))
	require.NoError(t, err)
	assert.Empty(t, none, "not expired yet")
	expired, err := pc.ExpireTrash(time.Now().Add(TrashRetention + time.Hour))
	require.NoError(t, err)
	require.Len(t, expired, 1)
	assert.Equal(t, "c.png", expired[0].Cover, "the caller gets the game as it was")
	assert.Empty(t, pc.Trash())
	_, err = pc.Restore(g.ID)
	require.ErrorIs(t, err, ErrNotRestorable)
	for _, c := range pc.Catalog().Games {
		assert.True(t, c.Deleted)
		assert.Equal(t, "Zelda", c.Name)
		assert.Empty(t, c.Devices)
		assert.Empty(t, c.Slots)
		assert.Empty(t, c.Cover)
	}

	// The deck still has the full tombstone: merging the stripped one replaces it
	// and the old settings don't come back.
	_, err = deck.MergeCatalog(pc.Catalog())
	require.NoError(t, err)
	for _, c := range deck.Catalog().Games {
		assert.Empty(t, c.Devices)
		assert.Empty(t, c.Slots)
	}
	_, err = deck.Restore(g.ID)
	require.ErrorIs(t, err, ErrNotRestorable)
}

func TestDeleteForeverAndEmulatorTrash(t *testing.T) {
	s, _ := Open(filepath.Join(t.TempDir(), "l.json"))
	g, err := s.Save(Game{Name: "Zelda", Slots: []SaveSlot{{Key: "saves"}}}, "pc")
	require.NoError(t, err)
	require.NoError(t, s.DeleteForever(g.ID))
	assert.Empty(t, s.Trash(), "nothing to restore")
	got, err := s.GetAny(g.ID)
	require.NoError(t, err)
	assert.True(t, got.Deleted)
	assert.Empty(t, got.Slots)

	e, err := s.SaveEmulator(Emulator{Name: "Eden", Devices: map[string]EmulatorDevice{"pc": {Exe: "eden"}}}, "pc")
	require.NoError(t, err)
	require.NoError(t, s.DeleteEmulator(e.ID))
	assert.Empty(t, s.Emulators())
	require.Len(t, s.TrashedEmulators(), 1)
	back, err := s.RestoreEmulator(e.ID)
	require.NoError(t, err)
	assert.Equal(t, "eden", back.Devices["pc"].Exe)
	assert.Len(t, s.Emulators(), 1)

	_, err = s.DeleteEmulatorEverywhere(e.ID)
	require.NoError(t, err)
	_, err = s.RestoreEmulator(e.ID)
	require.ErrorIs(t, err, ErrNotRestorable)
}

func TestDeleteEmulatorEverywhere(t *testing.T) {
	s, _ := Open(filepath.Join(t.TempDir(), "l.json"))
	e, err := s.SaveEmulator(Emulator{Name: "Eden", Devices: map[string]EmulatorDevice{"pc": {Exe: "eden"}}}, "pc")
	require.NoError(t, err)
	slots := []SaveSlot{{Key: "saves"}}
	used, err := s.Save(Game{
		Name: "Zelda",
		Emulators: []GameEmulator{
			{EmulatorID: e.ID, Slots: slots},
			{EmulatorID: "fork", Slots: slots, ShareWith: e.ID},
		},
		Devices: map[string]DeviceConfig{
			"pc": {
				LastEmulator: e.ID,
				Emulators: map[string]EmulatorConfig{
					e.ID:   {Rom: "z.nsp", Paths: map[string]string{"saves": "/e"}},
					"fork": {Rom: "z.nsp", Paths: map[string]string{"saves": "/f"}},
				},
			},
			"deck": {Emulators: map[string]EmulatorConfig{e.ID: {Rom: "z.nsp"}}},
		},
	}, "pc")
	require.NoError(t, err)
	unused, err := s.Save(Game{Name: "Native only", Slots: slots}, "pc")
	require.NoError(t, err)

	changed, err := s.DeleteEmulatorEverywhere(e.ID)
	require.NoError(t, err)
	assert.Equal(t, []string{used.ID}, changed)
	assert.Empty(t, s.Emulators())

	got, err := s.Get(used.ID)
	require.NoError(t, err)
	require.Len(t, got.Emulators, 1)
	assert.Equal(t, "fork", got.Emulators[0].EmulatorID)
	assert.Empty(t, got.Emulators[0].ShareWith, "the fork has its own save now")
	assert.Equal(t, slots, got.Emulators[0].Slots)
	assert.Empty(t, got.Devices["pc"].LastEmulator)
	assert.Contains(t, got.Devices["pc"].Emulators, "fork")
	assert.NotContains(t, got.Devices["pc"].Emulators, e.ID)
	assert.Empty(t, got.Devices["deck"].Emulators)
	_, err = s.Save(got, "pc") // still valid
	require.NoError(t, err)

	same, err := s.Get(unused.ID)
	require.NoError(t, err)
	assert.Equal(t, unused.UpdatedAt, same.UpdatedAt, "games without it are untouched")

	_, err = s.DeleteEmulatorEverywhere("nope")
	require.ErrorIs(t, err, ErrNotFound)
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

// Tombstones written before DeletedAt existed carry the deletion time in
// UpdatedAt: they show up in the trash with it and expire like the others.
func TestLegacyTombstonesUseUpdatedAt(t *testing.T) {
	s, _ := Open(filepath.Join(t.TempDir(), "l.json"))
	deleted := time.Now().Add(-10 * 24 * time.Hour).UTC()
	_, err := s.MergeCatalog(Catalog{Version: CatalogVersion,
		Games:     []Game{{ID: "g1", Name: "Old", Slots: []SaveSlot{{Key: "saves"}}, Deleted: true, UpdatedAt: deleted}},
		Emulators: []Emulator{{ID: "e1", Name: "Eden", Devices: map[string]EmulatorDevice{"pc": {Exe: "eden"}}, Deleted: true, UpdatedAt: deleted}},
	})
	require.NoError(t, err)
	require.Len(t, s.Trash(), 1)
	assert.WithinDuration(t, deleted, s.Trash()[0].DeletedTime(), time.Second)
	require.Len(t, s.TrashedEmulators(), 1)

	expired, err := s.ExpireTrash(time.Now().Add(TrashRetention - 15*24*time.Hour))
	require.NoError(t, err)
	assert.Empty(t, expired, "deleted 10 days ago: 20 days left")
	expired, err = s.ExpireTrash(time.Now().Add(TrashRetention - 5*24*time.Hour))
	require.NoError(t, err)
	assert.Len(t, expired, 1)
	assert.Empty(t, s.Trash())
	assert.Empty(t, s.TrashedEmulators())
}

// An emulator deleted for good, or expired from the trash, leaves no game
// pointing at it, games in the trash included (they keep their deletion time).
func TestEmulatorGoneLeavesNoDanglingGames(t *testing.T) {
	s, _ := Open(filepath.Join(t.TempDir(), "l.json"))
	mk := func(name string) Emulator {
		e, err := s.SaveEmulator(Emulator{Name: name, Devices: map[string]EmulatorDevice{"pc": {Exe: name}}}, "pc")
		require.NoError(t, err)
		return e
	}
	gameWith := func(name, emuID string) Game {
		g, err := s.Save(Game{Name: name, Emulators: []GameEmulator{{EmulatorID: emuID, Slots: []SaveSlot{{Key: "saves"}}}}}, "pc")
		require.NoError(t, err)
		return g
	}

	eden := mk("Eden")
	trashed := gameWith("In the trash", eden.ID)
	require.NoError(t, s.Delete(trashed.ID))
	deletedAt := s.Trash()[0].DeletedTime()
	time.Sleep(2 * time.Millisecond)
	_, err := s.DeleteEmulatorEverywhere(eden.ID)
	require.NoError(t, err)
	got, err := s.GetAny(trashed.ID)
	require.NoError(t, err)
	assert.Empty(t, got.Emulators, "a game in the trash drops it too")
	assert.True(t, got.Deleted)
	assert.Equal(t, deletedAt, got.DeletedTime(), "still expires on time")

	ryu := mk("Ryujinx")
	live := gameWith("Live", ryu.ID)
	require.NoError(t, s.DeleteEmulator(ryu.ID))
	g, _ := s.Get(live.ID)
	assert.Len(t, g.Emulators, 1, "still listed while it can be restored")
	_, err = s.ExpireTrash(time.Now().Add(TrashRetention + time.Hour))
	require.NoError(t, err)
	g, _ = s.Get(live.ID)
	assert.Empty(t, g.Emulators, "dropped once it expired")
}
