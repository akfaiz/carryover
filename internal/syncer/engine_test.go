package syncer

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/akfaiz/carryover/internal/cloud"
)

func setupCloud(t *testing.T) string {
	t.Helper()
	require.NoError(t, cloud.Init(filepath.Join(t.TempDir(), "rclone.conf")))
	return filepath.Join(t.TempDir(), "cloud")
}

func newDevice(t *testing.T, cloudDir, id string) (*Engine, Job) {
	t.Helper()
	c, err := cloud.NewClient(cloudDir)
	require.NoError(t, err)
	root := t.TempDir()
	e := &Engine{
		Client:    c,
		States:    &StateStore{Dir: filepath.Join(root, "state")},
		Device:    Device{ID: id, Name: id},
		Retention: 3,
	}
	require.NoError(t, os.MkdirAll(e.States.Dir, 0o755))
	job := Job{GameID: "game1", Slots: []Slot{{Key: "saves", LocalPath: filepath.Join(root, "saves")}}}
	return e, job
}

func write(t *testing.T, dir, name, content string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(dir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644))
	// ensure mtime changes even on coarse filesystems
	future := time.Now().Add(time.Duration(len(content)) * time.Second)
	_ = os.Chtimes(filepath.Join(dir, name), future, future)
}

func read(t *testing.T, dir, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(dir, name))
	require.NoError(t, err)
	return string(b)
}

func mustSync(t *testing.T, e *Engine, job Job, res Resolution, want Action) {
	t.Helper()
	_, got, err := e.Sync(context.Background(), job, res)
	if !errors.Is(err, ErrConflict) {
		require.NoError(t, err, "sync")
	}
	require.Equal(t, want, got)
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
	require.Equal(t, "pc-1", read(t, deckSaves, "slot1.sav"))
	mustSync(t, deck, deckJob, ResolveAuto, ActionNone)

	// Deck plays -> push; PC pulls.
	write(t, deckSaves, "slot1.sav", "deck-22")
	write(t, deckSaves, "slot2.sav", "new")
	mustSync(t, deck, deckJob, ResolveAuto, ActionPush)
	mustSync(t, pc, pcJob, ResolveAuto, ActionPull)
	require.Equal(t, "deck-22", read(t, pcSaves, "slot1.sav"))

	// Deletion propagates.
	require.NoError(t, os.Remove(filepath.Join(pcSaves, "slot2.sav")))
	mustSync(t, pc, pcJob, ResolveAuto, ActionPush)
	mustSync(t, deck, deckJob, ResolveAuto, ActionPull)
	require.NoFileExists(t, filepath.Join(deckSaves, "slot2.sav"), "slot2.sav should be deleted on deck")

	// Both change -> conflict, then resolve keeping cloud on PC.
	write(t, pcSaves, "slot1.sav", "pc-conflict")
	write(t, deckSaves, "slot1.sav", "deck-conflict!")
	mustSync(t, deck, deckJob, ResolveAuto, ActionPush)
	mustSync(t, pc, pcJob, ResolveAuto, ActionConflict)
	st, _ := pc.States.Load("game1")
	require.True(t, st.Conflict, "expected conflict flag in state")
	mustSync(t, pc, pcJob, ResolveCloud, ActionPull)
	require.Equal(t, "deck-conflict!", read(t, pcSaves, "slot1.sav"))

	// The overwritten PC save must be in a backup.
	backups, err := pc.ListBackups(context.Background(), "game1")
	require.NoError(t, err)
	require.NotEmpty(t, backups)
	require.LessOrEqual(t, len(backups), pc.Retention)
	var localBackup string
	for _, b := range backups {
		if b.Kind == "local" && b.DeviceName == "pc" {
			localBackup = b.Name
			break
		}
	}
	require.NotEqual(t, "", localBackup)
	require.Equal(t, "pc-conflict", readZipEntry(t, filepath.Join(cloudDir, "games", "game1", "backups", localBackup+".zip"), "saves/slot1.sav"))

	// Restore that backup: becomes local and cloud version.
	require.NoError(t, pc.RestoreBackup(context.Background(), pcJob, localBackup))
	require.Equal(t, "pc-conflict", read(t, pcSaves, "slot1.sav"))
	mustSync(t, deck, deckJob, ResolveAuto, ActionPull)
	require.Equal(t, "pc-conflict", read(t, deckSaves, "slot1.sav"))
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
	require.NoFileExists(t, filepath.Join(cloudDir, "games", "game1", "current", "saves", "debug.log"), "excluded file was uploaded")
	// changing an excluded file doesn't trigger a sync
	write(t, job.Slots[0].LocalPath, "debug.log", "more noise")
	mustSync(t, pc, job, ResolveAuto, ActionNone)
}

