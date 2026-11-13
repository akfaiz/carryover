package core

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/akfaiz/carryover/internal/cloudsync/syncer"
	"github.com/akfaiz/carryover/internal/library"
)

// A game in the library, or in the trash, never has leftover folders; one
// deleted for good, or unknown, does.
func TestIsLeftover(t *testing.T) {
	c, id, _, _ := newPlayable(t)
	assert.False(t, c.isLeftover(id), "a live game with save slots")
	assert.True(t, c.isLeftover("unknown-game"))

	emu := addEmulator(t, c, id, "Eden", t.TempDir())
	assert.False(t, c.isLeftover(id+"."+emu))
	assert.True(t, c.isLeftover(id+".other-emulator"), "the game doesn't use that emulator")

	// A game with only emulators doesn't sync its bare-id folder.
	only, err := c.Library.Save(library.Game{Name: "Only emu"}, c.DeviceID())
	require.NoError(t, err)
	assert.True(t, c.isLeftover(only.ID))

	require.NoError(t, c.Library.Delete(id))
	assert.False(t, c.isLeftover(id), "in the trash it can still be restored")
	require.NoError(t, c.Library.DeleteForever(id))
	assert.True(t, c.isLeftover(id), "deleted for good")
}

func TestLeftoversListsWhatNoGameUses(t *testing.T) {
	c, id, _, cloudDir := newPlayable(t)
	ctx := context.Background()
	uploaded(t, c, id)

	got, err := c.Leftovers(ctx)
	require.NoError(t, err)
	assert.Empty(t, got, "the game uses its folder")
	n, err := c.CountLeftovers(ctx)
	require.NoError(t, err)
	assert.Zero(t, n)

	// The game is deleted for good: its folder is a leftover, with what is in it.
	require.NoError(t, c.Library.DeleteForever(id))
	mkCloudFolder(t, cloudDir, "stranger") // no manifest at all
	got, err = c.Leftovers(ctx)
	require.NoError(t, err)
	require.Len(t, got, 2)
	byKey := map[string]Leftover{}
	for _, l := range got {
		byKey[l.Key] = l
	}
	l := byKey[id]
	assert.Equal(t, "Game A", l.GameName, "the tombstone keeps the name")
	assert.Equal(t, []string{"saves"}, l.Slots)
	assert.Positive(t, l.Files)
	assert.Positive(t, l.Bytes)
	assert.False(t, l.Time.IsZero())
	assert.Equal(t, got[0].Key, id, "the one with a sync time comes first")
	s := byKey["stranger"]
	assert.Empty(t, s.GameName)
	assert.Zero(t, s.Files)

	n, err = c.CountLeftovers(ctx)
	require.NoError(t, err)
	assert.Equal(t, 2, n)
}

func TestLeftoversNeedACloud(t *testing.T) {
	c, _, _, _ := newPlayable(t)
	st := c.Settings.Get()
	st.Remote = ""
	require.NoError(t, c.Settings.Update(st))
	got, err := c.Leftovers(context.Background())
	require.NoError(t, err)
	assert.Nil(t, got)
	n, err := c.CountLeftovers(context.Background())
	require.NoError(t, err)
	assert.Zero(t, n)
}

func TestLeftoversShowEmulatorNamesAndBackups(t *testing.T) {
	c, id, _, cloudDir := newPlayable(t)
	emu := addEmulator(t, c, id, "Eden", t.TempDir())
	key := id + "." + emu
	mkCloudFolder(t, cloudDir, key)
	require.NoError(t, os.MkdirAll(filepath.Join(cloudFolder(cloudDir, key), "backups"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(cloudFolder(cloudDir, key), "backups", "b.zip"), []byte("z"), 0o644))

	// The emulator is removed from the game: its folder is left over.
	g := mustFind(t, c, id)
	g.Emulators = nil
	_, err := c.Library.Save(g, c.DeviceID())
	require.NoError(t, err)

	got, err := c.Leftovers(context.Background())
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, key, got[0].Key)
	assert.Equal(t, "Eden", got[0].EmulatorName)
	assert.Equal(t, emu, got[0].EmulatorID)
	assert.Equal(t, "Game A", got[0].GameName)
	assert.Equal(t, 1, got[0].Backups)
}

