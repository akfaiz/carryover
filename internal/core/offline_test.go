package core

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/akfaiz/carryover/internal/cloudsync/syncer"
	"github.com/akfaiz/carryover/internal/library"
)

// recordUI counts the prompts a session raises and can switch the network
// while the game runs.
type recordUI struct {
	HeadlessUI
	mu        sync.Mutex
	failures  int
	conflicts []bool // beforePlay of each conflict prompt
	phases    []string
	onRunning func(bool)
}

func (u *recordUI) Status(phase, _ string) {
	u.mu.Lock()
	u.phases = append(u.phases, phase)
	u.mu.Unlock()
}

func (u *recordUI) ConfirmSyncFailure(context.Context, library.Game, error, bool) bool {
	u.mu.Lock()
	u.failures++
	u.mu.Unlock()
	return true
}

func (u *recordUI) ResolveConflict(_ context.Context, _ library.Game, _ *syncer.Plan, before bool) syncer.Resolution {
	u.mu.Lock()
	u.conflicts = append(u.conflicts, before)
	u.mu.Unlock()
	return syncer.ResolveAuto // "Decide later"
}

func (u *recordUI) GameRunning(running bool) {
	if u.onRunning != nil {
		u.onRunning(running)
	}
}

// network makes c's connectivity check answer from a switch.
func network(t *testing.T, c *Core, online bool) *atomic.Bool {
	t.Helper()
	old := offlineNotice
	offlineNotice = 0
	t.Cleanup(func() { offlineNotice = old })
	var on atomic.Bool
	on.Store(online)
	c.probe = func(ctx context.Context) error {
		if on.Load() {
			return nil
		}
		return fmt.Errorf("%w: test", ErrOffline)
	}
	return &on
}

func requireMarker(t *testing.T, c *Core, key string, ended bool) {
	t.Helper()
	info, ok := c.readSession(key)
	require.True(t, ok, "session marker removed")
	require.Equal(t, ended, info.Ended)
}

// Offline before and after: the game starts without a prompt, nothing is
// recorded as a sync error, and the next session online uploads the saves.
func TestPlayOffline(t *testing.T) {
	c, id, saves, cloudDir := newPlayable(t)
	on := network(t, c, false)
	ui := &recordUI{}
	res, err := c.Play(context.Background(), id, RunOptions{}, ui)
	require.NoError(t, err)
	require.FileExists(t, filepath.Join(saves, "launched.txt"), "game didn't run")
	require.True(t, res.PreOffline)
	require.ErrorIs(t, res.PostError, ErrOffline)
	require.Zero(t, ui.failures, "asked about a sync failure while offline")
	require.Contains(t, ui.phases, "offline")
	st, _ := c.States.Load(id)
	require.Empty(t, st.LastError, "offline recorded as a sync error")
	require.NoFileExists(t, filepath.Join(cloudDir, "games", id, "lock.json"), "cloud lock taken offline")
	requireMarker(t, c, id, true)

	on.Store(true)
	res, err = c.Play(context.Background(), id, RunOptions{}, &recordUI{})
	require.NoError(t, err)
	require.Equal(t, syncer.ActionPush, res.PreAction, "saves played offline not uploaded before the next session")
	require.FileExists(t, filepath.Join(cloudDir, "games", id, "current", "saves", "launched.txt"))
	require.NoFileExists(t, c.sessionMarkerPath(id), "marker left after an online session")
}

// Connection lost while playing: no failure prompt, the cloud lock and the
// marker stay, the watcher and recovery give up at once without blocking
// the next session, which finishes everything once online.
func TestPlayOfflineAfterPlaying(t *testing.T) {
	c, id, _, cloudDir := newPlayable(t)
	on := network(t, c, true)
	ui := &recordUI{onRunning: func(r bool) {
		if r {
			on.Store(false)
		}
	}}
	res, err := c.Play(context.Background(), id, RunOptions{}, ui)
	require.NoError(t, err)
	require.False(t, res.PreOffline)
	require.ErrorIs(t, res.PostError, ErrOffline)
	require.Zero(t, ui.failures)
	lockFile := filepath.Join(cloudDir, "games", id, "lock.json")
	require.FileExists(t, lockFile, "cloud lock should stay until online")
	requireMarker(t, c, id, true)

	start := time.Now()
	finished, notice, err := c.FinishSession(context.Background(), id, 0)
	require.ErrorIs(t, err, ErrOffline)
	require.False(t, finished)
	require.Nil(t, notice, "the session already told the user")
	n, err := c.RecoverSessions(context.Background())
	require.NoError(t, err)
	require.Zero(t, n)
	require.Less(t, time.Since(start), 2*time.Second)
	requireMarker(t, c, id, true)

	// Playing again right away works, offline too.
	res, err = c.Play(context.Background(), id, RunOptions{}, &recordUI{})
	require.NoError(t, err)
	require.True(t, res.PreOffline)

	on.Store(true)
	_, err = c.Play(context.Background(), id, RunOptions{}, &recordUI{})
	require.NoError(t, err)
	require.NoFileExists(t, lockFile, "cloud lock left behind once online")
	require.NoFileExists(t, c.sessionMarkerPath(id))
	require.FileExists(t, filepath.Join(cloudDir, "games", id, "current", "saves", "launched.txt"))
}