// A slot with both an include and an exclude list keeps what the include list
// matches except what the exclude list matches (regression: the excludes were
// ignored for files the include list also matched).
func TestIncludeAndExcludeFilters(t *testing.T) {
	cloudDir := setupCloud(t)
	pc, job := newDevice(t, cloudDir, "pc")
	job.Slots[0].Filters = cloud.Filters{Include: []string{"*"}, Exclude: []string{"*.log", "cache/"}}
	saves := job.Slots[0].LocalPath
	write(t, saves, "a.sav", "x")
	write(t, saves, "debug.log", "noise")
	write(t, filepath.Join(saves, "cache"), "tmp.bin", "tmp")
	mustSync(t, pc, job, ResolveAuto, ActionPush)

	cloudSaves := filepath.Join(cloudDir, "games", "game1", "current", "saves")
	require.FileExists(t, filepath.Join(cloudSaves, "a.sav"))
	require.NoFileExists(t, filepath.Join(cloudSaves, "debug.log"), "excluded file was uploaded")
	require.NoFileExists(t, filepath.Join(cloudSaves, "cache", "tmp.bin"), "file in an excluded folder was uploaded")
	// Changing excluded files doesn't trigger a sync.
	write(t, saves, "debug.log", "more noise")
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
		assert.Equal(t, c.want, Decide(c.local, c.remote, c.last))
	}
}

func TestLocalLock(t *testing.T) {
	dir := t.TempDir()
	l, err := AcquireLocalLock(dir, "g")
	require.NoError(t, err)
	if _, err := AcquireLocalLock(dir, "g"); err == nil {
		// same pid holds it; our own pid is treated as stale only if different
		t.Log("lock re-acquired by same process (expected: pid equals ours)")
	}
	l.Release()
	_, err = AcquireLocalLock(dir, "g")
	require.NoError(t, err)
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
		assert.FileExists(t, filepath.Join(cloudDir, "games", "game1", "current", "saves", filepath.FromSlash(rel)), "%s not uploaded", rel)
	}

	// A file-only pattern must not pull in unrelated folders.
	pc2, job2 := newDevice(t, setupCloud(t), "pc2")
	job2.Slots[0].Filters = cloud.Filters{Include: []string{"*/*.sav", "*/*.sav/**"}}
	write(t, filepath.Join(job2.Slots[0].LocalPath, "slot1"), "a.sav", "x")
	write(t, filepath.Join(job2.Slots[0].LocalPath, "slot1"), "shader.cache", "y")
	p, err := pc2.Plan(context.Background(), job2)
	require.NoError(t, err)
	require.Len(t, p.Local.Files, 1)
	require.Contains(t, p.Local.Files, "saves/slot1/a.sav")
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
	require.NoError(t, os.WriteFile(filepath.Join(cloudSaves(cloudDir), "b.sav"), []byte("XXX"), 0o644))
	write(t, saves, "a.sav", "a-2!")
	_, _, err := pc.Sync(context.Background(), job, ResolveAuto)
	require.NoError(t, err)
	require.Equal(t, "a-2!", read(t, cloudSaves(cloudDir), "a.sav"))
	require.Equal(t, "XXX", read(t, cloudSaves(cloudDir), "b.sav"))

	// A deleted save is removed from the cloud.
	require.NoError(t, os.Remove(filepath.Join(saves, "b.sav")))
	mustSync(t, pc, job, ResolveAuto, ActionPush)
	require.NoFileExists(t, filepath.Join(cloudSaves(cloudDir), "b.sav"), "b.sav still in cloud")
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

	require.NoFileExists(t, filepath.Join(cloudSaves(cloudDir), "stray.tmp"), "stray file not removed")
	require.Equal(t, "b-1", read(t, cloudSaves(cloudDir), "b.sav"))
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
	require.NoError(t, os.Chtimes(bPath, old, old))
	mustSync(t, deck, deckJob, ResolveAuto, ActionNone) // re-baseline after touching mtime

	write(t, pcSaves, "a.sav", "a-2!")
	mustSync(t, pc, pcJob, ResolveAuto, ActionPush)
	mustSync(t, deck, deckJob, ResolveAuto, ActionPull)
	require.Equal(t, "a-2!", read(t, deckSaves, "a.sav"))
	fi, err := os.Stat(bPath)
	require.NoError(t, err)
	require.True(t, fi.ModTime().Equal(old))

	require.NoError(t, os.Remove(filepath.Join(pcSaves, "b.sav")))
	mustSync(t, pc, pcJob, ResolveAuto, ActionPush)
	mustSync(t, deck, deckJob, ResolveAuto, ActionPull)
	require.NoFileExists(t, bPath, "b.sav not deleted on deck")
}

