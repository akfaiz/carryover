package services

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/rclone/rclone/fs/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/akfaiz/carryover/internal/appdir"
	"github.com/akfaiz/carryover/internal/core"
	"github.com/akfaiz/carryover/internal/library"
	"github.com/akfaiz/carryover/internal/syncer"
)

// syncEnv is an isolated Carryover with a local "cloud" remote and one game.
type syncEnv struct {
	c     *core.Core
	svc   *SyncService
	id    string
	saves string
}

func newSyncEnv(t *testing.T) syncEnv {
	t.Helper()
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
	require.NoError(t, os.WriteFile(filepath.Join(saves, "a.sav"), []byte("one"), 0o644))
	g, err := c.Library.Save(library.Game{
		Name:    "Game A",
		Slots:   []library.SaveSlot{{Key: "saves"}},
		Devices: map[string]library.DeviceConfig{c.DeviceID(): {Paths: map[string]string{"saves": saves}, Exe: "x"}},
	}, c.DeviceID())
	require.NoError(t, err)
	return syncEnv{c: c, svc: NewSyncService(c, NewBus()), id: g.ID, saves: saves}
}

func TestPlanThenSyncAll(t *testing.T) {
	e := newSyncEnv(t)

	p, err := e.svc.Plan(e.id)
	require.NoError(t, err)
	require.Equal(t, string(syncer.ActionPush), p.Action, "Plan: want a push")
	require.False(t, p.HasRemote, "Plan: want no cloud copy")
	require.Equal(t, 1, p.Local.Files, "Plan: want 1 local file")
	_, err = e.svc.Plan("no-such-game")
	require.Error(t, err, "Plan of an unknown game")

	res, err := e.svc.SyncAll()
	require.NoError(t, err)
	require.Len(t, res, 1)
	require.Equal(t, e.id, res[0].GameID)
	require.Equal(t, string(syncer.ActionPush), res[0].Action)
	require.Empty(t, res[0].Error)

	// Everything is uploaded now: a second run has nothing to do.
	res, err = e.svc.SyncAll()
	require.NoError(t, err, "second SyncAll")
	require.Len(t, res, 1)
	require.Equal(t, string(syncer.ActionNone), res[0].Action)
}

func TestSyncAllNeedsCloud(t *testing.T) {
	t.Setenv(appdir.EnvHome, t.TempDir())
	c, err := core.New()
	require.NoError(t, err)
	_, err = NewSyncService(c, NewBus()).SyncAll()
	require.Error(t, err, "SyncAll without a remote")
}

func TestSettingsRoundTrip(t *testing.T) {
	e := newSyncEnv(t)
	st := e.svc.GetSettings()
	st.BackupRetention = 7
	got, err := e.svc.SaveSettings(st)
	require.NoError(t, err)
	require.Equal(t, 7, got.BackupRetention)
	require.Equal(t, 7, e.svc.GetSettings().BackupRetention)
}

// BackupNow stores a pinned backup; it can be unpinned, restored over changed
// saves and deleted.
func TestBackupLifecycle(t *testing.T) {
	e := newSyncEnv(t)
	_, err := e.svc.Sync(e.id, "")
	require.NoError(t, err)

	none, err := e.svc.Backups(e.id)
	require.NoError(t, err)
	require.NotNil(t, none, "want an empty, non-nil list")
	require.Empty(t, none)

	require.NoError(t, e.svc.BackupNow(e.id))
	backups, err := e.svc.Backups(e.id)
	require.NoError(t, err)
	require.Len(t, backups, 1)
	b := backups[0]
	require.True(t, b.Pinned, "manual backup is pinned")
	require.Equal(t, "manual", b.Kind)
	require.False(t, b.Local)

	require.NoError(t, e.svc.SetBackupPinned(e.id, b.Name, false))
	backups, _ = e.svc.Backups(e.id)
	require.False(t, backups[0].Pinned, "backup still pinned")

	// Change the saves, then restore: the old content comes back and is the
	// current cloud version.
	save := filepath.Join(e.saves, "a.sav")
	require.NoError(t, os.WriteFile(save, []byte("two, changed"), 0o644))
	require.NoError(t, e.svc.RestoreBackup(e.id, b.Name))
	got, _ := os.ReadFile(save)
	require.Equal(t, "one", string(got), "restored save")
	require.Error(t, e.svc.RestoreBackup(e.id, "../evil"), "RestoreBackup with a path in the name")

	require.NoError(t, e.svc.DeleteBackup(e.id, b.Name))
	for _, left := range mustBackups(t, e) {
		require.NotEqual(t, b.Name, left.Name)
	}
}

func mustBackups(t *testing.T, e syncEnv) []syncer.Backup {
	t.Helper()
	b, err := e.svc.Backups(e.id)
	require.NoError(t, err)
	return b
}

// A service call for a game that is already being synced is refused.
func TestBusyGameIsRefused(t *testing.T) {
	e := newSyncEnv(t)
	e.svc.busy.Store(e.id, struct{}{})
	_, err := e.svc.Sync(e.id, "")
	assert.Error(t, err, "Sync while busy")
	assert.Error(t, e.svc.BackupNow(e.id), "BackupNow while busy")
	assert.Error(t, e.svc.DeleteBackup(e.id, "x"), "DeleteBackup while busy")
	assert.Error(t, e.svc.RestoreBackup(e.id, "x"), "RestoreBackup while busy")
}

func TestRecoverSessionsService(t *testing.T) {
	e := newSyncEnv(t)
	require.NoError(t, e.svc.RecoverSessions())
}

func TestLibraryDeleteAndPathHelpers(t *testing.T) {
	// No cloud remote: Delete publishes the library in the background, which
	// must not still be writing when the temp dir is removed.
	lib, c, id := newLibEnv(t)

	dir := t.TempDir()
	require.NoError(t, lib.CheckFolder(dir))
	require.Error(t, lib.CheckFolder(filepath.Join(dir, "missing")), "CheckFolder(missing)")
	file := filepath.Join(dir, "f")
	require.NoError(t, os.WriteFile(file, nil, 0o644))
	require.Error(t, lib.CheckFolder(file), "CheckFolder(file)")

	home, _ := os.UserHomeDir()
	require.Equal(t, "{home}/Games/A", lib.ContractPath(filepath.Join(home, "Games", "A")))

	require.NoError(t, lib.Delete(id))
	_, err := c.Library.Get(id)
	require.Error(t, err, "game still in the library after Delete")
	// Deleting again is harmless.
	require.NoError(t, lib.Delete(id))
}
