package syncer

import (
	"context"
	"io"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/akfaiz/carryover/internal/cloudsync/localfs"
)

// newSAFDevice is newDevice whose save folder has no file path: it lives in
// an in-memory localfs registered for ":saf:", as an Android folder granted
// through the Storage Access Framework does.
func newSAFDevice(t *testing.T, cloudDir, id string) (*Engine, Job, *localfs.Mem, string) {
	t.Helper()
	e, job := newDevice(t, cloudDir, id)
	mem := localfs.NewMem()
	var tick time.Duration
	mem.Now = func() time.Time { // every write gets a later time, as on a real device
		tick += time.Second
		return time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC).Add(tick)
	}
	localfs.Register(localfs.SAF, mem)
	t.Cleanup(func() { localfs.Register(localfs.SAF, nil) })
	saves := localfs.SAF + "com.android.externalstorage.documents/primary%3AEmu/saves"
	job.Slots[0].LocalPath = saves
	return e, job, mem, saves
}

func writeFS(t *testing.T, p, content string) {
	t.Helper()
	w, err := localfs.Create(p)
	require.NoError(t, err)
	_, err = io.WriteString(w, content)
	require.NoError(t, err)
	require.NoError(t, w.Close())
}

func readFS(t *testing.T, p string) string {
	t.Helper()
	r, err := localfs.Open(p)
	require.NoError(t, err)
	defer r.Close()
	b, err := io.ReadAll(r)
	require.NoError(t, err)
	return string(b)
}

// A device whose saves are in a SAF folder syncs with one using file paths:
// pull into an empty folder, push changes and deletions, conflicts, cloud
// backups taken from and restored into the SAF folder.
func TestSAFSaveFolder(t *testing.T) {
	cloudDir := setupCloud(t)
	pc, pcJob := newDevice(t, cloudDir, "pc")
	phone, phoneJob, mem, saves := newSAFDevice(t, cloudDir, "phone")
	pcSaves := pcJob.Slots[0].LocalPath
	ctx := context.Background()

	write(t, pcSaves, "slot1.sav", "pc-1")
	write(t, filepath.Join(pcSaves, "dir"), "a.sav", "nested")
	mustSync(t, pc, pcJob, ResolveAuto, ActionPush)

	// The phone's folder doesn't exist yet: a pull creates it.
	mustSync(t, phone, phoneJob, ResolveAuto, ActionPull)
	assert.Equal(t, map[string]string{saves + "/slot1.sav": "pc-1", saves + "/dir/a.sav": "nested"}, mem.Files())
	mustSync(t, phone, phoneJob, ResolveAuto, ActionNone)

	// The phone changes a save and deletes another.
	writeFS(t, saves+"/slot1.sav", "phone-2")
	require.NoError(t, localfs.Remove(saves+"/dir/a.sav"))
	mustSync(t, phone, phoneJob, ResolveAuto, ActionPush)
	mustSync(t, pc, pcJob, ResolveAuto, ActionPull)
	assert.Equal(t, "phone-2", read(t, pcSaves, "slot1.sav"))
	assert.NoFileExists(t, filepath.Join(pcSaves, "dir", "a.sav"))

	// Both change: a conflict; keeping the cloud's backs up the phone's first
	// (Retention 3: a cloud backup read from the SAF folder).
	writeFS(t, saves+"/slot1.sav", "phone-conflict")
	write(t, pcSaves, "slot1.sav", "pc-33")
	mustSync(t, pc, pcJob, ResolveAuto, ActionPush)
	mustSync(t, phone, phoneJob, ResolveAuto, ActionConflict)
	mustSync(t, phone, phoneJob, ResolveCloud, ActionPull)
	assert.Equal(t, "pc-33", readFS(t, saves+"/slot1.sav"))

	backups, err := phone.ListBackups(ctx, phoneJob.GameID)
	require.NoError(t, err)
	var local string
	for _, b := range backups {
		if b.Kind == "local" && b.DeviceName == "phone" {
			local = b.Name
		}
	}
	require.NotEmpty(t, local, "no backup of the phone's saves in %v", backups)

	// Restoring it writes the backed-up save into the SAF folder.
	require.NoError(t, phone.RestoreBackup(ctx, phoneJob, local))
	assert.Equal(t, "phone-conflict", readFS(t, saves+"/slot1.sav"))
	mustSync(t, pc, pcJob, ResolveAuto, ActionPull)
	assert.Equal(t, "phone-conflict", read(t, pcSaves, "slot1.sav"))
}