func TestDiff(t *testing.T) {
	src := map[string]FileInfo{"same": {Size: 1, Hash: "h"}, "changed": {Size: 1, Hash: "new"}, "added": {Size: 4, Hash: "a"}}
	dst := map[string]FileInfo{"same": {Size: 1, Hash: "h"}, "changed": {Size: 1, Hash: "old"}, "removed": {Size: 1, Hash: "r"}}
	d := diff(src, dst)
	assert.Equal(t, []string{"added", "changed"}, d.copy)
	assert.Equal(t, []string{"removed"}, d.del)
	assert.EqualValues(t, 5, d.bytes)
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
	require.NoError(t, err)
	require.Empty(t, backups)
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
	_, _, err := pc.Sync(ctx, job, ResolveAuto)
	require.ErrorIs(t, err, context.Canceled)
	pc.OnEvent = nil

	backups, err := pc.ListBackups(context.Background(), "game1")
	require.NoError(t, err)
	require.Empty(t, backups)
	after, _ := pc.States.Load("game1")
	require.True(t, after.LastSyncAt.Equal(before.LastSyncAt), "cancelled sync changed the last sync time")
	require.Empty(t, after.LastError, "cancelled sync recorded an error")
	require.Equal(t, "a-1", read(t, cloudSaves(cloudDir), "a.sav"))

	// The next sync does the work.
	mustSync(t, pc, job, ResolveAuto, ActionPush)
	require.Equal(t, "a-2!", read(t, cloudSaves(cloudDir), "a.sav"))
}

// A cancelled context stops the local scan.
func TestScanLocalCancelled(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "a.sav", "x")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := ScanLocal(ctx, []Slot{{Key: "s", LocalPath: dir}}, nil)
	require.ErrorIs(t, err, context.Canceled)
}

// A backup archive whose upload didn't complete is deleted.
func TestDiscardPartialBackup(t *testing.T) {
	cloudDir := setupCloud(t)
	pc, _ := newDevice(t, cloudDir, "pc")
	backups := filepath.Join(cloudDir, "games", "game1", "backups")
	write(t, backups, "20260101T000000.000Z_pc_cloud.zip", "half")
	pc.discardBackup("game1", "20260101T000000.000Z_pc_cloud")
	require.NoFileExists(t, filepath.Join(backups, "20260101T000000.000Z_pc_cloud.zip"), "partial backup still there")
}

// Restoring a backup keeps the current saves as a backup even when backups
// are off, and doesn't prune that backup away.
func TestRestoreBacksUpLocalWhenRetentionOff(t *testing.T) {
	cloudDir := setupCloud(t)
	pc, job := newDevice(t, cloudDir, "pc")
	ctx := context.Background()
	write(t, job.Slots[0].LocalPath, "a.sav", "a-1")
	mustSync(t, pc, job, ResolveAuto, ActionPush)
	write(t, job.Slots[0].LocalPath, "a.sav", "a-2")
	mustSync(t, pc, job, ResolveAuto, ActionPush) // backs up a-1 (cloud)
	backups, err := pc.ListBackups(ctx, "game1")
	require.NoError(t, err)
	require.Len(t, backups, 1)
	pc.Retention = 0
	require.NoError(t, pc.RestoreBackup(ctx, job, backups[0].Name))
	require.Equal(t, "a-1", read(t, job.Slots[0].LocalPath, "a.sav"))
	after, _ := pc.ListBackups(ctx, "game1")
	require.Len(t, after, 2)
}

// A truncated state file counts as "never synced" instead of failing syncs.
func TestCorruptStateIsIgnored(t *testing.T) {
	dir := t.TempDir()
	s := &StateStore{Dir: dir}
	require.NoError(t, os.WriteFile(filepath.Join(dir, "g.json"), []byte(`{"lastSyn`), 0o644))
	st, err := s.Load("g")
	require.NoError(t, err)
	require.Nil(t, st.LastSynced)
}

