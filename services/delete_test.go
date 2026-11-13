package services

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Deleting an emulator "with saves" removes it from the games that use it and
// deletes its cloud folders, including a folder left by a game that dropped the
// emulator earlier; the game's own saves stay.
func TestDeleteEmulatorWithSaves(t *testing.T) {
	e := newSyncEnv(t)
	quiet(t, e)
	cloud := filepath.Join(filepath.FromSlash(e.c.Settings.Get().RemoteRoot), "games")
	bus := NewBus()
	emus := NewEmulatorService(e.c, bus)
	lib := NewLibraryService(e.c, bus, nil)

	em, err := emus.Save(EmulatorSetup{Name: "Eden", Exe: "eden"})
	require.NoError(t, err)
	edenSaves := filepath.Join(t.TempDir(), "eden")
	require.NoError(t, os.MkdirAll(edenSaves, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(edenSaves, "e.sav"), []byte("eden"), 0o644))

	v, err := lib.Get(e.id)
	require.NoError(t, err)
	in := GameInput{
		ID: v.ID, Name: v.Name, Slots: v.Slots, Device: DeviceInput{Paths: v.Device.Paths, Exe: v.Device.Exe},
		Emulators: []GameEmulatorInput{{EmulatorID: em.ID, Slots: v.Slots, Rom: "z.nsp", Paths: map[string]string{"saves": edenSaves}}},
	}
	_, err = lib.Save(in)
	require.NoError(t, err)
	_, err = e.svc.SyncAll()
	require.NoError(t, err)
	edenDir := filepath.Join(cloud, e.id+"."+em.ID)
	require.DirExists(t, edenDir)
	// A folder of a game that no longer exists, made with the same emulator.
	stray := filepath.Join(cloud, "gone."+em.ID)
	require.NoError(t, os.MkdirAll(stray, 0o755))

	require.NoError(t, emus.DeleteWithSaves(em.ID))

	assert.NoDirExists(t, edenDir)
	assert.NoDirExists(t, stray)
	assert.DirExists(t, filepath.Join(cloud, e.id), "the game's own saves stay")
	assert.Empty(t, emus.List())
	v, err = lib.Get(e.id)
	require.NoError(t, err)
	assert.Empty(t, v.Emulators, "the emulator is removed from the game")
	st, err := e.c.States.Load(e.id + "." + em.ID)
	require.NoError(t, err)
	assert.Nil(t, st.LastSynced, "its sync state is forgotten")
}

// A game that is deleted with its saves loses every cloud folder: its own and
// those of its emulators, also the ones of emulators it no longer lists.
func TestDeleteGameWithSaves(t *testing.T) {
	e := newSyncEnv(t)
	quiet(t, e)
	cloud := filepath.Join(filepath.FromSlash(e.c.Settings.Get().RemoteRoot), "games")
	lib := NewLibraryService(e.c, NewBus(), nil)

	_, err := e.svc.SyncAll()
	require.NoError(t, err)
	own := filepath.Join(cloud, e.id)
	other := filepath.Join(cloud, e.id+".old-emulator")
	unrelated := filepath.Join(cloud, "another-game")
	for _, d := range []string{other, unrelated} {
		require.NoError(t, os.MkdirAll(d, 0o755))
	}
	require.DirExists(t, own)

	require.NoError(t, lib.DeleteWithSaves(e.id))

	assert.NoDirExists(t, own)
	assert.NoDirExists(t, other)
	assert.DirExists(t, unrelated, "another game is untouched")
	_, err = e.c.Library.Get(e.id)
	require.Error(t, err)
	st, err := e.c.States.Load(e.id)
	require.NoError(t, err)
	assert.Nil(t, st.LastSynced)
	// The tombstone only identifies the game.
	for _, g := range e.c.Library.Catalog().Games {
		if g.ID == e.id {
			assert.True(t, g.Deleted)
			assert.Empty(t, g.Devices)
			assert.Empty(t, g.Slots)
		}
	}
}

// A deleted game waits in the trash and comes back with its settings.
func TestTrashRestore(t *testing.T) {
	e := newSyncEnv(t)
	quiet(t, e)
	lib := NewLibraryService(e.c, NewBus(), nil)

	require.NoError(t, lib.Delete(e.id))
	_, err := lib.Get(e.id)
	require.Error(t, err)
	trash := lib.Trash()
	require.Len(t, trash, 1)
	assert.Equal(t, "Game A", trash[0].Name)
	assert.True(t, trash[0].ExpiresAt.After(trash[0].DeletedAt))

	require.NoError(t, lib.Restore(e.id))
	v, err := lib.Get(e.id)
	require.NoError(t, err)
	assert.Equal(t, e.saves, v.Device.Paths["saves"], "settings are as they were")
	assert.Empty(t, lib.Trash())

	// A game deleted for good can't be restored.
	require.NoError(t, lib.DeleteForever(e.id))
	require.Error(t, lib.Restore(e.id))
	assert.Empty(t, lib.Trash())
}

