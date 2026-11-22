package core

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/akfaiz/carryover/internal/cloudsync/syncer"
)

// requireLocalLockFree fails if a process holds key's local lock.
func requireLocalLockFree(t *testing.T, c *Core, key string) {
	t.Helper()
	l, err := syncer.AcquireLocalLock(c.Dirs.StateDir(), key)
	require.NoError(t, err, "local lock held")
	l.Release()
}

// StartSession syncs before playing and takes the cloud lock without launching
// anything; the session stays open, holding no local lock, until EndSession
// uploads what the game changed, releases the lock and removes the marker.
func TestStartAndEndSession(t *testing.T) {
	c, id, saves, cloudDir := newPlayable(t)
	lockFile := filepath.Join(cloudDir, "games", id, "lock.json")
	ui := &recordUI{}

	start, err := c.StartSession(context.Background(), id, "", ui)
	require.NoError(t, err)
	require.Equal(t, SessionStart{Key: id, CloudKey: id, PreAction: syncer.ActionPush, Synced: true}, start)
	require.NoFileExists(t, filepath.Join(saves, "launched.txt"), "StartSession launched the game")
	require.FileExists(t, lockFile, "cloud lock not taken")
	require.True(t, c.SessionOpen(id))
	info, ok := c.readSession(id)
	require.True(t, ok)
	require.Equal(t, sessionInfo{Game: id, Detached: true}, info)
	requireLocalLockFree(t, c, id)
	_, playing := c.playing.Load(id)
	require.False(t, playing, "still marked as playing in this process")

	require.Equal(t, []OpenSession{{CloudKey: id, Key: id, Game: id}}, c.OpenSessions())

	_, err = c.StartSession(context.Background(), id, "", ui)
	require.ErrorIs(t, err, ErrPlaying, "second start of an open session")
	require.ErrorIs(t, err, syncer.ErrLocked, "ErrPlaying wraps ErrLocked")

	// An open session isn't a killed one.
	n, err := c.RecoverSessions(context.Background())
	require.NoError(t, err)
	require.Zero(t, n, "RecoverSessions finished an open session")
	require.FileExists(t, lockFile)

	// The game writes a save; the session ends.
	require.NoError(t, os.WriteFile(filepath.Join(saves, "b.sav"), []byte("played"), 0o644))
	finished, notice, err := c.EndSession(context.Background(), id)
	require.NoError(t, err)
	require.True(t, finished)
	require.NotNil(t, notice)
	require.Equal(t, NoticeUploaded, notice.Kind)
	require.FileExists(t, filepath.Join(cloudDir, "games", id, "current", "saves", "b.sav"), "save not uploaded")
	require.NoFileExists(t, lockFile, "cloud lock not released")
	require.NoFileExists(t, c.sessionMarkerPath(id), "marker left behind")
	require.False(t, c.SessionOpen(id))
	require.Empty(t, c.OpenSessions())

	finished, _, err = c.EndSession(context.Background(), id)
	require.NoError(t, err)
	require.False(t, finished, "second EndSession")
}

// Offline, the session starts without syncing and takes no cloud lock; ending
// it offline keeps the marker, marked ended, so the session is no longer open
// (syncs may run) and RecoverSessions finishes it once online.
func TestStartSessionOffline(t *testing.T) {
	c, id, _, cloudDir := newPlayable(t)
	net := network(t, c, false)

	start, err := c.StartSession(context.Background(), id, "", &recordUI{})
	require.NoError(t, err)
	require.True(t, start.PreOffline)
	require.Equal(t, syncer.ActionNone, start.PreAction)
	require.True(t, c.SessionOpen(id))
	require.NoFileExists(t, filepath.Join(cloudDir, "games", id, "lock.json"))

	_, notice, err := c.EndSession(context.Background(), id)
	require.ErrorIs(t, err, ErrOffline)
	require.NotNil(t, notice)
	require.Equal(t, NoticeOffline, notice.Kind)
	requireMarker(t, c, id, true)
	require.False(t, c.SessionOpen(id), "an ended session is still open")
	require.Empty(t, c.OpenSessions(), "an ended session is listed as open")

	net.Store(true)
	n, err := c.RecoverSessions(context.Background())
	require.NoError(t, err)
	require.Equal(t, 1, n)
	require.NoFileExists(t, c.sessionMarkerPath(id))
}

// Cancelling the sync before playing starts nothing: no marker, no locks.
func TestStartSessionCancelled(t *testing.T) {
	c, id, _, cloudDir := newPlayable(t)
	_, err := c.StartSession(context.Background(), id, "", &cancelUI{cancelAt: 1})
	require.ErrorIs(t, err, ErrCancelled)
	require.False(t, c.SessionOpen(id))
	require.NoFileExists(t, c.sessionMarkerPath(id))
	require.NoFileExists(t, filepath.Join(cloudDir, "games", id, "lock.json"))
	requireLocalLockFree(t, c, id)
	st, _ := c.States.Load(id)
	require.Empty(t, st.LastError, "cancel recorded as a sync error")
}

