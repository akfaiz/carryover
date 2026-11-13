package syncer

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// A save folder that disappears (unmounted card, wrong path, new Proton
// prefix) must never be pushed as "delete everything": the cloud copy and the
// other devices keep their saves until the user chooses.
func TestVanishedSaveFolderIsAConflict(t *testing.T) {
	cloudDir := setupCloud(t)
	pc, pcJob := newDevice(t, cloudDir, "pc")
	deck, deckJob := newDevice(t, cloudDir, "deck")
	pc.Retention, deck.Retention = 0, 0
	pcSaves, deckSaves := pcJob.Slots[0].LocalPath, deckJob.Slots[0].LocalPath

	write(t, pcSaves, "slot1.sav", "40 hours of progress")
	mustSync(t, pc, pcJob, ResolveAuto, ActionPush)
	mustSync(t, deck, deckJob, ResolveAuto, ActionPull)

	require.NoError(t, os.RemoveAll(pcSaves))
	mustSync(t, pc, pcJob, ResolveAuto, ActionConflict)
	st, err := pc.States.Load("game1")
	require.NoError(t, err)
	require.True(t, st.Conflict, "the conflict isn't flagged")
	require.FileExists(t, filepath.Join(cloudDir, "games", "game1", "current", "saves", "slot1.sav"), "the cloud save was deleted")

	// The other device is unaffected.
	mustSync(t, deck, deckJob, ResolveAuto, ActionNone)
	require.Equal(t, "40 hours of progress", read(t, deckSaves, "slot1.sav"))

	// Keeping the cloud brings the folder back.
	mustSync(t, pc, pcJob, ResolveCloud, ActionPull)
	require.Equal(t, "40 hours of progress", read(t, pcSaves, "slot1.sav"))
}

// A deliberate "keep local" after the folder was emptied still works.
func TestEmptiedFolderCanBePushedOnPurpose(t *testing.T) {
	cloudDir := setupCloud(t)
	pc, pcJob := newDevice(t, cloudDir, "pc")
	pc.Retention = 0
	write(t, pcJob.Slots[0].LocalPath, "slot1.sav", "x")
	mustSync(t, pc, pcJob, ResolveAuto, ActionPush)
	require.NoError(t, os.Remove(filepath.Join(pcJob.Slots[0].LocalPath, "slot1.sav")))
	mustSync(t, pc, pcJob, ResolveAuto, ActionConflict)
	mustSync(t, pc, pcJob, ResolveLocal, ActionPush)
	require.NoFileExists(t, filepath.Join(cloudDir, "games", "game1", "current", "saves", "slot1.sav"))
	mustSync(t, pc, pcJob, ResolveAuto, ActionNone)
}

// makeDirLink makes link point at dir: a symbolic link, or on Windows (where
// that needs a privilege) a junction. It skips the test if neither works.
func makeDirLink(t *testing.T, dir, link string) {
	t.Helper()
	if err := os.Symlink(dir, link); err == nil {
		return
	}
	if runtime.GOOS == "windows" {
		if err := exec.Command("cmd", "/c", "mklink", "/J", link, dir).Run(); err == nil {
			return
		}
	}
	t.Skip("cannot create a directory link here")
}

// A save folder reached through a link scans and syncs like the real one
// (WalkDir doesn't follow a link used as the root).
func TestLinkedSaveFolder(t *testing.T) {
	real := filepath.Join(t.TempDir(), "real")
	write(t, real, "slot1.sav", "progress")
	link := filepath.Join(t.TempDir(), "link")
	makeDirLink(t, real, link)

	m, err := ScanLocal(context.Background(), []Slot{{Key: "saves", LocalPath: link}}, nil)
	require.NoError(t, err)
	require.Len(t, m.Files, 1, "a linked save folder scanned as empty")

	cloudDir := setupCloud(t)
	pc, job := newDevice(t, cloudDir, "pc")
	job.Slots[0].LocalPath = link
	mustSync(t, pc, job, ResolveAuto, ActionPush)
	require.FileExists(t, filepath.Join(cloudDir, "games", "game1", "current", "saves", "slot1.sav"))
	mustSync(t, pc, job, ResolveAuto, ActionNone)

	// A second device pulling into a linked folder writes through the link.
	deck, deckJob := newDevice(t, cloudDir, "deck")
	deckReal := filepath.Join(t.TempDir(), "real")
	require.NoError(t, os.MkdirAll(deckReal, 0o755))
	deckLink := filepath.Join(t.TempDir(), "link")
	makeDirLink(t, deckReal, deckLink)
	deckJob.Slots[0].LocalPath = deckLink
	mustSync(t, deck, deckJob, ResolveAuto, ActionPull)
	require.Equal(t, "progress", read(t, deckReal, "slot1.sav"))
}

