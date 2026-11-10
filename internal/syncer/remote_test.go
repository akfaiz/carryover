package syncer

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"testing"

	_ "github.com/rclone/rclone/backend/memory" // a non-local remote for tests
	"github.com/rclone/rclone/fs/config"
)

// The cloud client opens a named remote at its root and addresses files
// relative to it. The local-folder tests don't exercise that, so run a full
// round trip against rclone's in-memory backend at "mem:PlayRelay".
func TestSyncAgainstNamedRemote(t *testing.T) {
	setupCloud(t) // initialises rclone's config
	config.FileSetValue("mem", "type", "memory")
	base := "mem:PlayRelay"
	pc, pcJob := newDevice(t, base, "pc")
	deck, deckJob := newDevice(t, base, "deck")
	pcSaves, deckSaves := pcJob.Slots[0].LocalPath, deckJob.Slots[0].LocalPath
	ctx := context.Background()

	write(t, pcSaves, "a.sav", "a-1")
	write(t, filepath.Join(pcSaves, "sub"), "b.sav", "b-1")
	mustSync(t, pc, pcJob, ResolveAuto, ActionPush)
	mustSync(t, deck, deckJob, ResolveAuto, ActionPull)
	if got := read(t, filepath.Join(deckSaves, "sub"), "b.sav"); got != "b-1" {
		t.Fatalf("deck sub/b.sav = %q", got)
	}

	// Change one file and delete another on the PC; the deck follows.
	write(t, pcSaves, "a.sav", "a-2!")
	if err := os.Remove(filepath.Join(pcSaves, "sub", "b.sav")); err != nil {
		t.Fatal(err)
	}
	mustSync(t, pc, pcJob, ResolveAuto, ActionPush)
	listed, err := pc.Client.ListFiles(ctx, currentDir("game1")+"/saves")
	if err != nil {
		t.Fatal(err)
	}
	if len(listed) != 1 || listed["a.sav"] != 4 {
		t.Fatalf("cloud files = %v, want only a.sav (4 bytes)", listed)
	}
	mustSync(t, deck, deckJob, ResolveAuto, ActionPull)
	if got := read(t, deckSaves, "a.sav"); got != "a-2!" {
		t.Fatalf("deck a.sav = %q", got)
	}
	if _, err := os.Stat(filepath.Join(deckSaves, "sub", "b.sav")); !os.IsNotExist(err) {
		t.Fatalf("deck sub/b.sav not deleted, err=%v", err)
	}

	// Conflict, resolved by keeping the cloud: the PC's version is backed up
	// (server-side copy within the remote) and can be restored.
	write(t, pcSaves, "a.sav", "pc-conflict")
	write(t, deckSaves, "a.sav", "deck-conflict!")
	mustSync(t, deck, deckJob, ResolveAuto, ActionPush)
	mustSync(t, pc, pcJob, ResolveAuto, ActionConflict)
	mustSync(t, pc, pcJob, ResolveCloud, ActionPull)
	backups, err := pc.ListBackups(ctx, "game1")
	if err != nil {
		t.Fatal(err)
	}
	var local, cloudKind string
	for _, b := range backups {
		switch {
		case b.Kind == "local" && b.DeviceName == "pc":
			local = b.Name
		case b.Kind == "cloud":
			cloudKind = b.Name
		}
	}
	if local == "" || cloudKind == "" {
		t.Fatalf("want a local and a cloud backup, got %+v", backups)
	}
	_, files, err := pc.Client.List(ctx, backupsDir("game1"))
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(files, cloudKind+".zip") {
		t.Fatalf("backup files = %v, want %s.zip", files, cloudKind)
	}
	if err := pc.RestoreBackup(ctx, pcJob, local); err != nil {
		t.Fatal(err)
	}
	if got := read(t, pcSaves, "a.sav"); got != "pc-conflict" {
		t.Fatalf("after restore = %q", got)
	}

	// Locks round-trip through the remote too.
	if err := deck.AcquireLock(ctx, "game1"); err != nil {
		t.Fatal(err)
	}
	if l, err := pc.CheckLock(ctx, "game1"); err != nil || l == nil || l.DeviceName != "deck" {
		t.Fatalf("CheckLock = %+v, %v", l, err)
	}
	if err := deck.ReleaseLock(ctx, "game1"); err != nil {
		t.Fatal(err)
	}
	if l, err := pc.CheckLock(ctx, "game1"); err != nil || l != nil {
		t.Fatalf("after release CheckLock = %+v, %v", l, err)
	}
}

// A remote of type local takes a filesystem path; it must not be treated as
// a path relative to the remote's root.
func TestLocalTypeRemoteWithAbsolutePath(t *testing.T) {
	dir := setupCloud(t)
	config.FileSetValue("disk", "type", "local")
	pc, job := newDevice(t, "disk:"+filepath.ToSlash(dir), "pc")
	write(t, job.Slots[0].LocalPath, "a.sav", "x")
	mustSync(t, pc, job, ResolveAuto, ActionPush)
	if got := read(t, filepath.Join(dir, "games", "game1", "current", "saves"), "a.sav"); got != "x" {
		t.Fatalf("cloud a.sav = %q", got)
	}
}
