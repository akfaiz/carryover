package services

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/rclone/rclone/fs/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/akfaiz/carryover/internal/app/appdir"
	"github.com/akfaiz/carryover/internal/cloudsync/syncer"
	"github.com/akfaiz/carryover/internal/core"
	"github.com/akfaiz/carryover/internal/library"
)

// A session started with Begin (the game started by the caller, as the Android
// app does) asks through prompts like Play, keeps manual syncs, backups and
// SyncAll away from the game while it is open, and End uploads and closes it.
func TestBeginAndEnd(t *testing.T) {
	home := t.TempDir()
	t.Setenv(appdir.EnvHome, home)
	t.Cleanup(background.Wait) // catalog uploads write into the home
	c, err := core.New()
	require.NoError(t, err)
	config.FileSetValue("cloud", "type", "local")
	cloudDir := filepath.Join(home, "cloud")
	st := c.Settings.Get()
	st.Remote, st.RemoteRoot = "cloud", filepath.ToSlash(cloudDir)
	require.NoError(t, c.Settings.Update(st))
	saves := filepath.Join(home, "saves")
	require.NoError(t, os.MkdirAll(saves, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(saves, "a.sav"), []byte("save"), 0o644))
	g, err := c.Library.Save(library.Game{
		Name:    "Game A",
		Slots:   []library.SaveSlot{{Key: "saves"}},
		Devices: map[string]library.DeviceConfig{c.DeviceID(): {Paths: map[string]string{"saves": saves}}},
	}, c.DeviceID())
	require.NoError(t, err)

	// Another device holds the cloud lock: Begin asks before playing.
	lockFile := filepath.Join(cloudDir, "games", g.ID, "lock.json")
	require.NoError(t, os.MkdirAll(filepath.Dir(lockFile), 0o755))
	b, _ := json.Marshal(syncer.Lock{Device: "other", DeviceName: "Phone B", Since: time.Now().UTC()})
	require.NoError(t, os.WriteFile(lockFile, b, 0o644))

	bus := NewBus()
	sessions := NewSessionService(c, bus)
	syncs := NewSyncService(c, bus)
	prompts := make(chan Prompt, 4)
	bus.OnPrompt(func(p Prompt) { prompts <- p })
	var mu sync.Mutex
	var statuses []Status
	bus.OnStatus(func(s Status) {
		mu.Lock()
		statuses = append(statuses, s)
		mu.Unlock()
	})
	last := func() Status {
		mu.Lock()
		defer mu.Unlock()
		require.NotEmpty(t, statuses)
		return statuses[len(statuses)-1]
	}

	type begun struct {
		start core.SessionStart
		err   error
	}
	done := make(chan begun, 1)
	go func() {
		start, err := sessions.Begin(g.ID, "")
		done <- begun{start, err}
	}()
	select {
	case p := <-prompts:
		require.Equal(t, "lock", p.Kind)
		require.True(t, p.BeforePlay || p.Lock != nil)
		require.True(t, sessions.Respond(p.ID, "yes"))
	case <-time.After(30 * time.Second):
		require.Fail(t, "no lock prompt")
	}
	var r begun
	select {
	case r = <-done:
	case <-time.After(30 * time.Second):
		require.Fail(t, "Begin never returned")
	}
	require.NoError(t, r.err)
	assert.Equal(t, syncer.ActionPush, r.start.PreAction)
	assert.Equal(t, "playing", last().Phase)
	assert.True(t, c.SessionOpen(g.ID))
	assert.False(t, sessions.IsRunning(g.ID), "Begin's bookkeeping outlived it")

	// While the session is open nothing else touches the game's saves.
	_, err = syncs.Sync(g.ID, "")
	require.ErrorIs(t, err, core.ErrPlaying)
	require.ErrorIs(t, syncs.BackupNow(g.ID), core.ErrPlaying)
	all, err := syncs.SyncAll()
	require.NoError(t, err)
	assert.Empty(t, all, "SyncAll synced a game being played")

	require.NoError(t, os.WriteFile(filepath.Join(saves, "b.sav"), []byte("played"), 0o644))
	notice, err := sessions.End(g.ID)
	require.NoError(t, err)
	require.NotNil(t, notice)
	assert.Equal(t, core.NoticeUploaded, notice.Kind)
	assert.Equal(t, Status{GameID: g.ID, Key: g.ID, Phase: "done", Message: "Game closed, saves synced"}, last())
	assert.False(t, c.SessionOpen(g.ID))
	assert.NoFileExists(t, lockFile, "cloud lock not released")

	a, err := syncs.Sync(g.ID, "")
	require.NoError(t, err, "sync after the session")
	assert.Equal(t, string(syncer.ActionNone), a)
}