// Offline before playing, back online after: the saves are uploaded and no
// lock is left (none was taken).
func TestPlayOnlineAfterPlayingOffline(t *testing.T) {
	c, id, _, cloudDir := newPlayable(t)
	on := network(t, c, false)
	ui := &recordUI{onRunning: func(r bool) {
		if r {
			on.Store(true)
		}
	}}
	res, err := c.Play(context.Background(), id, RunOptions{}, ui)
	require.NoError(t, err)
	require.True(t, res.PreOffline)
	require.NoError(t, res.PostError)
	require.Equal(t, syncer.ActionPush, res.PostAction)
	require.FileExists(t, filepath.Join(cloudDir, "games", id, "current", "saves", "launched.txt"))
	require.NoFileExists(t, filepath.Join(cloudDir, "games", id, "lock.json"))
	require.NoFileExists(t, c.sessionMarkerPath(id))
}

// A killed session found offline: the watcher returns at once, frees the
// game's local lock, keeps the marker and tells the user why nothing was
// uploaded.
func TestFinishSessionOffline(t *testing.T) {
	c, id, _, cloudDir := killedSession(t)
	network(t, c, false)
	finished, notice, err := c.FinishSession(context.Background(), id, 0)
	require.ErrorIs(t, err, ErrOffline)
	require.False(t, finished)
	require.NotNil(t, notice)
	require.Equal(t, NoticeOffline, notice.Kind)
	require.Equal(t, "Game A", notice.Title)
	requireMarker(t, c, id, false)
	require.FileExists(t, filepath.Join(cloudDir, "games", id, "lock.json"))
	lock, err := syncer.AcquireLocalLock(c.Dirs.StateDir(), id)
	require.NoError(t, err, "local lock still held")
	lock.Release()
}

// Play waits for another process (the previous session's watcher) to let go
// of the game, but not forever.
func TestPlayWaitsForLocalLock(t *testing.T) {
	c, id, saves, _ := newPlayable(t)
	held, err := syncer.AcquireLocalLock(c.Dirs.StateDir(), id)
	require.NoError(t, err)
	time.AfterFunc(700*time.Millisecond, held.Release)
	ui := &recordUI{}
	_, err = c.Play(context.Background(), id, RunOptions{NoSync: true}, ui)
	require.NoError(t, err)
	require.FileExists(t, filepath.Join(saves, "launched.txt"))

	old := localLockWait
	localLockWait = 200 * time.Millisecond
	t.Cleanup(func() { localLockWait = old })
	held, err = syncer.AcquireLocalLock(c.Dirs.StateDir(), id)
	require.NoError(t, err)
	defer held.Release()
	_, err = c.Play(context.Background(), id, RunOptions{NoSync: true}, ui)
	require.ErrorIs(t, err, syncer.ErrLocked)
}

// Offline before playing while another device uploaded, online after: the
// upload finds a conflict and asks. "Decide later" leaves both sides as they
// are and flags the game; nothing is recorded as an error.
func TestPlayOfflineThenConflict(t *testing.T) {
	c, id, saves, cloudDir := newPlayable(t)
	eng, err := c.Engine()
	require.NoError(t, err)
	_, _, err = eng.Sync(context.Background(), mustJob(t, c, id), syncer.ResolveAuto)
	require.NoError(t, err)

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

	on := network(t, c, false)
	ui := &recordUI{onRunning: func(r bool) {
		if r {
			on.Store(true)
		}
	}}
	res, err := c.Play(context.Background(), id, RunOptions{}, ui)
	require.NoError(t, err)
	require.True(t, res.PreOffline)
	require.NoError(t, res.PostError)
	require.Equal(t, syncer.ActionConflict, res.PostAction)
	require.Equal(t, []bool{false}, ui.conflicts, "expected one conflict prompt, after playing")
	require.Zero(t, ui.failures)

	local, _ := os.ReadFile(filepath.Join(saves, "a.sav"))
	require.Equal(t, "save", string(local), "local saves overwritten")
	require.FileExists(t, filepath.Join(saves, "launched.txt"))
	cloud, _ := os.ReadFile(filepath.Join(cloudDir, "games", id, "current", "saves", "a.sav"))
	require.Equal(t, "deck", string(cloud), "cloud saves overwritten")
	require.NoFileExists(t, filepath.Join(cloudDir, "games", id, "current", "saves", "launched.txt"))
	st, err := c.States.Load(id)
	require.NoError(t, err)
	require.True(t, st.Conflict, "conflict not flagged")
	require.Empty(t, st.LastError)
	require.NoFileExists(t, filepath.Join(cloudDir, "games", id, "lock.json"))
	require.NoFileExists(t, c.sessionMarkerPath(id))
}

func mustJob(t *testing.T, c *Core, id string) syncer.Job {
	t.Helper()
	job, err := c.Job(mustFind(t, c, id))
	require.NoError(t, err)
	return job
}
