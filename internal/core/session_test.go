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
	c.markSession(id, id, "")

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
	c.markSession(id, id, "")
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

// killedSession plays the game once, then leaves the state a session killed
// with the game leaves: cloud lock taken, marker written, saves changed since
// the last sync, no local lock.
func killedSession(t *testing.T) (c *Core, id, saves, cloudDir string) {
	t.Helper()
	c, id, saves, cloudDir = newPlayable(t)
	_, err := c.Play(context.Background(), id, RunOptions{}, &cancelUI{})
	require.NoError(t, err)
	eng, err := c.Engine()
	require.NoError(t, err)
	require.NoError(t, eng.AcquireLock(context.Background(), id))
	c.markSession(id, id, "")
	require.NoError(t, os.WriteFile(filepath.Join(saves, "a.sav"), []byte("played"), 0o644))
	return c, id, saves, cloudDir
}

// The session watcher uploads what a killed session didn't, and releases its lock.
func TestFinishSessionUploads(t *testing.T) {
	c, id, _, cloudDir := killedSession(t)
	finished, notice, err := c.FinishSession(context.Background(), id, 0)
	require.NoError(t, err)
	require.True(t, finished)
	require.NotNil(t, notice)
	require.Equal(t, NoticeUploaded, notice.Kind)
	require.Equal(t, "Game A", notice.Title)
	require.Equal(t, id, notice.Key)
	got, err := os.ReadFile(filepath.Join(cloudDir, "games", id, "current", "saves", "a.sav"))
	require.NoError(t, err)
	require.Equal(t, "played", string(got), "saves not uploaded")
	require.NoFileExists(t, filepath.Join(cloudDir, "games", id, "lock.json"), "cloud lock left behind")
	require.NoFileExists(t, c.sessionMarkerPath(id), "session marker left behind")

	finished, notice, err = c.FinishSession(context.Background(), id, 0)
	require.NoError(t, err)
	require.False(t, finished, "finished twice")
	require.Nil(t, notice)
}

// While the session's process is alive, the watcher waits.
func TestFinishSessionWhileRunning(t *testing.T) {
	c, id, _, cloudDir := killedSession(t)
	lockPath := filepath.Join(c.Dirs.StateDir(), id+".lock")
	require.NoError(t, os.WriteFile(lockPath, []byte(strconv.Itoa(os.Getppid())), 0o644))
	_, _, err := c.FinishSession(context.Background(), id, 0)
	require.ErrorIs(t, err, syncer.ErrLocked)
	require.FileExists(t, filepath.Join(cloudDir, "games", id, "lock.json"), "lock of a running session released")
	require.FileExists(t, c.sessionMarkerPath(id))
}

// A session that ended normally (no marker) needs nothing, even without a lock.
func TestFinishSessionAfterNormalEnd(t *testing.T) {
	c, id, saves, cloudDir := newPlayable(t)
	_, err := c.Play(context.Background(), id, RunOptions{}, &cancelUI{})
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(saves, "a.sav"), []byte("later"), 0o644))
	finished, notice, err := c.FinishSession(context.Background(), id, 0)
	require.NoError(t, err)
	require.False(t, finished)
	require.Nil(t, notice)
	got, _ := os.ReadFile(filepath.Join(cloudDir, "games", id, "current", "saves", "a.sav"))
	require.Equal(t, "save", string(got), "uploaded without a killed session")
}

// If another device changed the cloud saves meanwhile, the watcher doesn't
// overwrite either side: the conflict is flagged for the user.
func TestFinishSessionConflict(t *testing.T) {
	c, id, saves, cloudDir := killedSession(t)
	other, err := c.Engine()
	require.NoError(t, err)
	other.Device = syncer.Device{ID: "deck", Name: "deck"}
	other.States = &syncer.StateStore{Dir: t.TempDir()}
	otherSaves := t.TempDir()
	job := syncer.Job{GameID: id, Slots: []syncer.Slot{{Key: "saves", LocalPath: otherSaves}}}
	_, _, err = other.Sync(context.Background(), job, syncer.ResolveAuto)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(otherSaves, "a.sav"), []byte("deck"), 0o644))
	_, a, err := other.Sync(context.Background(), job, syncer.ResolveAuto)
	require.NoError(t, err)
	require.Equal(t, syncer.ActionPush, a)

	finished, notice, err := c.FinishSession(context.Background(), id, 0)
	require.NoError(t, err)
	require.True(t, finished)
	require.NotNil(t, notice)
	require.Equal(t, NoticeConflict, notice.Kind)
	local, _ := os.ReadFile(filepath.Join(saves, "a.sav"))
	require.Equal(t, "played", string(local), "local saves overwritten")
	cloud, _ := os.ReadFile(filepath.Join(cloudDir, "games", id, "current", "saves", "a.sav"))
	require.Equal(t, "deck", string(cloud), "cloud saves overwritten")
	st, err := c.States.Load(id)
	require.NoError(t, err)
	require.True(t, st.Conflict, "conflict not flagged")
	require.Empty(t, st.LastError, "conflict recorded as a sync error")
}

