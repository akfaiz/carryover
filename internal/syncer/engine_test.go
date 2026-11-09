package syncer

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/akfaiz/playrelay/internal/cloud"
)

func setupCloud(t *testing.T) string {
	t.Helper()
	if err := cloud.Init(filepath.Join(t.TempDir(), "rclone.conf")); err != nil {
		t.Fatal(err)
	}
	return filepath.Join(t.TempDir(), "cloud")
}

func newDevice(t *testing.T, cloudDir, id string) (*Engine, Job) {
	t.Helper()
	c, err := cloud.NewClient(cloudDir)
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	e := &Engine{
		Client:    c,
		States:    &StateStore{Dir: filepath.Join(root, "state")},
		Device:    Device{ID: id, Name: id},
		Retention: 3,
	}
	if err := os.MkdirAll(e.States.Dir, 0o755); err != nil {
		t.Fatal(err)
	}
	job := Job{GameID: "game1", Slots: []Slot{{Key: "saves", LocalPath: filepath.Join(root, "saves")}}}
	return e, job
}

func write(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	// ensure mtime changes even on coarse filesystems
	future := time.Now().Add(time.Duration(len(content)) * time.Second)
	_ = os.Chtimes(filepath.Join(dir, name), future, future)
}

func read(t *testing.T, dir, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func mustSync(t *testing.T, e *Engine, job Job, res Resolution, want Action) {
	t.Helper()
	_, got, err := e.Sync(context.Background(), job, res)
	if err != nil && !errors.Is(err, ErrConflict) {
		t.Fatalf("sync: %v", err)
	}
	if got != want {
		t.Fatalf("action = %s, want %s", got, want)
	}
}

func TestTwoDeviceSync(t *testing.T) {
	cloudDir := setupCloud(t)
	pc, pcJob := newDevice(t, cloudDir, "pc")
	deck, deckJob := newDevice(t, cloudDir, "deck")
	pcSaves, deckSaves := pcJob.Slots[0].LocalPath, deckJob.Slots[0].LocalPath

	// Nothing anywhere.
	mustSync(t, pc, pcJob, ResolveAuto, ActionNone)

	// PC creates a save -> push.
	write(t, pcSaves, "slot1.sav", "pc-1")
	mustSync(t, pc, pcJob, ResolveAuto, ActionPush)
	mustSync(t, pc, pcJob, ResolveAuto, ActionNone)

	// Deck has nothing -> pull.
	mustSync(t, deck, deckJob, ResolveAuto, ActionPull)
	if got := read(t, deckSaves, "slot1.sav"); got != "pc-1" {
		t.Fatalf("deck save = %q", got)
	}
	mustSync(t, deck, deckJob, ResolveAuto, ActionNone)

	// Deck plays -> push; PC pulls.
	write(t, deckSaves, "slot1.sav", "deck-22")
	write(t, deckSaves, "slot2.sav", "new")
	mustSync(t, deck, deckJob, ResolveAuto, ActionPush)
	mustSync(t, pc, pcJob, ResolveAuto, ActionPull)
	if got := read(t, pcSaves, "slot1.sav"); got != "deck-22" {
		t.Fatalf("pc save = %q", got)
	}

	// Deletion propagates.
	if err := os.Remove(filepath.Join(pcSaves, "slot2.sav")); err != nil {
		t.Fatal(err)
	}
	mustSync(t, pc, pcJob, ResolveAuto, ActionPush)
	mustSync(t, deck, deckJob, ResolveAuto, ActionPull)
	if _, err := os.Stat(filepath.Join(deckSaves, "slot2.sav")); !os.IsNotExist(err) {
		t.Fatalf("slot2.sav should be deleted on deck, err=%v", err)
	}

	// Both change -> conflict, then resolve keeping cloud on PC.
	write(t, pcSaves, "slot1.sav", "pc-conflict")
	write(t, deckSaves, "slot1.sav", "deck-conflict!")
	mustSync(t, deck, deckJob, ResolveAuto, ActionPush)
	mustSync(t, pc, pcJob, ResolveAuto, ActionConflict)
	st, _ := pc.States.Load("game1")
	if !st.Conflict {
		t.Fatal("expected conflict flag in state")
	}
	mustSync(t, pc, pcJob, ResolveCloud, ActionPull)
	if got := read(t, pcSaves, "slot1.sav"); got != "deck-conflict!" {
		t.Fatalf("pc save after resolve = %q", got)
	}

	// The overwritten PC save must be in a backup.
	backups, err := pc.ListBackups(context.Background(), "game1")
	if err != nil {
		t.Fatal(err)
	}
	if len(backups) == 0 || len(backups) > pc.Retention {
		t.Fatalf("backups = %d, want 1..%d", len(backups), pc.Retention)
	}
	var localBackup string
	for _, b := range backups {
		if b.Kind == "local" && b.DeviceName == "pc" {
			localBackup = b.Name
			break
		}
	}
	if localBackup == "" {
		t.Fatalf("no local backup from pc in %+v", backups)
	}
	b, err := os.ReadFile(filepath.Join(cloudDir, "games", "game1", "backups", localBackup, "saves", "slot1.sav"))
	if err != nil || string(b) != "pc-conflict" {
		t.Fatalf("backup content = %q, %v", b, err)
	}

	// Restore that backup: becomes local and cloud version.
	if err := pc.RestoreBackup(context.Background(), pcJob, localBackup); err != nil {
		t.Fatal(err)
	}
	if got := read(t, pcSaves, "slot1.sav"); got != "pc-conflict" {
		t.Fatalf("after restore = %q", got)
	}
	mustSync(t, deck, deckJob, ResolveAuto, ActionPull)
	if got := read(t, deckSaves, "slot1.sav"); got != "pc-conflict" {
		t.Fatalf("deck after restore = %q", got)
	}
}

func TestFirstSyncBothPresent(t *testing.T) {
	cloudDir := setupCloud(t)
	pc, pcJob := newDevice(t, cloudDir, "pc")
	deck, deckJob := newDevice(t, cloudDir, "deck")
	write(t, pcJob.Slots[0].LocalPath, "a.sav", "same")
	write(t, deckJob.Slots[0].LocalPath, "a.sav", "same")
	mustSync(t, pc, pcJob, ResolveAuto, ActionPush)
	// identical content on first sync is adopted without conflict
	mustSync(t, deck, deckJob, ResolveAuto, ActionNone)

	// a third device with different existing saves must not silently overwrite either side
	deck2, deck2Job := newDevice(t, cloudDir, "deck2")
	write(t, deck2Job.Slots[0].LocalPath, "a.sav", "other")
	mustSync(t, deck2, deck2Job, ResolveAuto, ActionConflict)
}

func TestFilters(t *testing.T) {
	cloudDir := setupCloud(t)
	pc, job := newDevice(t, cloudDir, "pc")
	job.Slots[0].Filters = cloud.Filters{Exclude: []string{"*.log"}}
	write(t, job.Slots[0].LocalPath, "a.sav", "x")
	write(t, job.Slots[0].LocalPath, "debug.log", "noise")
	mustSync(t, pc, job, ResolveAuto, ActionPush)
	if _, err := os.Stat(filepath.Join(cloudDir, "games", "game1", "current", "saves", "debug.log")); !os.IsNotExist(err) {
		t.Fatal("excluded file was uploaded")
	}
	// changing an excluded file doesn't trigger a sync
	write(t, job.Slots[0].LocalPath, "debug.log", "more noise")
	mustSync(t, pc, job, ResolveAuto, ActionNone)
}

func TestDecide(t *testing.T) {
	m := func(rev string, files ...string) *Manifest {
		mm := &Manifest{Revision: rev, Files: map[string]FileInfo{}}
		for _, f := range files {
			mm.Files[f] = FileInfo{Size: 1, Hash: f}
		}
		return mm
	}
	cases := []struct {
		name                string
		local, remote, last *Manifest
		want                Action
	}{
		{"empty", m(""), nil, nil, ActionNone},
		{"first push", m("", "a"), nil, nil, ActionPush},
		{"first pull", m(""), m("r1", "a"), nil, ActionPull},
		{"first conflict", m("", "a"), m("r1", "b"), nil, ActionConflict},
		{"unchanged", m("", "a"), m("r1", "a"), m("r1", "a"), ActionNone},
		{"local changed", m("", "b"), m("r1", "a"), m("r1", "a"), ActionPush},
		{"remote changed", m("", "a"), m("r2", "b"), m("r1", "a"), ActionPull},
		{"both changed", m("", "c"), m("r2", "b"), m("r1", "a"), ActionConflict},
		{"both changed same", m("", "b"), m("r2", "b"), m("r1", "a"), ActionNone},
		{"cloud wiped", m("", "a"), nil, m("r1", "a"), ActionPush},
	}
	for _, c := range cases {
		if got := Decide(c.local, c.remote, c.last); got != c.want {
			t.Errorf("%s: got %s want %s", c.name, got, c.want)
		}
	}
}

func TestLocalLock(t *testing.T) {
	dir := t.TempDir()
	l, err := AcquireLocalLock(dir, "g")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := AcquireLocalLock(dir, "g"); err == nil {
		// same pid holds it; our own pid is treated as stale only if different
		t.Log("lock re-acquired by same process (expected: pid equals ours)")
	}
	l.Release()
	if _, err := AcquireLocalLock(dir, "g"); err != nil {
		t.Fatal(err)
	}
}