// Without a cloud there is nothing to sync: the session isn't recorded and
// needs no EndSession.
func TestStartSessionWithoutCloud(t *testing.T) {
	c, id, _, _ := newPlayable(t)
	st := c.Settings.Get()
	st.Remote = ""
	require.NoError(t, c.Settings.Update(st))
	start, err := c.StartSession(context.Background(), id, "", &recordUI{})
	require.NoError(t, err)
	require.False(t, start.Synced)
	require.False(t, c.SessionOpen(id))
	require.NoFileExists(t, c.sessionMarkerPath(id))
}

// Any failure to end a session (not only being offline) ends it all the same:
// it is no longer open, so syncs run and the next play syncs before playing,
// and it is listed as pending until EndSession finishes it.
func TestEndSessionFailureEndsSession(t *testing.T) {
	c, id, saves, cloudDir := newPlayable(t)
	_, err := c.StartSession(context.Background(), id, "", &recordUI{})
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(saves, "b.sav"), []byte("played"), 0o644))

	c.probe = func(context.Context) error { return errors.New("token expired") }
	_, _, err = c.EndSession(context.Background(), id)
	require.Error(t, err)
	require.NotErrorIs(t, err, ErrOffline)
	requireMarker(t, c, id, true)
	require.False(t, c.SessionOpen(id), "a failed end left the session open")
	require.Equal(t, []PendingSession{{CloudKey: id, Key: id, Game: id}}, c.PendingSessions())

	// Retried later, it uploads and releases the lock.
	c.probe = nil
	finished, notice, err := c.EndSession(context.Background(), id)
	require.NoError(t, err)
	require.True(t, finished)
	require.NotNil(t, notice)
	require.FileExists(t, filepath.Join(cloudDir, "games", id, "current", "saves", "b.sav"))
	require.NoFileExists(t, c.sessionMarkerPath(id))
	require.Empty(t, c.PendingSessions())
}

// A session whose cloud was removed meanwhile has nothing to upload or
// release: ending it drops the marker instead of leaving it open for good.
func TestEndSessionWithoutCloud(t *testing.T) {
	c, id, _, _ := newPlayable(t)
	_, err := c.StartSession(context.Background(), id, "", &recordUI{})
	require.NoError(t, err)
	st := c.Settings.Get()
	st.Remote = ""
	require.NoError(t, c.Settings.Update(st))

	_, _, err = c.EndSession(context.Background(), id)
	require.NoError(t, err)
	require.NoFileExists(t, c.sessionMarkerPath(id))
	require.False(t, c.SessionOpen(id))
}

// A retry in the background (EndPending) finishes a session that ended
// offline, but never one that started again meanwhile.
func TestEndPending(t *testing.T) {
	c, id, saves, cloudDir := newPlayable(t)
	net := network(t, c, true)
	ctx := context.Background()
	_, err := c.StartSession(ctx, id, "", &recordUI{})
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(saves, "b.sav"), []byte("played"), 0o644))

	net.Store(false)
	_, _, err = c.EndSession(ctx, id)
	require.ErrorIs(t, err, ErrOffline)
	require.Len(t, c.PendingSessions(), 1)

	// Still offline: quiet, and still pending.
	finished, notice, err := c.EndPending(ctx, id)
	require.ErrorIs(t, err, ErrOffline)
	require.False(t, finished)
	require.Nil(t, notice, "a retry told the user again")
	require.Len(t, c.PendingSessions(), 1)

	// Played again before the network came back: the retry leaves it alone.
	net.Store(true)
	_, err = c.StartSession(ctx, id, "", &recordUI{})
	require.NoError(t, err)
	finished, notice, err = c.EndPending(ctx, id)
	require.NoError(t, err)
	require.False(t, finished)
	require.Nil(t, notice)
	require.True(t, c.SessionOpen(id), "a retry ended the session being played")
	_, _, err = c.EndSession(ctx, id)
	require.NoError(t, err)

	// A session left pending is finished by the retry.
	net.Store(false)
	_, err = c.StartSession(ctx, id, "", &recordUI{})
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(saves, "c.sav"), []byte("again"), 0o644))
	_, _, err = c.EndSession(ctx, id)
	require.ErrorIs(t, err, ErrOffline)
	net.Store(true)
	finished, notice, err = c.EndPending(ctx, id)
	require.NoError(t, err)
	require.True(t, finished)
	require.NotNil(t, notice)
	require.FileExists(t, filepath.Join(cloudDir, "games", id, "current", "saves", "c.sav"))
	require.Empty(t, c.PendingSessions())
}
