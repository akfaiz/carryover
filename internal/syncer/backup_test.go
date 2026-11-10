package syncer

import (
	"archive/zip"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func readZipEntry(t *testing.T, zipPath, name string) string {
	t.Helper()
	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		t.Fatal(err)
	}
	defer zr.Close()
	for _, f := range zr.File {
		if f.Name == name {
			r, err := f.Open()
			if err != nil {
				t.Fatal(err)
			}
			defer r.Close()
			b, err := io.ReadAll(r)
			if err != nil {
				t.Fatal(err)
			}
			return string(b)
		}
	}
	t.Fatalf("%s not in %s", name, zipPath)
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
	if err := os.Chtimes(filepath.Join(saves, "a.sav"), mt, mt); err != nil {
		t.Fatal(err)
	}
	mustSync(t, pc, job, ResolveAuto, ActionPush)
	write(t, saves, "a.sav", "a-2!")
	mustSync(t, pc, job, ResolveAuto, ActionPush) // backs up the cloud version

	backups, err := pc.ListBackups(context.Background(), "game1")
	if err != nil || len(backups) != 1 || backups[0].Kind != "cloud" {
		t.Fatalf("backups = %+v, %v", backups, err)
	}
	zipPath := filepath.Join(cloudDir, "games", "game1", "backups", backups[0].Name+".zip")
	if got := readZipEntry(t, zipPath, "saves/a.sav"); got != "a-1" {
		t.Fatalf("a.sav in backup = %q", got)
	}
	if got := readZipEntry(t, zipPath, "saves/sub/b.sav"); got != "b-1" {
		t.Fatalf("sub/b.sav in backup = %q", got)
	}
	if !strings.Contains(readZipEntry(t, zipPath, "manifest.json"), `"saves/a.sav"`) {
		t.Fatal("manifest missing from backup")
	}

	// Restoring brings back content and modification time.
	if err := pc.RestoreBackup(context.Background(), job, backups[0].Name); err != nil {
		t.Fatal(err)
	}
	if got := read(t, saves, "a.sav"); got != "a-1" {
		t.Fatalf("restored a.sav = %q", got)
	}
	fi, err := os.Stat(filepath.Join(saves, "a.sav"))
	if err != nil || !fi.ModTime().Equal(mt) {
		t.Fatalf("restored mtime = %v, want %v (%v)", fi.ModTime(), mt, err)
	}
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
	if err != nil || len(backups) != 1 || backups[0].Name != legacy || backups[0].DeviceName != "deck" {
		t.Fatalf("backups = %+v, %v", backups, err)
	}
	if err := pc.RestoreBackup(context.Background(), job, legacy); err != nil {
		t.Fatal(err)
	}
	if got := read(t, saves, "a.sav"); got != "old" {
		t.Fatalf("restored a.sav = %q", got)
	}
	if _, err := os.Stat(filepath.Join(saves, "extra.sav")); !os.IsNotExist(err) {
		t.Fatal("file missing from the backup wasn't removed on restore")
	}

	// Pruning removes the oldest first, whatever its format.
	pc.Retention = 1
	if err := pc.prune(context.Background(), "game1"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(cloudDir, "games", "game1", "backups", legacy)); !os.IsNotExist(err) {
		t.Fatalf("oldest (legacy) backup not pruned, err=%v", err)
	}
	if backups, _ := pc.ListBackups(context.Background(), "game1"); len(backups) != 1 {
		t.Fatalf("after prune: %+v", backups)
	}
}

// An archive entry escaping the save folder is refused.
func TestRestoreRejectsUnsafePaths(t *testing.T) {
	cloudDir := setupCloud(t)
	pc, job := newDevice(t, cloudDir, "pc")
	write(t, job.Slots[0].LocalPath, "a.sav", "x")
	mustSync(t, pc, job, ResolveAuto, ActionPush)

	name := "20200101T000000.000Z_evil_cloud"
	dir := filepath.Join(cloudDir, "games", "game1", "backups")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	f, err := os.Create(filepath.Join(dir, name+".zip"))
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	w, _ := zw.Create("saves/../../escaped.txt")
	_, _ = w.Write([]byte("pwned"))
	_ = zw.Close()
	_ = f.Close()

	err = pc.RestoreBackup(context.Background(), job, name)
	if err == nil || !strings.Contains(err.Error(), "unsafe path") {
		t.Fatalf("restore err = %v, want unsafe path", err)
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(job.Slots[0].LocalPath), "escaped.txt")); !os.IsNotExist(err) {
		t.Fatal("file was written outside the save folder")
	}
}