// Without cloud backups, a pull that replaces SAF saves keeps a safety copy
// of them on the device, readable from the SAF folder.
func TestSAFSafetyCopy(t *testing.T) {
	cloudDir := setupCloud(t)
	pc, pcJob := newDevice(t, cloudDir, "pc")
	phone, phoneJob, _, saves := newSAFDevice(t, cloudDir, "phone")
	pc.Retention, phone.Retention = 0, 0
	phone.SafetyDir = filepath.Join(t.TempDir(), "safety")
	pcSaves := pcJob.Slots[0].LocalPath

	write(t, pcSaves, "a.sav", "pc-1")
	mustSync(t, pc, pcJob, ResolveAuto, ActionPush)
	mustSync(t, phone, phoneJob, ResolveAuto, ActionPull)
	writeFS(t, saves+"/a.sav", "phone-conflict")
	write(t, pcSaves, "a.sav", "pc-22")
	mustSync(t, pc, pcJob, ResolveAuto, ActionPush)
	mustSync(t, phone, phoneJob, ResolveCloud, ActionPull)
	assert.Equal(t, "pc-22", readFS(t, saves+"/a.sav"))

	names, err := phone.safetyNames(phoneJob.GameID)
	require.NoError(t, err)
	require.Len(t, names, 1)
	assert.Equal(t, "phone-conflict", readZipEntry(t, phone.safetyPath(phoneJob.GameID, names[0]), "saves/a.sav"))
}

// A rescan reuses the hashes of SAF files whose size and time didn't change,
// and notices the ones that did.
func TestSAFScanCache(t *testing.T) {
	_, job, _, saves := newSAFDevice(t, setupCloud(t), "phone")
	writeFS(t, saves+"/a.sav", "one")
	writeFS(t, saves+"/b.sav", "two")
	first, err := ScanLocal(context.Background(), job.Slots, nil)
	require.NoError(t, err)
	require.Len(t, first.Files, 2)

	writeFS(t, saves+"/b.sav", "TWO")
	second, err := ScanLocal(context.Background(), job.Slots, first)
	require.NoError(t, err)
	assert.Equal(t, first.Files["saves/a.sav"], second.Files["saves/a.sav"])
	assert.NotEqual(t, first.Files["saves/b.sav"].Hash, second.Files["saves/b.sav"].Hash)
}

// Some providers report no modification time (Android's modTime 0): a cached
// hash can't be trusted then, or a change that keeps the size is never seen.
func TestSAFScanWithoutModTime(t *testing.T) {
	_, job, mem, saves := newSAFDevice(t, setupCloud(t), "phone")
	mem.Now = func() time.Time { return time.UnixMilli(0) }
	writeFS(t, saves+"/a.sav", "one")
	first, err := ScanLocal(context.Background(), job.Slots, nil)
	require.NoError(t, err)

	writeFS(t, saves+"/a.sav", "two") // same size, same (missing) time
	second, err := ScanLocal(context.Background(), job.Slots, first)
	require.NoError(t, err)
	assert.NotEqual(t, first.Files["saves/a.sav"].Hash, second.Files["saves/a.sav"].Hash)
}

// A SAF save folder is a valid location; a relative path still isn't.
func TestSAFValidateJob(t *testing.T) {
	job := Job{GameID: "g", Slots: []Slot{{Key: "s", LocalPath: localfs.SAF + "a/b"}}}
	require.NoError(t, validateJob(job))
	job.Slots[0].LocalPath = "relative/saves"
	require.Error(t, validateJob(job))
}