// Saves of a game deleted for good, or of an emulator removed from its game,
// are listed as leftovers; they can be deleted or brought back into the library.
func TestLeftoverSaves(t *testing.T) {
	e := newSyncEnv(t)
	quiet(t, e)
	cloud := filepath.Join(filepath.FromSlash(e.c.Settings.Get().RemoteRoot), "games")
	bus := NewBus()
	emus := NewEmulatorService(e.c, bus)
	lib := NewLibraryService(e.c, bus, nil)

	em, err := emus.Save(EmulatorSetup{Name: "Eden", Exe: "eden"})
	require.NoError(t, err)
	edenSaves := filepath.Join(t.TempDir(), "eden")
	require.NoError(t, os.MkdirAll(edenSaves, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(edenSaves, "e.sav"), []byte("eden"), 0o644))
	v, err := lib.Get(e.id)
	require.NoError(t, err)
	in := GameInput{
		ID: v.ID, Name: v.Name, Slots: v.Slots, Device: DeviceInput{Paths: v.Device.Paths, Exe: v.Device.Exe},
		Emulators: []GameEmulatorInput{{EmulatorID: em.ID, Slots: v.Slots, Rom: "z.nsp", Paths: map[string]string{"saves": edenSaves}}},
	}
	_, err = lib.Save(in)
	require.NoError(t, err)
	_, err = e.svc.SyncAll()
	require.NoError(t, err)

	left, err := lib.Leftovers()
	require.NoError(t, err)
	assert.Empty(t, left, "every folder belongs to the game")

	// The emulator is removed from the game: its folder is left over, and can come back.
	in.Emulators = nil
	_, err = lib.Save(in)
	require.NoError(t, err)
	left, err = lib.Leftovers()
	require.NoError(t, err)
	require.Len(t, left, 1)
	assert.Equal(t, e.id+"."+em.ID, left[0].Key)
	assert.Equal(t, "Game A", left[0].GameName)
	assert.Equal(t, "Eden", left[0].EmulatorName)
	assert.Equal(t, []string{"saves"}, left[0].Slots)
	assert.Equal(t, 1, left[0].Files)
	id, err := lib.RestoreLeftover(left[0].Key)
	require.NoError(t, err)
	assert.Equal(t, e.id, id)
	v, err = lib.Get(e.id)
	require.NoError(t, err)
	require.Len(t, v.Emulators, 1)
	assert.Equal(t, em.ID, v.Emulators[0].EmulatorID)
	left, err = lib.Leftovers()
	require.NoError(t, err)
	assert.Empty(t, left)

	// The game is deleted for good, keeping its saves: both folders are left over.
	require.NoError(t, lib.DeleteForever(e.id))
	left, err = lib.Leftovers()
	require.NoError(t, err)
	require.Len(t, left, 2)
	var own, emu LeftoverView
	for _, l := range left {
		if l.EmulatorID == "" {
			own = l
		} else {
			emu = l
		}
	}
	assert.Equal(t, e.id, own.Key)
	assert.Equal(t, "Game A", own.GameName, "the tombstone remembers the name")

	// An emulator folder can't come back while its emulator is deleted for good.
	require.NoError(t, emus.DeleteForever(em.ID))
	_, err = lib.RestoreLeftover(emu.Key)
	require.Error(t, err)
	require.NoError(t, lib.DeleteLeftover(emu.Key))
	assert.NoDirExists(t, filepath.Join(cloud, emu.Key))

	// The game's own folder comes back as a game with the slots found in the cloud.
	id, err = lib.RestoreLeftover(own.Key)
	require.NoError(t, err)
	v, err = lib.Get(id)
	require.NoError(t, err)
	assert.Equal(t, "Game A", v.Name)
	require.Len(t, v.Slots, 1)
	assert.Equal(t, "saves", v.Slots[0].Key)
	assert.False(t, v.Ready, "the save location on this device is still to be set")

	// A folder a live game uses is not a leftover and can't be deleted this way.
	require.Error(t, lib.DeleteLeftover(own.Key))
	require.Error(t, lib.DeleteLeftover("../x"))
	assert.DirExists(t, filepath.Join(cloud, own.Key))
}

// Restoring only the emulator folder of a deleted game gives an emulator-only
// game; the game's own folder is then still a leftover that can be restored.
func TestRestoreBothLeftoversOfAGame(t *testing.T) {
	e := newSyncEnv(t)
	quiet(t, e)
	bus := NewBus()
	emus := NewEmulatorService(e.c, bus)
	lib := NewLibraryService(e.c, bus, nil)

	em, err := emus.Save(EmulatorSetup{Name: "Eden", Exe: "eden"})
	require.NoError(t, err)
	edenSaves := filepath.Join(t.TempDir(), "eden")
	require.NoError(t, os.MkdirAll(edenSaves, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(edenSaves, "e.sav"), []byte("eden"), 0o644))
	v, err := lib.Get(e.id)
	require.NoError(t, err)
	_, err = lib.Save(GameInput{
		ID: v.ID, Name: v.Name, Slots: v.Slots, Device: DeviceInput{Paths: v.Device.Paths, Exe: v.Device.Exe},
		Emulators: []GameEmulatorInput{{EmulatorID: em.ID, Slots: v.Slots, Rom: "z.nsp", Paths: map[string]string{"saves": edenSaves}}},
	})
	require.NoError(t, err)
	_, err = e.svc.SyncAll()
	require.NoError(t, err)
	require.NoError(t, lib.DeleteForever(e.id))

	_, err = lib.RestoreLeftover(e.id + "." + em.ID)
	require.NoError(t, err)
	v, err = lib.Get(e.id)
	require.NoError(t, err)
	assert.Empty(t, v.Slots)
	left, err := lib.Leftovers()
	require.NoError(t, err)
	require.Len(t, left, 1, "the game's own folder is still left over")
	assert.Equal(t, e.id, left[0].Key)
	n, err := lib.LeftoverCount()
	require.NoError(t, err)
	assert.Equal(t, 1, n)

	_, err = lib.RestoreLeftover(e.id)
	require.NoError(t, err)
	v, err = lib.Get(e.id)
	require.NoError(t, err)
	assert.Len(t, v.Slots, 1)
	assert.Len(t, v.Emulators, 1)
	left, err = lib.Leftovers()
	require.NoError(t, err)
	assert.Empty(t, left)
}
