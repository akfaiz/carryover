package syncer

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/klauspost/compress/zstd"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// cloudObjects lists the objects under games/game1/current, by path relative to it.
func cloudObjects(t *testing.T, cloudDir string) []string {
	t.Helper()
	root := filepath.Join(cloudDir, "games", "game1", "current")
	var out []string
	err := filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, _ := filepath.Rel(root, p)
		out = append(out, filepath.ToSlash(rel))
		return nil
	})
	require.NoError(t, err)
	sort.Strings(out)
	return out
}

func cloudManifest(t *testing.T, e *Engine) Manifest {
	t.Helper()
	var m Manifest
	found, err := e.Client.ReadJSON(context.Background(), manifestPath("game1"), &m)
	require.NoError(t, err)
	require.True(t, found)
	return m
}

func zstdBytes(t *testing.T, b []byte) []byte {
	t.Helper()
	enc, err := zstd.NewWriter(nil)
	require.NoError(t, err)
	return enc.EncodeAll(b, nil)
}

func sha(b []byte) string {
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

func TestStoredNames(t *testing.T) {
	assert.Equal(t, "a/b.sav", storedName("a/b.sav", FormatRaw))
	assert.Equal(t, "a/b.sav.zst", storedName("a/b.sav", FormatZstd))
	// A save that already ends in .zst keeps that part of its name.
	assert.Equal(t, "x.zst.zst", storedName("x.zst", FormatZstd))
	rel, ok := logicalName("x.zst.zst", FormatZstd)
	assert.True(t, ok)
	assert.Equal(t, "x.zst", rel)
	_, ok = logicalName("b.sav", FormatZstd)
	assert.False(t, ok, "a raw object isn't a file of the compressed format")
	rel, ok = logicalName("b.sav.zst", FormatRaw)
	assert.True(t, ok)
	assert.Equal(t, "b.sav.zst", rel)

	files, others := cloudView(map[string]int64{"s/a.zst": 5, "s/b": 7}, FormatZstd)
	assert.Equal(t, map[string]int64{"s/a": 5}, files)
	assert.Equal(t, map[string]int64{"s/b": 7}, others)
}

func TestCompressRoundTrip(t *testing.T) {
	dir := t.TempDir()
	for _, content := range [][]byte{nil, []byte("x"), bytes.Repeat([]byte("save data "), 100000)} {
		src := filepath.Join(dir, "src")
		require.NoError(t, os.WriteFile(src, content, 0o644))
		c, err := newCompressor()
		require.NoError(t, err)
		mt := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)
		n, err := c.file(src, filepath.Join(dir, "obj", "src.zst"), mt)
		require.NoError(t, err)
		fi, err := os.Stat(filepath.Join(dir, "obj", "src.zst"))
		require.NoError(t, err)
		assert.Equal(t, fi.Size(), n)
		assert.True(t, fi.ModTime().Equal(mt), "the object keeps the save's time")
		if len(content) > 1000 {
			assert.Less(t, n, int64(len(content))/10)
		}

		dst := filepath.Join(dir, "out", "save.sav")
		want := FileInfo{Size: int64(len(content)), Hash: sha(content), ModTime: mt}
		require.NoError(t, decompressTo(filepath.Join(dir, "obj", "src.zst"), dst, want))
		got, err := os.ReadFile(dst)
		require.NoError(t, err)
		assert.True(t, bytes.Equal(content, got))
		fi, err = os.Stat(dst)
		require.NoError(t, err)
		assert.True(t, fi.ModTime().Equal(mt), "the save gets the manifest's time")
	}
}

