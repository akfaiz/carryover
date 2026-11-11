package services

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/rclone/rclone/fs/config"

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
	if err := os.WriteFile(filepath.Join(saves, "a.sav"), []byte("save"), 0o644); err != nil {
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

	bus := NewBus()
	NewSessionService(c, bus) // routes core sync events to the bus
	svc := NewSyncService(c, bus)
	cancelNext := true
	bus.OnSync(func(ev syncer.Event) {
		if cancelNext && ev.Phase == "scan" {
			cancelNext = false
			if !svc.Cancel(g.ID) {
				t.Error("Cancel found no running sync")
			}
		}
	})

	if _, err := svc.Sync(g.ID, ""); !errors.Is(err, ErrSyncCancelled) {
		t.Fatalf("Sync err = %v, want ErrSyncCancelled", err)
	}
	if s, _ := c.States.Load(g.ID); s.LastError != "" {
		t.Fatalf("cancel flagged as a sync error: %q", s.LastError)
	}
	if svc.Cancel(g.ID) {
		t.Fatal("Cancel reports a running sync after it ended")
	}
	if a, err := svc.Sync(g.ID, ""); err != nil || a != string(syncer.ActionPush) {
		t.Fatalf("next Sync = %q, %v; want push", a, err)
	}
}

// CheckLocalChanges flags a game whose saves changed after its last sync, and
// the flag clears once a sync uploads them.
func TestCheckLocalChanges(t *testing.T) {
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
	writeSave := func(content string, mtime time.Time) {
		t.Helper()
		p := filepath.Join(saves, "a.sav")
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(p, mtime, mtime); err != nil {
			t.Fatal(err)
		}
	}
	writeSave("save", time.Now().Add(-time.Hour))
	g, err := c.Library.Save(library.Game{
		Name:    "Game A",
		Slots:   []library.SaveSlot{{Key: "saves"}},
		Devices: map[string]library.DeviceConfig{c.DeviceID(): {Paths: map[string]string{"saves": saves}, Exe: "x"}},
	}, c.DeviceID())
	if err != nil {
		t.Fatal(err)
	}
	bus := NewBus()
	notified := 0
	bus.OnLibrary(func(string) { notified++ })
	lib := NewLibraryService(c, bus, nil)
	svc := NewSyncService(c, nil)
	flagged := func() bool {
		t.Helper()
		v, err := lib.Get(g.ID)
		if err != nil {
			t.Fatal(err)
		}
		return v.Status.LocalChanges
	}

	if _, err := svc.Sync(g.ID, ""); err != nil {
		t.Fatal(err)
	}
	lib.CheckLocalChanges()
	if flagged() || notified != 0 {
		t.Fatalf("flagged=%v notified=%d right after a sync", flagged(), notified)
	}

	writeSave("played outside", time.Now().Add(-time.Minute))
	lib.CheckLocalChanges()
	if !flagged() || notified != 1 {
		t.Fatalf("flagged=%v notified=%d after a local change", flagged(), notified)
	}
	lib.CheckLocalChanges() // unchanged result: no new notification
	if notified != 1 {
		t.Fatalf("notified=%d for an unchanged result", notified)
	}

	if a, err := svc.Sync(g.ID, ""); err != nil || a != string(syncer.ActionPush) {
		t.Fatalf("Sync = %q, %v; want push", a, err)
	}
	if flagged() {
		t.Fatal("still flagged after the sync uploaded the saves")
	}
}
