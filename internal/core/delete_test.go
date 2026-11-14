package core

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/akfaiz/carryover/internal/cloudsync/syncer"
	"github.com/akfaiz/carryover/internal/library"
)

// uploaded plays the game once, headless, so its saves reach the cloud.
func uploaded(t *testing.T, c *Core, gameID string) {
	t.Helper()
	res, err := c.Play(context.Background(), gameID, RunOptions{}, HeadlessUI{})
	require.NoError(t, err)
	require.NoError(t, res.PostError)
}

// cloudFolder is where a sync key keeps its saves in the local test cloud.
func cloudFolder(cloudDir, key string) string { return filepath.Join(cloudDir, "games", key) }

func mkCloudFolder(t *testing.T, cloudDir, key string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Join(cloudFolder(cloudDir, key), "current", "saves"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(cloudFolder(cloudDir, key), "current", "saves", "x.sav"), []byte("x"), 0o644))
}

func TestValidKey(t *testing.T) {
	for _, k := range []string{"game", "game.emu", "0000-1111"} {
		assert.NoError(t, validKey(k), k)
	}
	for _, k := range []string{"", ".", "..", "a/b", `a\b`, "../x"} {
		assert.Error(t, validKey(k), "%q", k)
	}
}

// A game's save keys are its own, one per emulator, and any other folder of
// that game left in the cloud (an emulator removed from it earlier).
func TestSaveKeysOfIncludesEmulatorsAndStrayFolders(t *testing.T) {
	c, id, _, cloudDir := newPlayable(t)
	emu := addEmulator(t, c, id, "Eden", t.TempDir())
	mkCloudFolder(t, cloudDir, id+".oldemu")
	mkCloudFolder(t, cloudDir, "other-game.emu") // another game's: not ours

	keys, err := c.SaveKeysOf(context.Background(), id)
	require.NoError(t, err)
	assert.Equal(t, []string{id, id + "." + emu, id + ".oldemu"}, keys)

	// Without a cloud only what the library knows is listed.
	st := c.Settings.Get()
	st.Remote = ""
	require.NoError(t, c.Settings.Update(st))
	keys, err = c.SaveKeysOf(context.Background(), id)
	require.NoError(t, err)
	assert.Equal(t, []string{id, id + "." + emu}, keys)
}

// An emulator's save keys come from the games that use it, from the trash too,
// and from cloud folders of games that no longer do.
func TestSaveKeysOfEmulator(t *testing.T) {
	c, id, _, cloudDir := newPlayable(t)
	emu := addEmulator(t, c, id, "Eden", t.TempDir())
	require.NoError(t, c.Library.Delete(id)) // in the trash: still counts
	mkCloudFolder(t, cloudDir, "gone-game."+emu)
	mkCloudFolder(t, cloudDir, "gone-game.other-emu")

	keys, err := c.SaveKeysOfEmulator(context.Background(), emu)
	require.NoError(t, err)
	assert.Equal(t, []string{id + "." + emu, "gone-game." + emu}, keys)

	st := c.Settings.Get()
	st.Remote = ""
	require.NoError(t, c.Settings.Update(st))
	keys, err = c.SaveKeysOfEmulator(context.Background(), emu)
	require.NoError(t, err)
	assert.Equal(t, []string{id + "." + emu}, keys, "no cloud, no stray folders")
}

// Purging removes the cloud folder and this device's state, marker and copies.
func TestPurgeSavesRemovesCloudAndLocalRecords(t *testing.T) {
	c, id, _, cloudDir := newPlayable(t)
	uploaded(t, c, id)
	require.DirExists(t, cloudFolder(cloudDir, id))
	require.NoError(t, c.States.Save(id, syncer.State{LastError: "old"}))
	c.markSession(id, id, "")
	safety := syncer.SafetyGameDir(c.Dirs.SafetyDir(), id)
	require.NoError(t, os.MkdirAll(safety, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(safety, "copy.zip"), []byte("z"), 0o644))

	require.NoError(t, c.PurgeSaves(context.Background(), nil), "nothing to do")
	require.DirExists(t, cloudFolder(cloudDir, id))

	require.NoError(t, c.PurgeSaves(context.Background(), []string{id}))
	assert.NoDirExists(t, cloudFolder(cloudDir, id))
	assert.NoDirExists(t, safety)
	assert.NoFileExists(t, c.sessionMarkerPath(id))
	st, err := c.States.Load(id)
	require.NoError(t, err)
	assert.Empty(t, st.LastError, "the sync state was reset")
}