// A downloaded object that isn't what the manifest describes never replaces
// the save.
func TestDecompressToRefusesMismatch(t *testing.T) {
	dir := t.TempDir()
	dst := filepath.Join(dir, "save.sav")
	require.NoError(t, os.WriteFile(dst, []byte("current save"), 0o644))
	content := []byte("cloud save")
	obj := filepath.Join(dir, "save.sav.zst")
	require.NoError(t, os.WriteFile(obj, zstdBytes(t, content), 0o644))

	cases := map[string]FileInfo{
		"other hash": {Size: int64(len(content)), Hash: sha([]byte("something else"))},
		"shorter":    {Size: int64(len(content)) - 1, Hash: sha(content)},
		"longer":     {Size: int64(len(content)) + 1, Hash: sha(content)},
	}
	for name, want := range cases {
		require.ErrorIs(t, decompressTo(obj, dst, want), errNoMatch, name)
		assert.Equal(t, "current save", read(t, dir, "save.sav"), name)
	}

	require.NoError(t, os.WriteFile(obj, []byte("not zstd at all"), 0o644))
	require.Error(t, decompressTo(obj, dst, FileInfo{Size: 5, Hash: sha(content)}))
	assert.Equal(t, "current save", read(t, dir, "save.sav"))
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	for _, en := range entries {
		assert.NotContains(t, en.Name(), ".carryover-pull", "no temporary file is left behind")
	}
}

// Two devices sync through a compressed cloud folder: every object is
// compressed, the manifest describes the files themselves, and a version that
// predates the format finds none of the files it would download.
func TestCompressedSync(t *testing.T) {
	cloudDir := setupCloud(t)
	pc, pcJob := newDevice(t, cloudDir, "pc")
	deck, deckJob := newDevice(t, cloudDir, "deck")
	pc.WriteFormat, deck.WriteFormat = FormatZstd, FormatZstd
	pcSaves, deckSaves := pcJob.Slots[0].LocalPath, deckJob.Slots[0].LocalPath

	big := strings.Repeat("progress ", 20000)
	write(t, pcSaves, "slot1.sav", big)
	write(t, pcSaves, "old.zst", "a save named like an object")
	write(t, filepath.Join(pcSaves, "dir"), "a.sav", "nested")
	mustSync(t, pc, pcJob, ResolveAuto, ActionPush)

	assert.Equal(t, []string{"saves/dir/a.sav.zst", "saves/old.zst.zst", "saves/slot1.sav.zst"}, cloudObjects(t, cloudDir))
	m := cloudManifest(t, pc)
	assert.Equal(t, FormatZstd, m.Format)
	require.Contains(t, m.Files, "saves/slot1.sav")
	f := m.Files["saves/slot1.sav"]
	assert.Equal(t, int64(len(big)), f.Size)
	assert.Equal(t, sha([]byte(big)), f.Hash)
	fi, err := os.Stat(filepath.Join(cloudDir, "games", "game1", "current", "saves", "slot1.sav.zst"))
	require.NoError(t, err)
	assert.Equal(t, fi.Size(), f.StoredSize)
	assert.Less(t, f.StoredSize, f.Size/10)
	for k := range m.Files {
		_, err := os.Stat(filepath.Join(cloudDir, "games", "game1", "current", filepath.FromSlash(k)))
		assert.ErrorIs(t, err, os.ErrNotExist, "an older version must not find %s", k)
	}

	mustSync(t, deck, deckJob, ResolveAuto, ActionPull)
	assert.Equal(t, big, read(t, deckSaves, "slot1.sav"))
	assert.Equal(t, "nested", read(t, filepath.Join(deckSaves, "dir"), "a.sav"))
	assert.Equal(t, "a save named like an object", read(t, deckSaves, "old.zst"))
	mustSync(t, deck, deckJob, ResolveAuto, ActionNone)

	// A change and a deletion travel back; only the changed file is uploaded.
	write(t, deckSaves, "slot1.sav", big+"more")
	require.NoError(t, os.Remove(filepath.Join(deckSaves, "old.zst")))
	_, _, err = deck.Sync(context.Background(), deckJob, ResolveAuto)
	require.NoError(t, err)
	assert.Equal(t, []string{"saves/dir/a.sav.zst", "saves/slot1.sav.zst"}, cloudObjects(t, cloudDir))
	mustSync(t, pc, pcJob, ResolveAuto, ActionPull)
	assert.Equal(t, big+"more", read(t, pcSaves, "slot1.sav"))
	_, err = os.Stat(filepath.Join(pcSaves, "old.zst"))
	assert.ErrorIs(t, err, os.ErrNotExist)

	// The cloud backup taken before deck's push holds the files themselves.
	backups, err := pc.ListBackups(context.Background(), "game1")
	require.NoError(t, err)
	var cloudBackup string
	for _, b := range backups {
		if b.Kind == "cloud" {
			cloudBackup = b.Name
		}
	}
	require.NotEmpty(t, cloudBackup)
	zipPath := filepath.Join(cloudDir, "games", "game1", "backups", cloudBackup+".zip")
	assert.Equal(t, big, readZipEntry(t, zipPath, "saves/slot1.sav"))
	assert.Equal(t, "a save named like an object", readZipEntry(t, zipPath, "saves/old.zst"))

	// Restoring it brings the files back and uploads them compressed.
	require.NoError(t, pc.RestoreBackup(context.Background(), pcJob, cloudBackup))
	assert.Equal(t, big, read(t, pcSaves, "slot1.sav"))
	assert.Equal(t, "a save named like an object", read(t, pcSaves, "old.zst"))
	assert.Equal(t, []string{"saves/dir/a.sav.zst", "saves/old.zst.zst", "saves/slot1.sav.zst"}, cloudObjects(t, cloudDir))
}

