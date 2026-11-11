package syncer

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// LocalChanges compares the saves with the last sync, offline.
func TestLocalChanges(t *testing.T) {
	cloudDir := setupCloud(t)
	pc, job := newDevice(t, cloudDir, "pc")
	saves := job.Slots[0].LocalPath
	ctx := context.Background()
	last := func() *Manifest {
		st, err := pc.States.Load(job.GameID)
		if err != nil {
			t.Fatal(err)
		}
		return st.LastSynced
	}

	// Never synced here: nothing to report, even with saves present.
	write(t, saves, "a.sav", "a-1")
	if changed, _, err := LocalChanges(ctx, job.Slots, last(), nil); err != nil || changed {
		t.Fatalf("never synced: changed=%v err=%v", changed, err)
	}
	mustSync(t, pc, job, ResolveAuto, ActionPush)
	changed, scan, err := LocalChanges(ctx, job.Slots, last(), nil)
	if err != nil || changed {
		t.Fatalf("after sync: changed=%v err=%v", changed, err)
	}

	// Played without Carryover.
	write(t, saves, "a.sav", "a-22")
	if changed, scan, err = LocalChanges(ctx, job.Slots, last(), scan); err != nil || !changed {
		t.Fatalf("after a change: changed=%v err=%v", changed, err)
	}
	if changed, _, err = LocalChanges(ctx, job.Slots, last(), scan); err != nil || !changed {
		t.Fatalf("rechecked with the previous scan: changed=%v err=%v", changed, err)
	}
	mustSync(t, pc, job, ResolveAuto, ActionPush)
	if changed, _, err = LocalChanges(ctx, job.Slots, last(), scan); err != nil || changed {
		t.Fatalf("after pushing: changed=%v err=%v", changed, err)
	}

	// A deleted save is a change too.
	if err := os.Remove(filepath.Join(saves, "a.sav")); err != nil {
		t.Fatal(err)
	}
	if changed, _, err = LocalChanges(ctx, job.Slots, last(), nil); err != nil || !changed {
		t.Fatalf("after a delete: changed=%v err=%v", changed, err)
	}
}

// With cloud backups off, a download that replaces local saves first keeps a
// copy on this device, which is listed, restorable, deletable and pruned.
func TestPullKeepsSafetyCopy(t *testing.T) {
	cloudDir := setupCloud(t)
	pc, pcJob := newDevice(t, cloudDir, "pc")
	deck, deckJob := newDevice(t, cloudDir, "deck")
	pc.Retention, deck.Retention = 0, 0
	deck.SafetyDir = filepath.Join(t.TempDir(), "safety")
	pcSaves, deckSaves := pcJob.Slots[0].LocalPath, deckJob.Slots[0].LocalPath
	ctx := context.Background()

	// A first pull onto an empty device has nothing to keep.
	write(t, pcSaves, "a.sav", "pc-1")
	mustSync(t, pc, pcJob, ResolveAuto, ActionPush)
	mustSync(t, deck, deckJob, ResolveAuto, ActionPull)
	if names, _ := deck.safetyNames(deckJob.GameID); len(names) != 0 {
		t.Fatalf("safety copies after pulling into an empty folder: %v", names)
	}

	// Both changed; the deck keeps the cloud version.
	write(t, deckSaves, "a.sav", "deck-conflict")
	write(t, pcSaves, "a.sav", "pc-22")
	mustSync(t, pc, pcJob, ResolveAuto, ActionPush)
	mustSync(t, deck, deckJob, ResolveAuto, ActionConflict)
	mustSync(t, deck, deckJob, ResolveCloud, ActionPull)
	if got := read(t, deckSaves, "a.sav"); got != "pc-22" {
		t.Fatalf("deck save = %q", got)
	}

	backups, err := deck.ListBackups(ctx, deckJob.GameID)
	if err != nil {
		t.Fatal(err)
	}
	if len(backups) != 1 || !backups[0].Local || backups[0].Kind != kindSafety || backups[0].DeviceName != "deck" {
		t.Fatalf("backups = %+v, want one safety copy", backups)
	}
	name := backups[0].Name
	if got := readZipEntry(t, deck.safetyPath(deckJob.GameID, name), "saves/a.sav"); got != "deck-conflict" {
		t.Fatalf("a.sav in safety copy = %q", got)
	}
	// Only on this device: nothing was uploaded.
	if cloudList, _ := pc.ListBackups(ctx, pcJob.GameID); len(cloudList) != 0 {
		t.Fatalf("cloud backups = %+v, want none", cloudList)
	}
	if err := deck.SetBackupPinned(ctx, deckJob.GameID, name, true); err == nil {
		t.Fatal("pinning a safety copy should fail")
	}

	// Restoring it brings the overwritten save back and uploads it.
	if err := deck.RestoreBackup(ctx, deckJob, name); err != nil {
		t.Fatal(err)
	}
	if got := read(t, deckSaves, "a.sav"); got != "deck-conflict" {
		t.Fatalf("deck save after restore = %q", got)
	}
	mustSync(t, pc, pcJob, ResolveAuto, ActionPull)
	if got := read(t, pcSaves, "a.sav"); got != "deck-conflict" {
		t.Fatalf("pc save after the deck restored = %q", got)
	}

	if err := deck.DeleteBackup(ctx, deckJob.GameID, name); err != nil {
		t.Fatal(err)
	}
	if names, _ := deck.safetyNames(deckJob.GameID); len(names) != 0 {
		t.Fatalf("safety copies after delete: %v", names)
	}
}

func TestSafetyCopiesArePruned(t *testing.T) {
	cloudDir := setupCloud(t)
	e, job := newDevice(t, cloudDir, "pc")
	e.SafetyDir = filepath.Join(t.TempDir(), "safety")
	write(t, job.Slots[0].LocalPath, "a.sav", "a")
	local, err := ScanLocal(context.Background(), job.Slots, nil)
	if err != nil {
		t.Fatal(err)
	}
	for range SafetyKeep + 2 {
		if err := e.safetyCopy(job, local); err != nil {
			t.Fatal(err)
		}
		time.Sleep(2 * time.Millisecond) // names have ms precision
	}
	names, err := e.safetyNames(job.GameID)
	if err != nil || len(names) != SafetyKeep {
		t.Fatalf("safety copies = %d (%v), want %d", len(names), err, SafetyKeep)
	}
}