// Only a download is pending (the cloud changed, the saves here didn't): the
// watcher leaves the game folder alone; the next session pulls.
func TestFinishSessionNeverDownloads(t *testing.T) {
	c, id, saves, cloudDir := killedSession(t)
	require.NoError(t, os.WriteFile(filepath.Join(saves, "a.sav"), []byte("save"), 0o644)) // unchanged after all
	cur := filepath.Join(cloudDir, "games", id, "current", "saves", "a.sav")
	other, err := c.Engine()
	require.NoError(t, err)
	other.Device = syncer.Device{ID: "deck", Name: "deck"}
	other.States = &syncer.StateStore{Dir: t.TempDir()}
	otherSaves := t.TempDir()
	job := syncer.Job{GameID: id, Slots: []syncer.Slot{{Key: "saves", LocalPath: otherSaves}}}
	_, _, err = other.Sync(context.Background(), job, syncer.ResolveAuto)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(otherSaves, "a.sav"), []byte("deck"), 0o644))
	_, _, err = other.Sync(context.Background(), job, syncer.ResolveAuto)
	require.NoError(t, err)

	finished, notice, err := c.FinishSession(context.Background(), id, 0)
	require.NoError(t, err)
	require.True(t, finished)
	require.Nil(t, notice, "nothing uploaded, nothing to tell")
	local, _ := os.ReadFile(filepath.Join(saves, "a.sav"))
	require.Equal(t, "save", string(local), "watcher downloaded into the game folder")
	cloud, _ := os.ReadFile(cur)
	require.Equal(t, "deck", string(cloud))
}

// Play asks for a session watcher once the session holds the cloud lock.
func TestPlayStartsSessionWatcher(t *testing.T) {
	c, id, _, _ := newPlayable(t)
	got := make(chan string, 2)
	c.WatchSession = func(key string) { got <- key }
	_, err := c.Play(context.Background(), id, RunOptions{}, &cancelUI{})
	require.NoError(t, err)
	select {
	case key := <-got:
		require.Equal(t, id, key)
	case <-time.After(5 * time.Second):
		t.Fatal("session watcher not started")
	}
	_, err = c.Play(context.Background(), id, RunOptions{NoSync: true}, &cancelUI{})
	require.NoError(t, err)
	require.Empty(t, got, "watcher started for a session without sync")
}

// Notices are kept until taken, oldest first, and taken once.
func TestNotices(t *testing.T) {
	c, id, _, _ := newPlayable(t)
	require.Empty(t, c.TakeNotices())
	t0 := time.Now()
	require.NoError(t, c.SaveNotice(Notice{Time: t0.Add(time.Second), Key: id, Title: "B", Kind: NoticeConflict}))
	require.NoError(t, c.SaveNotice(Notice{Time: t0, Key: id, Title: "A", Kind: NoticeUploaded}))
	got := c.TakeNotices()
	require.Len(t, got, 2)
	require.Equal(t, "A", got[0].Title)
	require.Equal(t, NoticeConflict, got[1].Kind)
	require.Empty(t, c.TakeNotices(), "taken twice")
}

// A failed upload is reported to the user and recorded as the sync error.
func TestFinishSessionFailed(t *testing.T) {
	c, id, _, _ := killedSession(t)
	_, err := c.Library.UpdateDevice(id, c.DeviceID(), func(d *library.DeviceConfig) { d.Paths = nil })
	require.NoError(t, err)
	finished, notice, err := c.FinishSession(context.Background(), id, 0)
	require.NoError(t, err)
	require.True(t, finished, "the lock must still be released")
	require.NotNil(t, notice)
	require.Equal(t, NoticeFailed, notice.Kind)
	require.NotEmpty(t, notice.Err)
	st, _ := c.States.Load(id)
	require.Equal(t, notice.Err, st.LastError)
}

// A marker written by an older version is empty: the watcher releases the
// lock as RecoverSessions does, without knowing what to upload.
func TestFinishSessionOldMarker(t *testing.T) {
	c, id, _, cloudDir := killedSession(t)
	require.NoError(t, os.WriteFile(c.sessionMarkerPath(id), nil, 0o644))
	finished, notice, err := c.FinishSession(context.Background(), id, 0)
	require.NoError(t, err)
	require.True(t, finished)
	require.Nil(t, notice)
	require.NoFileExists(t, filepath.Join(cloudDir, "games", id, "lock.json"), "cloud lock left behind")
	got, _ := os.ReadFile(filepath.Join(cloudDir, "games", id, "current", "saves", "a.sav"))
	require.Equal(t, "save", string(got), "uploaded without knowing the profile")
}