// A folder moves between formats on the first push with changes: all of it is
// uploaded again in the new format and the old objects are removed. Each
// format is read whatever the device writes.
func TestFormatMigration(t *testing.T) {
	cloudDir := setupCloud(t)
	pc, pcJob := newDevice(t, cloudDir, "pc")       // writes FormatRaw, as today
	deck, deckJob := newDevice(t, cloudDir, "deck") // writes FormatZstd
	deck.WriteFormat = FormatZstd
	pcSaves, deckSaves := pcJob.Slots[0].LocalPath, deckJob.Slots[0].LocalPath

	write(t, pcSaves, "slot1.sav", "pc-1")
	write(t, pcSaves, "slot2.sav", "unchanged")
	mustSync(t, pc, pcJob, ResolveAuto, ActionPush)
	assert.Equal(t, []string{"saves/slot1.sav", "saves/slot2.sav"}, cloudObjects(t, cloudDir))

	// Pulling doesn't change the format.
	mustSync(t, deck, deckJob, ResolveAuto, ActionPull)
	assert.Equal(t, []string{"saves/slot1.sav", "saves/slot2.sav"}, cloudObjects(t, cloudDir))

	write(t, deckSaves, "slot1.sav", "deck-1")
	mustSync(t, deck, deckJob, ResolveAuto, ActionPush)
	assert.Equal(t, []string{"saves/slot1.sav.zst", "saves/slot2.sav.zst"}, cloudObjects(t, cloudDir))
	assert.Equal(t, FormatZstd, cloudManifest(t, deck).Format)

	mustSync(t, pc, pcJob, ResolveAuto, ActionPull)
	assert.Equal(t, "deck-1", read(t, pcSaves, "slot1.sav"))

	// And back, as a version that writes raw files does.
	write(t, pcSaves, "slot1.sav", "pc-2")
	mustSync(t, pc, pcJob, ResolveAuto, ActionPush)
	assert.Equal(t, []string{"saves/slot1.sav", "saves/slot2.sav"}, cloudObjects(t, cloudDir))
	assert.Equal(t, FormatRaw, cloudManifest(t, pc).Format)
	mustSync(t, deck, deckJob, ResolveAuto, ActionPull)
	assert.Equal(t, "pc-2", read(t, deckSaves, "slot1.sav"))
	assert.Equal(t, "unchanged", read(t, deckSaves, "slot2.sav"))
}