func TestDeleteLeftover(t *testing.T) {
	c, id, _, cloudDir := newPlayable(t)
	uploaded(t, c, id)
	ctx := context.Background()

	require.Error(t, c.DeleteLeftover(ctx, id), "a game in the library uses these saves")
	require.Error(t, c.DeleteLeftover(ctx, "../x"))
	require.DirExists(t, cloudFolder(cloudDir, id))

	require.NoError(t, c.Library.DeleteForever(id))
	require.NoError(t, c.DeleteLeftover(ctx, id))
	assert.NoDirExists(t, cloudFolder(cloudDir, id))
}

// Restoring a game deleted for good brings it back under its id with the slots
// found in the cloud; the paths on this device are for the user to set.
func TestRestoreLeftoverRecreatesTheGame(t *testing.T) {
	c, id, _, cloudDir := newPlayable(t)
	uploaded(t, c, id)
	require.NoError(t, c.Library.DeleteForever(id))
	ctx := context.Background()

	got, err := c.RestoreLeftover(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, id, got)
	g, err := c.Library.Get(id)
	require.NoError(t, err)
	assert.Equal(t, "Game A", g.Name)
	require.Len(t, g.Slots, 1)
	assert.Equal(t, "saves", g.Slots[0].Key)
	dev, ok := g.Devices[c.DeviceID()]
	require.True(t, ok)
	assert.Empty(t, dev.Paths["saves"], "where the saves live here isn't known")
	assert.DirExists(t, cloudFolder(cloudDir, id), "the cloud saves stay")

	_, err = c.RestoreLeftover(ctx, id)
	require.Error(t, err, "it is in the library now")
	_, err = c.RestoreLeftover(ctx, "a/b")
	require.Error(t, err)
}

// A folder of a game the library never heard of gets a placeholder name, and
// its slots come from the folders under current/ when there is no manifest.
func TestRestoreLeftoverOfAnUnknownGame(t *testing.T) {
	c, _, _, cloudDir := newPlayable(t)
	const key = "12345678-aaaa-bbbb-cccc-000000000000"
	mkCloudFolder(t, cloudDir, key)

	got, err := c.RestoreLeftover(context.Background(), key)
	require.NoError(t, err)
	assert.Equal(t, key, got)
	g, err := c.Library.Get(key)
	require.NoError(t, err)
	assert.Equal(t, "Recovered game 12345678", g.Name)
	require.Len(t, g.Slots, 1)
	assert.Equal(t, "saves", g.Slots[0].Key)
}

func TestRestoreLeftoverOfAnEmulatorRemovedFromAGame(t *testing.T) {
	c, id, _, cloudDir := newPlayable(t)
	emu := addEmulator(t, c, id, "Eden", t.TempDir())
	key := id + "." + emu
	mkCloudFolder(t, cloudDir, key)
	g := mustFind(t, c, id)
	g.Emulators = nil
	_, err := c.Library.Save(g, c.DeviceID())
	require.NoError(t, err)
	ctx := context.Background()

	got, err := c.RestoreLeftover(ctx, key)
	require.NoError(t, err)
	assert.Equal(t, id, got)
	g = mustFind(t, c, id)
	require.Len(t, g.Emulators, 1)
	assert.Equal(t, emu, g.Emulators[0].EmulatorID)
	require.Len(t, g.Emulators[0].Slots, 1)

	// An emulator that no longer exists can't come back this way.
	mkCloudFolder(t, cloudDir, id+".vanished")
	_, err = c.RestoreLeftover(ctx, id+".vanished")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no longer exists")

	// Nor one that was deleted for good; one in the trash is restored with it.
	require.NoError(t, c.Library.DeleteEmulator(emu))
	g = mustFind(t, c, id)
	g.Emulators = nil
	_, err = c.Library.Save(g, c.DeviceID())
	require.NoError(t, err)
	_, err = c.RestoreLeftover(ctx, key)
	require.NoError(t, err)
	e, err := c.Library.Emulator(emu)
	require.NoError(t, err)
	assert.False(t, e.Deleted, "restored from the trash")
}

func TestSlotsOfAManifest(t *testing.T) {
	m := &syncer.Manifest{Files: map[string]syncer.FileInfo{
		"b/one.sav": {}, "a/x.sav": {}, "a/y.sav": {}, "loose.txt": {},
	}}
	assert.Equal(t, []string{"a", "b"}, slotsOf(m), "sorted, one per slot, files outside a slot ignored")
	assert.Empty(t, slotsOf(&syncer.Manifest{}))
}
