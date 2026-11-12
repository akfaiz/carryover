package syncer

import (
	"archive/zip"
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func readZipEntry(t *testing.T, zipPath, name string) string {
	t.Helper()
	zr, err := zip.OpenReader(zipPath)
	require.NoError(t, err)
	defer zr.Close()
	for _, f := range zr.File {
		if f.Name == name {
			r, err := f.Open()
			require.NoError(t, err)
			defer r.Close()
			b, err := io.ReadAll(r)
			require.NoError(t, err)
			return string(b)
		}
	}
	require.Failf(t, "entry missing", "%s not in %s", name, zipPath)
	return ""
}

// A backup is one archive with the manifest and the saves, keeping mtimes.
func TestBackupArchive(t *testing.T) {
	cloudDir := setupCloud(t)
	pc, job := newDevice(t, cloudDir, "pc")
	saves := job.Slots[0].LocalPath
	write(t, filepath.Join(saves, "sub"), "b.sav", "b-1")
	write(t, saves, "a.sav", "a-1")
	mt := time.Date(2024, 5, 6, 7, 8, 9, 0, time.UTC)
	require.NoError(t, os.Chtimes(filepath.Join(saves, "a.sav"), mt, mt))
	mustSync(t, pc, job, ResolveAuto, ActionPush)
	write(t, saves, "a.sav", "a-2!")
	mustSync(t, pc, job, ResolveAuto, ActionPush) // backs up the cloud version

	backups, err := pc.ListBackups(context.Background(), "game1")
	require.NoError(t, err)
	require.Len(t, backups, 1)
	require.Equal(t, "cloud", backups[0].Kind)
	zipPath := filepath.Join(cloudDir, "games", "game1", "backups", backups[0].Name+".zip")
	require.Equal(t, "a-1", readZipEntry(t, zipPath, "saves/a.sav"))
	require.Equal(t, "b-1", readZipEntry(t, zipPath, "saves/sub/b.sav"))
	require.Contains(t, readZipEntry(t, zipPath, "manifest.json"), `"saves/a.sav"`, "manifest missing from backup")

	// Restoring brings back content and modification time.
	require.NoError(t, pc.RestoreBackup(context.Background(), job, backups[0].Name))
	require.Equal(t, "a-1", read(t, saves, "a.sav"))
	fi, err := os.Stat(filepath.Join(saves, "a.sav"))
	require.NoError(t, err)
	require.True(t, fi.ModTime().Equal(mt), "restored mtime = %v, want %v", fi.ModTime(), mt)
}

// Folder backups made by older versions are still listed, restored and pruned.
func TestLegacyFolderBackups(t *testing.T) {
	cloudDir := setupCloud(t)
	pc, job := newDevice(t, cloudDir, "pc")
	saves := job.Slots[0].LocalPath
	legacy := "20200101T000000.000Z_deck_local"
	write(t, filepath.Join(cloudDir, "games", "game1", "backups", legacy, "saves"), "a.sav", "old")
	write(t, saves, "a.sav", "new")
	write(t, saves, "extra.sav", "x")
	mustSync(t, pc, job, ResolveAuto, ActionPush)

	backups, err := pc.ListBackups(context.Background(), "game1")
	require.NoError(t, err)
	require.Len(t, backups, 1)
	require.Equal(t, legacy, backups[0].Name)
	require.Equal(t, "deck", backups[0].DeviceName)
	require.NoError(t, pc.RestoreBackup(context.Background(), job, legacy))
	require.Equal(t, "old", read(t, saves, "a.sav"))
	require.NoFileExists(t, filepath.Join(saves, "extra.sav"), "file missing from the backup wasn't removed on restore")

	// Pruning removes the oldest first, whatever its format.
	pc.Retention = 1
	require.NoError(t, pc.prune(context.Background(), "game1"))
	require.NoDirExists(t, filepath.Join(cloudDir, "games", "game1", "backups", legacy), "oldest (legacy) backup not pruned")
	backups, _ = pc.ListBackups(context.Background(), "game1")
	require.Len(t, backups, 1, "after prune")
}

// An archive entry escaping the save folder is refused.
func TestRestoreRejectsUnsafePaths(t *testing.T) {
	cloudDir := setupCloud(t)
	pc, job := newDevice(t, cloudDir, "pc")
	write(t, job.Slots[0].LocalPath, "a.sav", "x")
	mustSync(t, pc, job, ResolveAuto, ActionPush)

	name := "20200101T000000.000Z_evil_cloud"
	dir := filepath.Join(cloudDir, "games", "game1", "backups")
	require.NoError(t, os.MkdirAll(dir, 0o755))
	f, err := os.Create(filepath.Join(dir, name+".zip"))
	require.NoError(t, err)
	zw := zip.NewWriter(f)
	w, _ := zw.Create("saves/../../escaped.txt")
	_, _ = w.Write([]byte("pwned"))
	_ = zw.Close()
	_ = f.Close()

	err = pc.RestoreBackup(context.Background(), job, name)
	require.ErrorContains(t, err, "unsafe path")
	require.NoFileExists(t, filepath.Join(filepath.Dir(job.Slots[0].LocalPath), "escaped.txt"), "file was written outside the save folder")
}
