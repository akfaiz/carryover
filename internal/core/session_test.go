package core

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"testing"
	"time"

	"github.com/rclone/rclone/fs/config"
	"github.com/stretchr/testify/require"

	"github.com/akfaiz/carryover/internal/app/appdir"
	"github.com/akfaiz/carryover/internal/cloudsync/syncer"
	"github.com/akfaiz/carryover/internal/library"
)

// cancelUI cancels the sync phase number cancelAt (1 = before playing,
// 2 = the upload after playing) as soon as it starts.
type cancelUI struct {
	HeadlessUI
	cancelAt int
	phase    int
}

func (u *cancelUI) SyncCancelable(cancel func()) {
	if cancel == nil {
		return
	}
	u.phase++
	if u.phase == u.cancelAt {
		cancel()
	}
}

// newPlayable sets up an isolated Carryover with a local "cloud" remote and a
// game whose "executable" writes launched.txt into the save folder.
func newPlayable(t *testing.T) (c *Core, gameID, saves, cloudDir string) {
	t.Helper()
	home := t.TempDir()
	t.Setenv(appdir.EnvHome, home)
	c, err := New()
	require.NoError(t, err)
	config.FileSetValue("cloud", "type", "local")
	cloudDir = filepath.Join(home, "cloud")
	st := c.Settings.Get()
	st.Remote, st.RemoteRoot = "cloud", filepath.ToSlash(cloudDir)
	require.NoError(t, c.Settings.Update(st))
	saves = filepath.Join(home, "saves")
	require.NoError(t, os.MkdirAll(saves, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(saves, "a.sav"), []byte("save"), 0o644))
	exe, args := "sh", "-c \"echo x > launched.txt\""
	if runtime.GOOS == "windows" {
		exe, args = "cmd.exe", "/c echo x>launched.txt"
	}
	g, err := c.Library.Save(library.Game{
		Name:  "Game A",
		Slots: []library.SaveSlot{{Key: "saves"}},
		Devices: map[string]library.DeviceConfig{c.DeviceID(): {
			Paths: map[string]string{"saves": saves}, Exe: exe, Args: args, WorkDir: saves,
			LaunchMode: library.LaunchDirect,
		}},
	}, c.DeviceID())
	require.NoError(t, err)
	return c, g.ID, saves, cloudDir
}

// Cancelling the sync before playing doesn't launch the game.
func TestPlayCancelBeforeLaunch(t *testing.T) {
	c, id, saves, _ := newPlayable(t)
	_, err := c.Play(context.Background(), id, RunOptions{}, &cancelUI{cancelAt: 1})
	require.ErrorIs(t, err, ErrCancelled)
	require.NoFileExists(t, filepath.Join(saves, "launched.txt"), "game was launched although the sync was cancelled")
	st, _ := c.States.Load(id)
	require.Empty(t, st.LastError, "cancel recorded as a sync error")
}

// Cancelling the upload after playing keeps the game's run, uploads nothing
// and isn't treated as a sync failure; the next sync uploads the saves.
func TestPlayCancelUpload(t *testing.T) {
	c, id, saves, cloudDir := newPlayable(t)
	res, err := c.Play(context.Background(), id, RunOptions{}, &cancelUI{cancelAt: 2})
	require.NoError(t, err, "Play err =")
	require.FileExists(t, filepath.Join(saves, "launched.txt"), "game didn't run")
	require.ErrorIs(t, res.PostError, ErrCancelled)
	st, _ := c.States.Load(id)
	require.Empty(t, st.LastError, "cancel recorded as a sync error")
	current := filepath.Join(cloudDir, "games", id, "current", "saves", "launched.txt")
	require.NoFileExists(t, current, "saves were uploaded although the upload was cancelled")

	eng, err := c.Engine()
	require.NoError(t, err)
	job, err := c.Job(mustFind(t, c, id))
	require.NoError(t, err)
	_, a, err := eng.Sync(context.Background(), job, syncer.ResolveAuto)
	require.NoError(t, err)
	require.Equal(t, syncer.ActionPush, a, "next sync")
	require.FileExists(t, current, "next sync didn't upload")
}

func mustFind(t *testing.T, c *Core, id string) library.Game {
	t.Helper()
	g, err := c.Library.Get(id)
	require.NoError(t, err)
	return g
}

// The lock is written in the background while the game starts; it must be
// gone again once the session ends.
func TestPlayReleasesLock(t *testing.T) {
	c, id, _, cloudDir := newPlayable(t)
	res, err := c.Play(context.Background(), id, RunOptions{}, &cancelUI{})
	require.NoError(t, err)
	require.NoError(t, res.PostError)
	require.True(t, res.PreAction == syncer.ActionPush || res.PostAction == syncer.ActionPush,
		"expected an upload, got pre %s post %s", res.PreAction, res.PostAction)
	require.NoFileExists(t, filepath.Join(cloudDir, "games", id, "lock.json"), "lock left behind")
	require.NoFileExists(t, c.sessionMarkerPath(id), "session marker left behind")
}