// A key that would reach outside games/ is refused, and nothing local is
// touched, so the call can be repeated.
func TestPurgeSavesRefusesBadKeys(t *testing.T) {
	c, id, _, cloudDir := newPlayable(t)
	uploaded(t, c, id)
	require.NoError(t, c.States.Save(id, syncer.State{LastError: "keep"}))

	err := c.PurgeSaves(context.Background(), []string{id, "../escape"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid save folder")
	assert.NoDirExists(t, cloudFolder(cloudDir, id), "the valid one before it was deleted")
	st, _ := c.States.Load(id)
	assert.Equal(t, "keep", st.LastError)
}

func TestPurgeSavesWithoutACloudStillForgetsLocally(t *testing.T) {
	c, id, _, _ := newPlayable(t)
	st := c.Settings.Get()
	st.Remote = ""
	require.NoError(t, c.Settings.Update(st))
	require.NoError(t, c.States.Save(id, syncer.State{LastError: "x"}))
	require.NoError(t, c.PurgeSaves(context.Background(), []string{id}))
	got, _ := c.States.Load(id)
	assert.Empty(t, got.LastError)
}

// Images of a game removed from the library are deleted here and in the cloud,
// except those another game still uses.
func TestDropImagesKeepsSharedOnes(t *testing.T) {
	c, id, _, cloudDir := newPlayable(t)
	require.NoError(t, os.MkdirAll(c.Dirs.CoversDir(), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(cloudDir, "covers"), 0o755))
	put := func(name string) {
		for _, dir := range []string{c.Dirs.CoversDir(), filepath.Join(cloudDir, "covers")} {
			require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte("img"), 0o644))
		}
	}
	put("own.png")
	put("shared.png")

	other, err := c.Library.Save(library.Game{Name: "B", Cover: "shared.png", Slots: []library.SaveSlot{{Key: "s"}}}, c.DeviceID())
	require.NoError(t, err)
	require.NotEmpty(t, other.ID)
	g := mustFind(t, c, id)
	g.Cover, g.Art = "own.png", map[string]string{"hero": "shared.png"}
	g.Deleted = true // as the callers pass it: it comes from the trash

	c.DropImages(context.Background(), g)
	assert.NoFileExists(t, filepath.Join(c.Dirs.CoversDir(), "own.png"))
	assert.NoFileExists(t, filepath.Join(cloudDir, "covers", "own.png"), "gone from the cloud too")
	assert.FileExists(t, filepath.Join(c.Dirs.CoversDir(), "shared.png"), "another game uses it")
	assert.FileExists(t, filepath.Join(cloudDir, "covers", "shared.png"))
}

func TestDropImagesLeavesTheCloudAloneWhenCoversStayLocal(t *testing.T) {
	c, id, _, cloudDir := newPlayable(t)
	require.NoError(t, os.MkdirAll(c.Dirs.CoversDir(), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(cloudDir, "covers"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(c.Dirs.CoversDir(), "own.png"), []byte("img"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(cloudDir, "covers", "own.png"), []byte("img"), 0o644))
	st := c.Settings.Get()
	st.CoversLocalOnly = true
	require.NoError(t, c.Settings.Update(st))

	g := mustFind(t, c, id)
	g.Cover = "own.png"
	c.DropImages(context.Background(), g)
	assert.NoFileExists(t, filepath.Join(c.Dirs.CoversDir(), "own.png"))
	assert.FileExists(t, filepath.Join(cloudDir, "covers", "own.png"), "this device never touched the cloud images")

	c.DropImages(context.Background(), library.Game{ID: "no-images"}) // nothing to drop is fine
}

// Games and emulators past their retention are reduced and their images go.
func TestExpireTrashDropsTheImagesOfExpiredGames(t *testing.T) {
	c, id, _, _ := newPlayable(t)
	require.NoError(t, os.MkdirAll(c.Dirs.CoversDir(), 0o755))
	cover := filepath.Join(c.Dirs.CoversDir(), "old.png")
	require.NoError(t, os.WriteFile(cover, []byte("img"), 0o644))
	g := mustFind(t, c, id)
	g.Cover = "old.png"
	_, err := c.Library.Save(g, c.DeviceID())
	require.NoError(t, err)
	require.NoError(t, c.Library.Delete(id))

	expired, err := c.Library.ExpireTrash(time.Now().Add(library.TrashRetention + time.Hour))
	require.NoError(t, err)
	require.Len(t, expired, 1)
	c.DropImages(context.Background(), expired[0])
	assert.NoFileExists(t, cover)
	assert.NotPanics(t, func() { c.ExpireTrash(context.Background()) })
}
