package syncer

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"testing"

	_ "github.com/rclone/rclone/backend/memory" // a non-local remote for tests
	"github.com/rclone/rclone/fs/config"
	"github.com/stretchr/testify/require"
)

// The cloud client opens a named remote at its root and addresses files
// relative to it. The local-folder tests don't exercise that, so run a full
// round trip against rclone's in-memory backend at "mem:Carryover".
func TestSyncAgainstNamedRemote(t *testing.T) {
	setupCloud(t) // initialises rclone's config
	config.FileSetValue("mem", "type", "memory")
	base := "mem:Carryover"
	pc, pcJob := newDevice(t, base, "pc")
	deck, deckJob := newDevice(t, base, "deck")
	pcSaves, deckSaves := pcJob.Slots[0].LocalPath, deckJob.Slots[0].LocalPath
	ctx := context.Background()

	write(t, pcSaves, "a.sav", "a-1")
	write(t, filepath.Join(pcSaves, "sub"), "b.sav", "b-1")
	mustSync(t, pc, pcJob, ResolveAuto, ActionPush)
	mustSync(t, deck, deckJob, ResolveAuto, ActionPull)
	require.Equal(t, "b-1", read(t, filepath.Join(deckSaves, "sub"), "b.sav"))

	// Change one file and delete another on the PC; the deck follows.
	write(t, pcSaves, "a.sav", "a-2!")
	require.NoError(t, os.Remove(filepath.Join(pcSaves, "sub", "b.sav")))
	mustSync(t, pc, pcJob, ResolveAuto, ActionPush)
	listed, err := pc.Client.ListFiles(ctx, currentDir("game1")+"/saves")
	require.NoError(t, err)
	require.Len(t, listed, 1, "cloud files, want only a.sav")
	require.EqualValues(t, 4, listed["a.sav"])
	mustSync(t, deck, deckJob, ResolveAuto, ActionPull)
	require.Equal(t, "a-2!", read(t, deckSaves, "a.sav"))
	require.NoFileExists(t, filepath.Join(deckSaves, "sub", "b.sav"), "deck sub/b.sav not deleted")

	// Conflict, resolved by keeping the cloud: the PC's version is backed up
	// (server-side copy within the remote) and can be restored.
	write(t, pcSaves, "a.sav", "pc-conflict")
	write(t, deckSaves, "a.sav", "deck-conflict!")
	mustSync(t, deck, deckJob, ResolveAuto, ActionPush)
	mustSync(t, pc, pcJob, ResolveAuto, ActionConflict)
	mustSync(t, pc, pcJob, ResolveCloud, ActionPull)
	backups, err := pc.ListBackups(ctx, "game1")
	require.NoError(t, err)
	var local, cloudKind string
	for _, b := range backups {
		switch {
		case b.Kind == "local" && b.DeviceName == "pc":
			local = b.Name
		case b.Kind == "cloud":
			cloudKind = b.Name
		}
	}
	require.NotEmpty(t, local, "want a local backup, got %+v", backups)
	require.NotEmpty(t, cloudKind, "want a cloud backup, got %+v", backups)
	_, files, err := pc.Client.List(ctx, backupsDir("game1"))
	require.NoError(t, err)
	require.True(t, slices.Contains(files, cloudKind+".zip"))
	require.NoError(t, pc.RestoreBackup(ctx, pcJob, local))
	require.Equal(t, "pc-conflict", read(t, pcSaves, "a.sav"))

	// Locks round-trip through the remote too.
	require.NoError(t, deck.AcquireLock(ctx, "game1"))
	l, err := pc.CheckLock(ctx, "game1")
	require.NoError(t, err)
	require.NotNil(t, l)
	require.Equal(t, "deck", l.DeviceName)
	require.NoError(t, deck.ReleaseLock(ctx, "game1"))
	l, err = pc.CheckLock(ctx, "game1")
	require.NoError(t, err)
	require.Nil(t, l, "lock after release")
}

// A remote of type local takes a filesystem path; it must not be treated as
// a path relative to the remote's root.
func TestLocalTypeRemoteWithAbsolutePath(t *testing.T) {
	dir := setupCloud(t)
	config.FileSetValue("disk", "type", "local")
	pc, job := newDevice(t, "disk:"+filepath.ToSlash(dir), "pc")
	write(t, job.Slots[0].LocalPath, "a.sav", "x")
	mustSync(t, pc, job, ResolveAuto, ActionPush)
	require.Equal(t, "x", read(t, filepath.Join(dir, "games", "game1", "current", "saves"), "a.sav"))
}
