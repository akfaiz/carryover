package services

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/rclone/rclone/fs/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/akfaiz/carryover/internal/appdir"
	"github.com/akfaiz/carryover/internal/core"
	"github.com/akfaiz/carryover/internal/library"
	"github.com/akfaiz/carryover/internal/syncer"
)

// Cancel stops a running manual sync: it reports ErrSyncCancelled, flags no
// sync error, and a later sync works.
func TestSyncCancel(t *testing.T) {
	home := t.TempDir()
	t.Setenv(appdir.EnvHome, home)
	c, err := core.New()
	require.NoError(t, err)
	config.FileSetValue("cloud", "type", "local")
	st := c.Settings.Get()
	st.Remote, st.RemoteRoot = "cloud", filepath.ToSlash(filepath.Join(home, "cloud"))
	require.NoError(t, c.Settings.Update(st))
	saves := filepath.Join(home, "saves")
	require.NoError(t, os.MkdirAll(saves, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(saves, "a.sav"), []byte("save"), 0o644))
	g, err := c.Library.Save(library.Game{
		Name:    "Game A",
		Slots:   []library.SaveSlot{{Key: "saves"}},
		Devices: map[string]library.DeviceConfig{c.DeviceID(): {Paths: map[string]string{"saves": saves}, Exe: "x"}},
	}, c.DeviceID())
	require.NoError(t, err)

	bus := NewBus()
	NewSessionService(c, bus) // routes core sync events to the bus
	svc := NewSyncService(c, bus)
	cancelNext := true
	bus.OnSync(func(ev syncer.Event) {
		if cancelNext && ev.Phase == "scan" {
			cancelNext = false
			assert.True(t, svc.Cancel(g.ID), "Cancel found no running sync")
		}
	})

	_, err = svc.Sync(g.ID, "")
	require.ErrorIs(t, err, ErrSyncCancelled)
	s, _ := c.States.Load(g.ID)
	require.Empty(t, s.LastError, "cancel flagged as a sync error")
	require.False(t, svc.Cancel(g.ID), "Cancel reports a running sync after it ended")
	a, err := svc.Sync(g.ID, "")
	require.NoError(t, err, "next Sync")
	require.Equal(t, string(syncer.ActionPush), a)
}

// CheckLocalChanges flags a game whose saves changed after its last sync, and
// the flag clears once a sync uploads them.
func TestCheckLocalChanges(t *testing.T) {
	home := t.TempDir()
	t.Setenv(appdir.EnvHome, home)
	c, err := core.New()
	require.NoError(t, err)
	config.FileSetValue("cloud", "type", "local")
	st := c.Settings.Get()
	st.Remote, st.RemoteRoot = "cloud", filepath.ToSlash(filepath.Join(home, "cloud"))
	require.NoError(t, c.Settings.Update(st))
	saves := filepath.Join(home, "saves")
	require.NoError(t, os.MkdirAll(saves, 0o755))
	writeSave := func(content string, mtime time.Time) {
		t.Helper()
		p := filepath.Join(saves, "a.sav")
		require.NoError(t, os.WriteFile(p, []byte(content), 0o644))
		require.NoError(t, os.Chtimes(p, mtime, mtime))
	}
	writeSave("save", time.Now().Add(-time.Hour))
	g, err := c.Library.Save(library.Game{
		Name:    "Game A",
		Slots:   []library.SaveSlot{{Key: "saves"}},
		Devices: map[string]library.DeviceConfig{c.DeviceID(): {Paths: map[string]string{"saves": saves}, Exe: "x"}},
	}, c.DeviceID())
	require.NoError(t, err)
	bus := NewBus()
	notified := 0
	bus.OnLibrary(func(string) { notified++ })
	lib := NewLibraryService(c, bus, nil)
	svc := NewSyncService(c, nil)
	flagged := func() bool {
		t.Helper()
		v, err := lib.Get(g.ID)
		require.NoError(t, err)
		return v.Status.LocalChanges
	}

	_, err = svc.Sync(g.ID, "")
	require.NoError(t, err)
	lib.CheckLocalChanges()
	require.False(t, flagged(), "right after a sync")
	require.Zero(t, notified, "right after a sync")

	writeSave("played outside", time.Now().Add(-time.Minute))
	lib.CheckLocalChanges()
	require.True(t, flagged(), "after a local change")
	require.Equal(t, 1, notified, "after a local change")
	lib.CheckLocalChanges() // unchanged result: no new notification
	require.Equal(t, 1, notified)

	a, err := svc.Sync(g.ID, "")
	require.NoError(t, err)
	require.Equal(t, string(syncer.ActionPush), a)
	require.False(t, flagged(), "still flagged after the sync uploaded the saves")
}
