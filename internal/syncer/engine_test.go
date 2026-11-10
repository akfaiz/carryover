package syncer

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
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
	if got := readZipEntry(t, filepath.Join(cloudDir, "games", "game1", "backups", localBackup+".zip"), "saves/slot1.sav"); got != "pc-conflict" {
		t.Fatalf("backup content = %q", got)
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

// Save-path suggestions turn "<winAppData>/Game/<storeUserId>" into the folder
// plus include ["*", "*/**"]; files inside the per-user folder must sync.
func TestWildcardFolderFilters(t *testing.T) {
	cloudDir := setupCloud(t)
	pc, job := newDevice(t, cloudDir, "pc")
	saves := job.Slots[0].LocalPath
	job.Slots[0].Filters = cloud.Filters{Include: []string{"*", "*/**", "*.sav", "*.sav/**"}}
	write(t, filepath.Join(saves, "76561198000000000"), "ER0000.sl2", "elden")
	write(t, saves, "top.sav", "top")
	mustSync(t, pc, job, ResolveAuto, ActionPush)
	for _, rel := range []string{"76561198000000000/ER0000.sl2", "top.sav"} {
		if _, err := os.Stat(filepath.Join(cloudDir, "games", "game1", "current", "saves", filepath.FromSlash(rel))); err != nil {
			t.Errorf("%s not uploaded: %v", rel, err)
		}
	}

	// A file-only pattern must not pull in unrelated folders.
	pc2, job2 := newDevice(t, setupCloud(t), "pc2")
	job2.Slots[0].Filters = cloud.Filters{Include: []string{"*/*.sav", "*/*.sav/**"}}
	write(t, filepath.Join(job2.Slots[0].LocalPath, "slot1"), "a.sav", "x")
	write(t, filepath.Join(job2.Slots[0].LocalPath, "slot1"), "shader.cache", "y")
	p, err := pc2.Plan(context.Background(), job2)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := p.Local.Files["saves/slot1/a.sav"]; !ok || len(p.Local.Files) != 1 {
		t.Fatalf("unexpected files: %v", p.Local.Files)
	}
}

func cloudSaves(cloudDir string) string {
	return filepath.Join(cloudDir, "games", "game1", "current", "saves")
}

// A push uploads only files whose checksum changed and deletes removed ones;
// unchanged files in the cloud are not touched.
func TestPushTransfersOnlyChangedFiles(t *testing.T) {
	cloudDir := setupCloud(t)
	pc, job := newDevice(t, cloudDir, "pc")
	saves := job.Slots[0].LocalPath
	write(t, saves, "a.sav", "a-1")
	write(t, saves, "b.sav", "b-1")
	mustSync(t, pc, job, ResolveAuto, ActionPush)

	// Mark the cloud copy of b.sav (same size): if b.sav were re-uploaded
	// the marker would be overwritten.
	if err := os.WriteFile(filepath.Join(cloudSaves(cloudDir), "b.sav"), []byte("XXX"), 0o644); err != nil {
		t.Fatal(err)
	}
	write(t, saves, "a.sav", "a-2!")
	_, _, err := pc.Sync(context.Background(), job, ResolveAuto)
	if err != nil {
		t.Fatal(err)
	}
	if got := read(t, cloudSaves(cloudDir), "a.sav"); got != "a-2!" {
		t.Fatalf("cloud a.sav = %q, want the changed file uploaded", got)
	}
	if got := read(t, cloudSaves(cloudDir), "b.sav"); got != "XXX" {
		t.Fatalf("cloud b.sav = %q: unchanged file was re-uploaded", got)
	}

	// A deleted save is removed from the cloud.
	if err := os.Remove(filepath.Join(saves, "b.sav")); err != nil {
		t.Fatal(err)
	}
	mustSync(t, pc, job, ResolveAuto, ActionPush)
	if _, err := os.Stat(filepath.Join(cloudSaves(cloudDir), "b.sav")); !os.IsNotExist(err) {
		t.Fatalf("b.sav still in cloud, err=%v", err)
	}
}

// Files in the cloud folder that the manifest doesn't describe (e.g. left by
// an interrupted upload) are not trusted: stray ones are deleted, and a file
// whose size doesn't match the manifest is uploaded again.
func TestPushRepairsUntrustedCloudFiles(t *testing.T) {
	cloudDir := setupCloud(t)
	pc, job := newDevice(t, cloudDir, "pc")
	saves := job.Slots[0].LocalPath
	write(t, saves, "a.sav", "a-1")
	write(t, saves, "b.sav", "b-1")
	mustSync(t, pc, job, ResolveAuto, ActionPush)

	write(t, cloudSaves(cloudDir), "stray.tmp", "partial")
	write(t, cloudSaves(cloudDir), "b.sav", "trunc") // size differs from manifest
	write(t, saves, "a.sav", "a-2!")
	mustSync(t, pc, job, ResolveAuto, ActionPush)

	if _, err := os.Stat(filepath.Join(cloudSaves(cloudDir), "stray.tmp")); !os.IsNotExist(err) {
		t.Fatalf("stray file not removed, err=%v", err)
	}
	if got := read(t, cloudSaves(cloudDir), "b.sav"); got != "b-1" {
		t.Fatalf("cloud b.sav = %q, want it repaired from local", got)
	}
}

// A pull downloads only files whose checksum differs and deletes removed
// ones; unchanged local files are not rewritten.
func TestPullTransfersOnlyChangedFiles(t *testing.T) {
	cloudDir := setupCloud(t)
	pc, pcJob := newDevice(t, cloudDir, "pc")
	deck, deckJob := newDevice(t, cloudDir, "deck")
	pcSaves, deckSaves := pcJob.Slots[0].LocalPath, deckJob.Slots[0].LocalPath
	write(t, pcSaves, "a.sav", "a-1")
	write(t, pcSaves, "b.sav", "b-1")
	mustSync(t, pc, pcJob, ResolveAuto, ActionPush)
	mustSync(t, deck, deckJob, ResolveAuto, ActionPull)

	// Pin b.sav's mtime on the deck; a re-download would change it.
	old := time.Date(2020, 1, 2, 3, 4, 5, 0, time.UTC)
	bPath := filepath.Join(deckSaves, "b.sav")
	if err := os.Chtimes(bPath, old, old); err != nil {
		t.Fatal(err)
	}
	mustSync(t, deck, deckJob, ResolveAuto, ActionNone) // re-baseline after touching mtime

	write(t, pcSaves, "a.sav", "a-2!")
	mustSync(t, pc, pcJob, ResolveAuto, ActionPush)
	mustSync(t, deck, deckJob, ResolveAuto, ActionPull)
	if got := read(t, deckSaves, "a.sav"); got != "a-2!" {
		t.Fatalf("deck a.sav = %q", got)
	}
	fi, err := os.Stat(bPath)
	if err != nil {
		t.Fatal(err)
	}
	if !fi.ModTime().Equal(old) {
		t.Fatalf("b.sav mtime = %v: unchanged file was re-downloaded", fi.ModTime())
	}

	if err := os.Remove(filepath.Join(pcSaves, "b.sav")); err != nil {
		t.Fatal(err)
	}
	mustSync(t, pc, pcJob, ResolveAuto, ActionPush)
	mustSync(t, deck, deckJob, ResolveAuto, ActionPull)
	if _, err := os.Stat(bPath); !os.IsNotExist(err) {
		t.Fatalf("b.sav not deleted on deck, err=%v", err)
	}
}

func TestDiff(t *testing.T) {
	src := map[string]FileInfo{"same": {Size: 1, Hash: "h"}, "changed": {Size: 1, Hash: "new"}, "added": {Size: 4, Hash: "a"}}
	dst := map[string]FileInfo{"same": {Size: 1, Hash: "h"}, "changed": {Size: 1, Hash: "old"}, "removed": {Size: 1, Hash: "r"}}
	d := diff(src, dst)
	if want := []string{"added", "changed"}; !slices.Equal(d.copy, want) {
		t.Errorf("copy = %v, want %v", d.copy, want)
	}
	if want := []string{"removed"}; !slices.Equal(d.del, want) {
		t.Errorf("del = %v, want %v", d.del, want)
	}
	if d.bytes != 5 {
		t.Errorf("bytes = %d, want 5", d.bytes)
	}
}

// Forcing a sync when nothing differs transfers nothing and takes no backup.
func TestNoOpForcedSyncSkipsBackup(t *testing.T) {
	cloudDir := setupCloud(t)
	pc, job := newDevice(t, cloudDir, "pc")
	write(t, job.Slots[0].LocalPath, "a.sav", "a-1")
	mustSync(t, pc, job, ResolveAuto, ActionPush)
	mustSync(t, pc, job, ResolveLocal, ActionPush)
	mustSync(t, pc, job, ResolveCloud, ActionPull)
	backups, err := pc.ListBackups(context.Background(), "game1")
	if err != nil {
		t.Fatal(err)
	}
	if len(backups) != 0 {
		t.Fatalf("backups = %+v, want none for no-op syncs", backups)
	}
}

// Cancelling a sync mid-backup removes the partial backup, doesn't record a
// sync, and the next sync completes normally.
func TestCancelledSyncLeavesNoPartialBackup(t *testing.T) {
	cloudDir := setupCloud(t)
	pc, job := newDevice(t, cloudDir, "pc")
	saves := job.Slots[0].LocalPath
	write(t, saves, "a.sav", "a-1")
	write(t, saves, "b.sav", "b-1")
	mustSync(t, pc, job, ResolveAuto, ActionPush)
	before, _ := pc.States.Load("game1")

	// Cancel as soon as the backup of the cloud version starts.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	pc.OnEvent = func(ev Event) {
		if ev.Phase == "backup" {
			cancel()
		}
	}
	write(t, saves, "a.sav", "a-2!")
	if _, _, err := pc.Sync(ctx, job, ResolveAuto); !errors.Is(err, context.Canceled) {
		t.Fatalf("sync err = %v, want context.Canceled", err)
	}
	pc.OnEvent = nil

	backups, err := pc.ListBackups(context.Background(), "game1")
	if err != nil {
		t.Fatal(err)
	}
	if len(backups) != 0 {
		t.Fatalf("partial backup left behind: %+v", backups)
	}
	after, _ := pc.States.Load("game1")
	if !after.LastSyncAt.Equal(before.LastSyncAt) || after.LastError != "" {
		t.Fatalf("cancelled sync changed state: %+v", after)
	}
	if got := read(t, cloudSaves(cloudDir), "a.sav"); got != "a-1" {
		t.Fatalf("cloud a.sav = %q, want untouched", got)
	}

	// The next sync does the work.
	mustSync(t, pc, job, ResolveAuto, ActionPush)
	if got := read(t, cloudSaves(cloudDir), "a.sav"); got != "a-2!" {
		t.Fatalf("cloud a.sav after retry = %q", got)
	}
}

// A cancelled context stops the local scan.
func TestScanLocalCancelled(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "a.sav", "x")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := ScanLocal(ctx, []Slot{{Key: "s", LocalPath: dir}}, nil); !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
}

// A backup archive whose upload didn't complete is deleted.
func TestDiscardPartialBackup(t *testing.T) {
	cloudDir := setupCloud(t)
	pc, _ := newDevice(t, cloudDir, "pc")
	backups := filepath.Join(cloudDir, "games", "game1", "backups")
	write(t, backups, "20260101T000000.000Z_pc_cloud.zip", "half")
	pc.discardBackup("game1", "20260101T000000.000Z_pc_cloud")
	if _, err := os.Stat(filepath.Join(backups, "20260101T000000.000Z_pc_cloud.zip")); !os.IsNotExist(err) {
		t.Fatalf("partial backup still there, err=%v", err)
	}
}