// Objects of another format that no manifest describes (an interrupted push,
// or a version that predates the format pushing with include filters) are
// replaced or cleared by the next push.
func TestCompressedPushClearsOtherFormat(t *testing.T) {
	cloudDir := setupCloud(t)
	pc, pcJob := newDevice(t, cloudDir, "pc")
	deck, deckJob := newDevice(t, cloudDir, "deck")
	pc.WriteFormat, deck.WriteFormat = FormatZstd, FormatZstd
	pcSaves, deckSaves := pcJob.Slots[0].LocalPath, deckJob.Slots[0].LocalPath
	current := filepath.Join(cloudDir, "games", "game1", "current", "saves")

	write(t, pcSaves, "slot1.sav", "pc-1")
	mustSync(t, pc, pcJob, ResolveAuto, ActionPush)
	// Left over: a raw object, and a damaged compressed one.
	write(t, current, "stray.sav", "raw")
	require.NoError(t, os.WriteFile(filepath.Join(current, "slot1.sav.zst"), []byte("damaged"), 0o644))

	_, _, err := deck.Sync(context.Background(), deckJob, ResolveAuto)
	require.ErrorContains(t, err, "doesn't match the cloud manifest")
	_, err = os.Stat(filepath.Join(deckSaves, "slot1.sav"))
	require.ErrorIs(t, err, os.ErrNotExist, "nothing is written from the damaged object")

	write(t, pcSaves, "slot2.sav", "pc-2")
	mustSync(t, pc, pcJob, ResolveAuto, ActionPush)
	assert.Equal(t, []string{"saves/slot1.sav.zst", "saves/slot2.sav.zst"}, cloudObjects(t, cloudDir))
	mustSync(t, deck, deckJob, ResolveAuto, ActionPull)
	assert.Equal(t, "pc-1", read(t, deckSaves, "slot1.sav"))
	assert.Equal(t, "pc-2", read(t, deckSaves, "slot2.sav"))
}

// A migration interrupted after uploading leaves compressed objects next to
// the raw folder; the next push replaces or deletes them all, including one of
// a file deleted since.
func TestMigrationAfterInterruptedOne(t *testing.T) {
	cloudDir := setupCloud(t)
	pc, pcJob := newDevice(t, cloudDir, "pc")
	deck, deckJob := newDevice(t, cloudDir, "deck")
	deck.WriteFormat = FormatZstd
	pcSaves, deckSaves := pcJob.Slots[0].LocalPath, deckJob.Slots[0].LocalPath
	current := filepath.Join(cloudDir, "games", "game1", "current", "saves")

	write(t, pcSaves, "slot1.sav", "pc-1")
	write(t, pcSaves, "slot2.sav", "pc-2")
	mustSync(t, pc, pcJob, ResolveAuto, ActionPush)
	for _, name := range []string{"slot1.sav", "slot2.sav"} {
		b, err := os.ReadFile(filepath.Join(current, name))
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(filepath.Join(current, name+".zst"), zstdBytes(t, b), 0o644))
	}

	// The raw manifest still describes the folder.
	mustSync(t, deck, deckJob, ResolveAuto, ActionPull)
	assert.Equal(t, "pc-2", read(t, deckSaves, "slot2.sav"))

	require.NoError(t, os.Remove(filepath.Join(deckSaves, "slot2.sav")))
	write(t, deckSaves, "slot1.sav", "deck-1")
	mustSync(t, deck, deckJob, ResolveAuto, ActionPush)
	assert.Equal(t, []string{"saves/slot1.sav.zst"}, cloudObjects(t, cloudDir))
	mustSync(t, pc, pcJob, ResolveAuto, ActionPull)
	assert.Equal(t, "deck-1", read(t, pcSaves, "slot1.sav"))
	_, err := os.Stat(filepath.Join(pcSaves, "slot2.sav"))
	assert.ErrorIs(t, err, os.ErrNotExist)
}

// An object replaced with other content after the manifest was written (a
// push still running elsewhere) is refused, and the save is left as it was.
func TestCompressedPullRefusesOtherContent(t *testing.T) {
	cloudDir := setupCloud(t)
	pc, pcJob := newDevice(t, cloudDir, "pc")
	deck, deckJob := newDevice(t, cloudDir, "deck")
	pc.WriteFormat, deck.WriteFormat = FormatZstd, FormatZstd
	pcSaves, deckSaves := pcJob.Slots[0].LocalPath, deckJob.Slots[0].LocalPath

	write(t, pcSaves, "slot1.sav", "pc-1")
	mustSync(t, pc, pcJob, ResolveAuto, ActionPush)
	mustSync(t, deck, deckJob, ResolveAuto, ActionPull)
	write(t, pcSaves, "slot1.sav", "pc-2")
	mustSync(t, pc, pcJob, ResolveAuto, ActionPush)

	obj := filepath.Join(cloudDir, "games", "game1", "current", "saves", "slot1.sav.zst")
	require.NoError(t, os.WriteFile(obj, zstdBytes(t, []byte("pc-3")), 0o644))
	_, _, err := deck.Sync(context.Background(), deckJob, ResolveAuto)
	require.ErrorContains(t, err, "doesn't match the cloud manifest")
	assert.Equal(t, "pc-1", read(t, deckSaves, "slot1.sav"))
}