// A conflict prompt can keep a plan waiting while another device pushes. Keeping
// "local" afterwards must not upload over the other device's new files.
func TestStalePlanDoesNotUploadOverNewerCloud(t *testing.T) {
	cloudDir := setupCloud(t)
	pc, pcJob := newDevice(t, cloudDir, "pc")
	deck, deckJob := newDevice(t, cloudDir, "deck")
	pc.Retention, deck.Retention = 0, 0
	pcSaves, deckSaves := pcJob.Slots[0].LocalPath, deckJob.Slots[0].LocalPath
	write(t, pcSaves, "slot1.sav", "base")
	mustSync(t, pc, pcJob, ResolveAuto, ActionPush)
	mustSync(t, deck, deckJob, ResolveAuto, ActionPull)

	write(t, pcSaves, "slot1.sav", "pc-edit")
	write(t, deckSaves, "slot1.sav", "deck-edit-1")
	mustSync(t, deck, deckJob, ResolveAuto, ActionPush)
	plan, err := pc.Plan(context.Background(), pcJob)
	require.NoError(t, err)
	require.Equal(t, ActionConflict, plan.Action)
	plan.plannedAt = time.Now().Add(-time.Minute) // the user took a while to answer

	write(t, deckSaves, "slot1.sav", "deck-edit-2!") // meanwhile the deck pushes again
	mustSync(t, deck, deckJob, ResolveAuto, ActionPush)

	_, err = pc.Apply(context.Background(), plan, ResolveLocal)
	require.ErrorIs(t, err, errCloudChanged)
	cloudSaves := filepath.Join(cloudDir, "games", "game1", "current", "saves")
	require.Equal(t, "deck-edit-2!", read(t, cloudSaves, "slot1.sav"), "the stale push overwrote the cloud")

	// The cloud is still consistent: the pc can now choose again.
	mustSync(t, pc, pcJob, ResolveCloud, ActionPull)
	require.Equal(t, "deck-edit-2!", read(t, pcSaves, "slot1.sav"))
	mustSync(t, deck, deckJob, ResolveAuto, ActionNone)
}

// Excluding a file in the slot's filters stops syncing it; it must not be
// deleted from the cloud or from the other devices.
func TestNewExcludeKeepsFilesEverywhere(t *testing.T) {
	cloudDir := setupCloud(t)
	pc, pcJob := newDevice(t, cloudDir, "pc")
	deck, deckJob := newDevice(t, cloudDir, "deck")
	pc.Retention, deck.Retention = 0, 0
	pcSaves, deckSaves := pcJob.Slots[0].LocalPath, deckJob.Slots[0].LocalPath
	write(t, pcSaves, "a.sav", "x")
	write(t, pcSaves, "keep.dat", "important")
	mustSync(t, pc, pcJob, ResolveAuto, ActionPush)
	mustSync(t, deck, deckJob, ResolveAuto, ActionPull)

	// The filter is edited and reaches both devices; a.sav changes on the deck.
	exclude := func(j *Job) { j.Slots[0].Filters.Exclude = []string{"keep.dat"} }
	exclude(&pcJob)
	exclude(&deckJob)
	write(t, deckSaves, "a.sav", "changed on the deck")
	mustSync(t, deck, deckJob, ResolveAuto, ActionPush)
	cloudSaves := filepath.Join(cloudDir, "games", "game1", "current", "saves")
	require.FileExists(t, filepath.Join(cloudSaves, "keep.dat"), "the newly excluded file was deleted from the cloud")
	require.FileExists(t, filepath.Join(deckSaves, "keep.dat"))

	mustSync(t, pc, pcJob, ResolveAuto, ActionPull)
	require.Equal(t, "changed on the deck", read(t, pcSaves, "a.sav"))
	require.Equal(t, "important", read(t, pcSaves, "keep.dat"), "the newly excluded file was deleted on another device")
}