// Stopping the watcher while it waits for the game to shut down leaves the
// session for the next recovery.
func TestFinishSessionCancelled(t *testing.T) {
	c, id, _, cloudDir := killedSession(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	finished, notice, err := c.FinishSession(ctx, id, time.Hour)
	require.ErrorIs(t, err, context.Canceled)
	require.False(t, finished)
	require.Nil(t, notice)
	require.FileExists(t, c.sessionMarkerPath(id))
	require.FileExists(t, filepath.Join(cloudDir, "games", id, "lock.json"))
	_, err = syncer.AcquireLocalLock(c.Dirs.StateDir(), id)
	require.NoError(t, err, "local lock still held after cancelling")
}

// Without a cloud remote there is nothing to finish.
func TestFinishSessionWithoutCloud(t *testing.T) {
	c, id, _, _ := killedSession(t)
	st := c.Settings.Get()
	st.Remote = ""
	require.NoError(t, c.Settings.Update(st))
	finished, notice, err := c.FinishSession(context.Background(), id, 0)
	require.NoError(t, err)
	require.False(t, finished)
	require.Nil(t, notice)
	require.FileExists(t, c.sessionMarkerPath(id), "marker of an unfinished session removed")
}

// An emulator profile: the cloud key names the session, the notice names the
// emulator and carries the profile's sync key.
func TestFinishSessionEmulator(t *testing.T) {
	c, id, _, cloudDir := newPlayable(t)
	eden, err := c.Library.SaveEmulator(library.Emulator{Name: "Eden"}, c.DeviceID())
	require.NoError(t, err)
	edenSaves := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(edenSaves, "e.sav"), []byte("eden"), 0o644))
	g := mustFind(t, c, id)
	g.Emulators = []library.GameEmulator{{EmulatorID: eden.ID, Slots: []library.SaveSlot{{Key: "saves"}}}}
	d := g.Devices[c.DeviceID()]
	d.Emulators = map[string]library.EmulatorConfig{eden.ID: {Paths: map[string]string{"saves": edenSaves}}}
	g.Devices[c.DeviceID()] = d
	g, err = c.Library.Save(g, c.DeviceID())
	require.NoError(t, err)

	eng, err := c.Engine()
	require.NoError(t, err)
	job, err := c.JobFor(g, eden.ID)
	require.NoError(t, err)
	_, _, err = eng.Sync(context.Background(), job, syncer.ResolveAuto)
	require.NoError(t, err)
	key := CloudKey(g, eden.ID)
	require.NoError(t, eng.AcquireLock(context.Background(), key))
	c.markSession(key, id, eden.ID)
	require.NoError(t, os.WriteFile(filepath.Join(edenSaves, "e.sav"), []byte("played"), 0o644))

	finished, notice, err := c.FinishSession(context.Background(), key, 0)
	require.NoError(t, err)
	require.True(t, finished)
	require.NotNil(t, notice)
	require.Equal(t, NoticeUploaded, notice.Kind)
	require.Equal(t, "Game A (Eden)", notice.Title)
	require.Equal(t, SyncKey(id, eden.ID), notice.Key)
	got, err := os.ReadFile(filepath.Join(cloudDir, "games", key, "current", "saves", "e.sav"))
	require.NoError(t, err)
	require.Equal(t, "played", string(got))
}

// A notice still being written (.tmp) is left alone; an unreadable one is
// dropped rather than shown or kept forever.
func TestTakeNoticesSkipsPartialAndBroken(t *testing.T) {
	c, id, _, _ := newPlayable(t)
	require.NoError(t, c.SaveNotice(Notice{Time: time.Now(), Key: id, Title: "A", Kind: NoticeUploaded}))
	partial := filepath.Join(c.noticesDir(), "00000000000000000001_x.json.tmp")
	require.NoError(t, os.WriteFile(partial, []byte("{"), 0o644))
	broken := filepath.Join(c.noticesDir(), "00000000000000000002_y.json")
	require.NoError(t, os.WriteFile(broken, []byte("not json"), 0o644))

	got := c.TakeNotices()
	require.Len(t, got, 1)
	require.Equal(t, "A", got[0].Title)
	require.FileExists(t, partial, "a notice being written was taken")
	require.NoFileExists(t, broken, "a broken notice stays forever")
}