func TestNewerSaveFormatRefused(t *testing.T) {
	cloudDir := setupCloud(t)
	pc, pcJob := newDevice(t, cloudDir, "pc")
	write(t, pcJob.Slots[0].LocalPath, "slot1.sav", "pc-1")
	require.NoError(t, pc.Client.WriteJSON(context.Background(), manifestPath("game1"), Manifest{
		Format: maxFormat + 1, Revision: "r1", Files: map[string]FileInfo{"saves/slot1.sav": {Size: 1, Hash: "x"}},
	}))
	_, _, err := pc.Sync(context.Background(), pcJob, ResolveLocal)
	require.ErrorIs(t, err, ErrNewerSaveFormat)
	assert.Equal(t, "pc-1", read(t, pcJob.Slots[0].LocalPath, "slot1.sav"))
}

// A version that predates compressed saves backs up a compressed folder's
// objects as they are; restoring such a backup decompresses them.
func TestRestoreBackupOfCompressedObjects(t *testing.T) {
	cloudDir := setupCloud(t)
	pc, pcJob := newDevice(t, cloudDir, "pc")
	pcSaves := pcJob.Slots[0].LocalPath
	write(t, pcSaves, "slot1.sav", "current")
	mustSync(t, pc, pcJob, ResolveAuto, ActionPush)

	content := []byte("from the backup")
	tmp := t.TempDir()
	obj := filepath.Join(tmp, "slot1.sav.zst")
	require.NoError(t, os.WriteFile(obj, zstdBytes(t, content), 0o644))
	name := "20260101T000000.000Z_old_cloud"
	dir := filepath.Join(cloudDir, "games", "game1", "backups")
	require.NoError(t, os.MkdirAll(dir, 0o755))
	m := &Manifest{Revision: "r0", Files: map[string]FileInfo{"saves/slot1.sav": {Size: int64(len(content)), Hash: sha(content)}}}
	require.NoError(t, writeZip(filepath.Join(dir, name+".zip"), []zipEntry{{name: "saves/slot1.sav.zst", src: obj, modTime: time.Now()}}, m))

	require.NoError(t, pc.RestoreBackup(context.Background(), pcJob, name))
	assert.Equal(t, string(content), read(t, pcSaves, "slot1.sav"))
	_, err := os.Stat(filepath.Join(pcSaves, "slot1.sav.zst"))
	assert.ErrorIs(t, err, os.ErrNotExist)
}

// A device whose saves are in a SAF folder pulls and pushes compressed saves.
func TestCompressedSAFSaveFolder(t *testing.T) {
	cloudDir := setupCloud(t)
	pc, pcJob := newDevice(t, cloudDir, "pc")
	phone, phoneJob, _, saves := newSAFDevice(t, cloudDir, "phone")
	pc.WriteFormat, phone.WriteFormat = FormatZstd, FormatZstd
	pcSaves := pcJob.Slots[0].LocalPath

	write(t, pcSaves, "slot1.sav", "pc-1")
	write(t, filepath.Join(pcSaves, "dir"), "a.sav", "nested")
	mustSync(t, pc, pcJob, ResolveAuto, ActionPush)
	mustSync(t, phone, phoneJob, ResolveAuto, ActionPull)
	assert.Equal(t, "pc-1", readFS(t, saves+"/slot1.sav"))
	assert.Equal(t, "nested", readFS(t, saves+"/dir/a.sav"))
	mustSync(t, phone, phoneJob, ResolveAuto, ActionNone)

	writeFS(t, saves+"/slot1.sav", "phone-1")
	mustSync(t, phone, phoneJob, ResolveAuto, ActionPush)
	assert.Equal(t, []string{"saves/dir/a.sav.zst", "saves/slot1.sav.zst"}, cloudObjects(t, cloudDir))
	mustSync(t, pc, pcJob, ResolveAuto, ActionPull)
	assert.Equal(t, "phone-1", read(t, pcSaves, "slot1.sav"))
}
