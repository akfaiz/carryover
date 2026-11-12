package services

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/rclone/rclone/fs/config"

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
	if err != nil {
		t.Fatal(err)
	}
	config.FileSetValue("cloud", "type", "local")
	st := c.Settings.Get()
	st.Remote, st.RemoteRoot = "cloud", filepath.ToSlash(filepath.Join(home, "cloud"))
	if err := c.Settings.Update(st); err != nil {
		t.Fatal(err)
	}
	saves := filepath.Join(home, "saves")
	if err := os.MkdirAll(saves, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(saves, "a.sav"), []byte("one"), 0o644); err != nil {
		t.Fatal(err)
	}
	g, err := c.Library.Save(library.Game{
		Name:    "Game A",
		Slots:   []library.SaveSlot{{Key: "saves"}},
		Devices: map[string]library.DeviceConfig{c.DeviceID(): {Paths: map[string]string{"saves": saves}, Exe: "x"}},
	}, c.DeviceID())
	if err != nil {
		t.Fatal(err)
	}
	return syncEnv{c: c, svc: NewSyncService(c, NewBus()), id: g.ID, saves: saves}
}

func TestPlanThenSyncAll(t *testing.T) {
	e := newSyncEnv(t)

	p, err := e.svc.Plan(e.id)
	if err != nil {
		t.Fatal(err)
	}
	if p.Action != string(syncer.ActionPush) || p.HasRemote || p.Local.Files != 1 {
		t.Fatalf("Plan = %+v, want a push of 1 local file and no cloud copy", p)
	}
	if _, err := e.svc.Plan("no-such-game"); err == nil {
		t.Fatal("Plan of an unknown game: want an error")
	}

	res, err := e.svc.SyncAll()
	if err != nil || len(res) != 1 {
		t.Fatalf("SyncAll = %+v, %v", res, err)
	}
	if res[0].GameID != e.id || res[0].Action != string(syncer.ActionPush) || res[0].Error != "" {
		t.Fatalf("SyncAll result = %+v", res[0])
	}

	// Everything is uploaded now: a second run has nothing to do.
	res, err = e.svc.SyncAll()
	if err != nil || len(res) != 1 || res[0].Action != string(syncer.ActionNone) {
		t.Fatalf("second SyncAll = %+v, %v", res, err)
	}
}

func TestSyncAllNeedsCloud(t *testing.T) {
	t.Setenv(appdir.EnvHome, t.TempDir())
	c, err := core.New()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := NewSyncService(c, NewBus()).SyncAll(); err == nil {
		t.Fatal("SyncAll without a remote: want an error")
	}
}

func TestSettingsRoundTrip(t *testing.T) {
	e := newSyncEnv(t)
	st := e.svc.GetSettings()
	st.BackupRetention = 7
	got, err := e.svc.SaveSettings(st)
	if err != nil || got.BackupRetention != 7 || e.svc.GetSettings().BackupRetention != 7 {
		t.Fatalf("SaveSettings = %+v, %v", got, err)
	}
}

// BackupNow stores a pinned backup; it can be unpinned, restored over changed
// saves and deleted.
func TestBackupLifecycle(t *testing.T) {
	e := newSyncEnv(t)
	if _, err := e.svc.Sync(e.id, ""); err != nil {
		t.Fatal(err)
	}

	if b, err := e.svc.Backups(e.id); err != nil || b == nil || len(b) != 0 {
		t.Fatalf("Backups before any = %v, %v; want an empty, non-nil list", b, err)
	}

	if err := e.svc.BackupNow(e.id); err != nil {
		t.Fatal(err)
	}
	backups, err := e.svc.Backups(e.id)
	if err != nil || len(backups) != 1 {
		t.Fatalf("Backups = %+v, %v; want one", backups, err)
	}
	b := backups[0]
	if !b.Pinned || b.Kind != "manual" || b.Local {
		t.Fatalf("manual backup = %+v", b)
	}

	if err := e.svc.SetBackupPinned(e.id, b.Name, false); err != nil {
		t.Fatal(err)
	}
	if backups, _ = e.svc.Backups(e.id); backups[0].Pinned {
		t.Fatal("backup still pinned")
	}

	// Change the saves, then restore: the old content comes back and is the
	// current cloud version.
	save := filepath.Join(e.saves, "a.sav")
	if err := os.WriteFile(save, []byte("two, changed"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := e.svc.RestoreBackup(e.id, b.Name); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(save); string(got) != "one" {
		t.Fatalf("restored save = %q, want %q", got, "one")
	}
	if err := e.svc.RestoreBackup(e.id, "../evil"); err == nil {
		t.Fatal("RestoreBackup with a path in the name: want an error")
	}

	if err := e.svc.DeleteBackup(e.id, b.Name); err != nil {
		t.Fatal(err)
	}
	for _, left := range mustBackups(t, e) {
		if left.Name == b.Name {
			t.Fatalf("backup %s not deleted", b.Name)
		}
	}
}

func mustBackups(t *testing.T, e syncEnv) []syncer.Backup {
	t.Helper()
	b, err := e.svc.Backups(e.id)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// A service call for a game that is already being synced is refused.
func TestBusyGameIsRefused(t *testing.T) {
	e := newSyncEnv(t)
	e.svc.busy.Store(e.id, struct{}{})
	if _, err := e.svc.Sync(e.id, ""); err == nil {
		t.Error("Sync while busy: want an error")
	}
	if err := e.svc.BackupNow(e.id); err == nil {
		t.Error("BackupNow while busy: want an error")
	}
	if err := e.svc.DeleteBackup(e.id, "x"); err == nil {
		t.Error("DeleteBackup while busy: want an error")
	}
	if err := e.svc.RestoreBackup(e.id, "x"); err == nil {
		t.Error("RestoreBackup while busy: want an error")
	}
}

func TestRecoverSessionsService(t *testing.T) {
	e := newSyncEnv(t)
	if err := e.svc.RecoverSessions(); err != nil {
		t.Fatal(err)
	}
}

func TestLibraryDeleteAndPathHelpers(t *testing.T) {
	e := newSyncEnv(t)
	lib := NewLibraryService(e.c, NewBus(), nil)

	dir := t.TempDir()
	if err := lib.CheckFolder(dir); err != nil {
		t.Fatalf("CheckFolder(existing) = %v", err)
	}
	if err := lib.CheckFolder(filepath.Join(dir, "missing")); err == nil {
		t.Fatal("CheckFolder(missing): want an error")
	}
	file := filepath.Join(dir, "f")
	if err := os.WriteFile(file, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := lib.CheckFolder(file); err == nil {
		t.Fatal("CheckFolder(file): want an error")
	}

	home, _ := os.UserHomeDir()
	if got := lib.ContractPath(filepath.Join(home, "Games", "A")); got != "{home}/Games/A" {
		t.Fatalf("ContractPath = %q", got)
	}

	if err := lib.Delete(e.id); err != nil {
		t.Fatal(err)
	}
	if _, err := e.c.Library.Get(e.id); err == nil {
		t.Fatal("game still in the library after Delete")
	}
	// Deleting again is harmless.
	if err := lib.Delete(e.id); err != nil {
		t.Fatalf("second Delete = %v", err)
	}
}
