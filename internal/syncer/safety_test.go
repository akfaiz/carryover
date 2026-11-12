package syncer

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// LocalChanges compares the saves with the last sync, offline.
func TestLocalChanges(t *testing.T) {
	cloudDir := setupCloud(t)
	pc, job := newDevice(t, cloudDir, "pc")
	saves := job.Slots[0].LocalPath
	ctx := context.Background()
	last := func() *Manifest {
		st, err := pc.States.Load(job.GameID)
		require.NoError(t, err)
		return st.LastSynced
	}

	// Never synced here: nothing to report, even with saves present.
	write(t, saves, "a.sav", "a-1")
	changed, _, err := LocalChanges(ctx, job.Slots, last(), nil)
	require.NoError(t, err)
	require.False(t, changed, "never synced")
	mustSync(t, pc, job, ResolveAuto, ActionPush)
	changed, scan, err := LocalChanges(ctx, job.Slots, last(), nil)
	require.NoError(t, err)
	require.False(t, changed, "after sync")

	// Played without Carryover.
	write(t, saves, "a.sav", "a-22")
	changed, scan, err = LocalChanges(ctx, job.Slots, last(), scan)
	require.NoError(t, err)
	require.True(t, changed, "after a change")
	changed, _, err = LocalChanges(ctx, job.Slots, last(), scan)
	require.NoError(t, err)
	require.True(t, changed, "rechecked with the previous scan")
	mustSync(t, pc, job, ResolveAuto, ActionPush)
	changed, _, err = LocalChanges(ctx, job.Slots, last(), scan)
	require.NoError(t, err)
	require.False(t, changed, "after pushing")

	// A deleted save is a change too.
	require.NoError(t, os.Remove(filepath.Join(saves, "a.sav")))
	changed, _, err = LocalChanges(ctx, job.Slots, last(), nil)
	require.NoError(t, err)
	require.True(t, changed, "after a delete")
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
	names, _ := deck.safetyNames(deckJob.GameID)
	require.Empty(t, names, "safety copies after pulling into an empty folder")

	// Both changed; the deck keeps the cloud version.
	write(t, deckSaves, "a.sav", "deck-conflict")
	write(t, pcSaves, "a.sav", "pc-22")
	mustSync(t, pc, pcJob, ResolveAuto, ActionPush)
	mustSync(t, deck, deckJob, ResolveAuto, ActionConflict)
	mustSync(t, deck, deckJob, ResolveCloud, ActionPull)
	require.Equal(t, "pc-22", read(t, deckSaves, "a.sav"))

	backups, err := deck.ListBackups(ctx, deckJob.GameID)
	require.NoError(t, err)
	require.Len(t, backups, 1, "want one safety copy")
	require.True(t, backups[0].Local)
	require.Equal(t, kindSafety, backups[0].Kind)
	require.Equal(t, "deck", backups[0].DeviceName)
	name := backups[0].Name
	require.Equal(t, "deck-conflict", readZipEntry(t, deck.safetyPath(deckJob.GameID, name), "saves/a.sav"))
	// Only on this device: nothing was uploaded.
	cloudList, _ := pc.ListBackups(ctx, pcJob.GameID)
	require.Empty(t, cloudList, "cloud backups")
	require.Error(t, deck.SetBackupPinned(ctx, deckJob.GameID, name, true), "pinning a safety copy should fail")

	// Restoring it brings the overwritten save back and uploads it.
	require.NoError(t, deck.RestoreBackup(ctx, deckJob, name))
	require.Equal(t, "deck-conflict", read(t, deckSaves, "a.sav"))
	mustSync(t, pc, pcJob, ResolveAuto, ActionPull)
	require.Equal(t, "deck-conflict", read(t, pcSaves, "a.sav"))

	require.NoError(t, deck.DeleteBackup(ctx, deckJob.GameID, name))
	names, _ = deck.safetyNames(deckJob.GameID)
	require.Empty(t, names, "safety copies after delete")
}

func TestSafetyCopiesArePruned(t *testing.T) {
	cloudDir := setupCloud(t)
	e, job := newDevice(t, cloudDir, "pc")
	e.SafetyDir = filepath.Join(t.TempDir(), "safety")
	write(t, job.Slots[0].LocalPath, "a.sav", "a")
	local, err := ScanLocal(context.Background(), job.Slots, nil)
	require.NoError(t, err)
	for range SafetyKeep + 2 {
		require.NoError(t, e.safetyCopy(job, local))
		time.Sleep(2 * time.Millisecond) // names have ms precision
	}
	names, err := e.safetyNames(job.GameID)
	require.NoError(t, err)
	require.Len(t, names, SafetyKeep)
}