// "Back up now" works with backups off, pins the backup, and pruning keeps it
// until it is unpinned.
func TestManualBackupIsPinned(t *testing.T) {
	cloudDir := setupCloud(t)
	pc, job := newDevice(t, cloudDir, "pc")
	ctx := context.Background()
	pc.Retention = 0
	_, err := pc.BackupNow(ctx, job)
	require.Error(t, err, "BackupNow with no local saves should fail")
	write(t, job.Slots[0].LocalPath, "a.sav", "a-1")
	name, err := pc.BackupNow(ctx, job)
	require.NoError(t, err)
	list, err := pc.ListBackups(ctx, "game1")
	require.NoError(t, err)
	require.Len(t, list, 1)
	require.Equal(t, name, list[0].Name)
	require.Equal(t, "manual", list[0].Kind)
	require.True(t, list[0].Pinned)

	// Backups on, keep 1: later automatic backups rotate, the pinned one stays.
	pc.Retention = 1
	for _, c := range []string{"a-2", "a-3", "a-4"} {
		write(t, job.Slots[0].LocalPath, "a.sav", c)
		mustSync(t, pc, job, ResolveAuto, ActionPush)
	}
	list, _ = pc.ListBackups(ctx, "game1")
	require.Len(t, list, 2)
	var kept bool
	for _, b := range list {
		kept = kept || (b.Name == name && b.Pinned)
	}
	require.True(t, kept)

	// Unpinned, it is pruned like any other once there are more than Retention.
	require.NoError(t, pc.SetBackupPinned(ctx, "game1", name, false))
	write(t, job.Slots[0].LocalPath, "a.sav", "a-5")
	mustSync(t, pc, job, ResolveAuto, ActionPush)
	// Pruning runs in the background of the sync; run it again to be sure.
	require.NoError(t, pc.prune(ctx, "game1"))
	list, _ = pc.ListBackups(ctx, "game1")
	require.Len(t, list, 1, "want only the latest automatic backup")
	require.NotEqual(t, name, list[0].Name)
	assert.Error(t, pc.SetBackupPinned(ctx, "game1", "nope", true), "pinning an unknown backup should fail")
}

// Retention 0 means backups are off: pruning never deletes anything.
func TestPruneWithRetentionOffKeepsEverything(t *testing.T) {
	cloudDir := setupCloud(t)
	pc, job := newDevice(t, cloudDir, "pc")
	ctx := context.Background()
	write(t, job.Slots[0].LocalPath, "a.sav", "a-1")
	_, err := pc.BackupNow(ctx, job)
	require.NoError(t, err)
	require.NoError(t, pc.SetBackupPinned(ctx, "game1", mustOnlyBackup(t, pc), false))
	pc.Retention = 0
	require.NoError(t, pc.prune(ctx, "game1"))
	list, _ := pc.ListBackups(ctx, "game1")
	require.Len(t, list, 1)
}

func mustOnlyBackup(t *testing.T, e *Engine) string {
	t.Helper()
	list, err := e.ListBackups(context.Background(), "game1")
	require.NoError(t, err)
	require.Len(t, list, 1)
	return list[0].Name
}

func TestPlanActionFor(t *testing.T) {
	withRemote := &Plan{Action: ActionConflict, Remote: &Manifest{}}
	noRemote := &Plan{Action: ActionPush}
	cases := []struct {
		p    *Plan
		res  Resolution
		want Action
		err  bool
	}{
		{withRemote, ResolveAuto, ActionConflict, false},
		{withRemote, ResolveLocal, ActionPush, false},
		{withRemote, ResolveCloud, ActionPull, false},
		{noRemote, ResolveLocal, ActionPush, false},
		{noRemote, ResolveCloud, ActionNone, true},
	}
	for _, c := range cases {
		got, err := c.p.ActionFor(c.res)
		assert.Equal(t, c.want, got, "ActionFor(%q)", c.res)
		assert.Equal(t, c.err, err != nil, "ActionFor(%q) error", c.res)
	}
}

// Deleting a backup removes it (pinned or not) and drops its pin; others stay.
func TestDeleteBackup(t *testing.T) {
	cloudDir := setupCloud(t)
	pc, job := newDevice(t, cloudDir, "pc")
	ctx := context.Background()
	write(t, job.Slots[0].LocalPath, "a.sav", "a-1")
	mustSync(t, pc, job, ResolveAuto, ActionPush) // the cloud has nothing to back up yet
	pinned, err := pc.BackupNow(ctx, job)
	require.NoError(t, err)
	write(t, job.Slots[0].LocalPath, "a.sav", "a-2")
	mustSync(t, pc, job, ResolveAuto, ActionPush) // automatic backup of the cloud side
	list, _ := pc.ListBackups(ctx, "game1")
	require.Len(t, list, 2)
	require.NoError(t, pc.DeleteBackup(ctx, "game1", pinned))
	list, _ = pc.ListBackups(ctx, "game1")
	require.Len(t, list, 1, "backups after delete")
	require.NotEqual(t, pinned, list[0].Name)
	require.False(t, list[0].Pinned)
	pins, _ := pc.readPins(ctx, "game1")
	assert.Empty(t, pins, "pins")
	assert.Error(t, pc.DeleteBackup(ctx, "game1", pinned), "deleting a missing backup should fail")
	assert.Error(t, pc.DeleteBackup(ctx, "game1", "../x"), "invalid name should fail")
}