// A session killed before it released the cloud lock leaves a marker behind;
// RecoverSessions releases the lock, and leaves sessions still running alone.
func TestRecoverSessions(t *testing.T) {
	c, id, _, cloudDir := newPlayable(t)
	eng, err := c.Engine()
	require.NoError(t, err)
	lockFile := filepath.Join(cloudDir, "games", id, "lock.json")
	// The killed wrapper: lock written, marker left, no local lock held.
	require.NoError(t, eng.AcquireLock(context.Background(), id))
	c.markSession(id)

	// Another live process (the parent, here) holds the game's local lock:
	// nothing is released.
	lockPath := filepath.Join(c.Dirs.StateDir(), id+".lock")
	require.NoError(t, os.WriteFile(lockPath, []byte(strconv.Itoa(os.Getppid())), 0o644))
	n, err := c.RecoverSessions(context.Background())
	require.NoError(t, err)
	require.Zero(t, n, "RecoverSessions while running")
	require.FileExists(t, lockFile, "lock of a running session was released")
	require.NoError(t, os.Remove(lockPath))

	n, err = c.RecoverSessions(context.Background())
	require.NoError(t, err)
	require.Equal(t, 1, n, "RecoverSessions")
	require.NoFileExists(t, lockFile, "cloud lock still there after recovery")
	require.NoFileExists(t, c.sessionMarkerPath(id), "session marker still there after recovery")
	n, _ = c.RecoverSessions(context.Background())
	require.Zero(t, n, "second RecoverSessions")
}

// A game Play is running in this process is never recovered, even though its
// local lock (same process id) looks takeable.
func TestRecoverSessionsSkipsPlayingGame(t *testing.T) {
	c, id, _, cloudDir := newPlayable(t)
	eng, err := c.Engine()
	require.NoError(t, err)
	require.NoError(t, eng.AcquireLock(context.Background(), id))
	c.markSession(id)
	c.playing.Store(id, struct{}{})
	n, err := c.RecoverSessions(context.Background())
	require.NoError(t, err)
	require.Zero(t, n)
	require.FileExists(t, filepath.Join(cloudDir, "games", id, "lock.json"), "lock of a game being played was released")
}

// Another device excludes a file from the sync and publishes the new filter in
// the library catalog. This device only learns of it while Play checks the cloud;
// the plan made with the old filter would delete the now-excluded file here.
func TestPlayReplansWithFiltersFromCatalog(t *testing.T) {
	c, id, saves, cloudDir := newPlayable(t)
	require.NoError(t, os.WriteFile(filepath.Join(saves, "extra.dat"), []byte("keep me"), 0o644))
	_, err := c.Play(context.Background(), id, RunOptions{}, &cancelUI{})
	require.NoError(t, err)

	// The deck syncs the same cloud folder, excludes extra.dat and changes a.sav.
	deckSaves := t.TempDir()
	deck, err := c.Engine()
	require.NoError(t, err)
	deck.Device = syncer.Device{ID: "deck", Name: "deck"}
	deck.States = &syncer.StateStore{Dir: t.TempDir()}
	deckJob := syncer.Job{GameID: id, Slots: []syncer.Slot{{Key: "saves", LocalPath: deckSaves}}}
	deckJob.Slots[0].Filters.Exclude = []string{"extra.dat"}
	_, _, err = deck.Sync(context.Background(), deckJob, syncer.ResolveAuto)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(deckSaves, "a.sav"), []byte("changed on the deck"), 0o644))
	_, a, err := deck.Sync(context.Background(), deckJob, syncer.ResolveAuto)
	require.NoError(t, err)
	require.Equal(t, syncer.ActionPush, a)

	// ...and publishes the filter with the game in the catalog.
	catalogFile := filepath.Join(cloudDir, "library.json")
	b, err := os.ReadFile(catalogFile)
	require.NoError(t, err)
	var cat library.Catalog
	require.NoError(t, json.Unmarshal(b, &cat))
	require.Len(t, cat.Games, 1)
	cat.Games[0].Slots[0].Exclude = []string{"extra.dat"}
	cat.Games[0].UpdatedAt = time.Now().Add(time.Minute)
	b, err = json.Marshal(cat)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(catalogFile, b, 0o644))

	res, err := c.Play(context.Background(), id, RunOptions{}, &cancelUI{})
	require.NoError(t, err)
	require.Equal(t, syncer.ActionPull, res.PreAction)
	require.FileExists(t, filepath.Join(saves, "extra.dat"), "the file the new filter excludes was deleted")
	got, err := os.ReadFile(filepath.Join(saves, "a.sav"))
	require.NoError(t, err)
	require.Equal(t, "changed on the deck", string(got))
}
