package syncer

import (
	"archive/zip"
	"compress/flate"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/akfaiz/carryover/internal/cloud"
)

// Backups live in games/<id>/backups/<time>_<device>_<kind>.zip, holding
// "manifest.json" and the saves as "<slot>/<path>". One file per backup keeps
// them fast to write: creating folders and copying files server-side are
// slow on some providers (OneDrive: ~4s per folder, ~9s per copy).
//
// Older versions stored each backup as a folder with the same layout; those
// are still listed, restored and pruned.

// backupTimeLayout is the timestamp prefix of backup names; it sorts chronologically.
const backupTimeLayout = "20060102T150405.000Z"

const (
	backupExt          = ".zip"
	backupManifestName = "manifest.json"
)

var unsafeChars = regexp.MustCompile(`[^A-Za-z0-9._-]+`)

func (e *Engine) backupName(kind string) string {
	dev := strings.Trim(unsafeChars.ReplaceAllString(e.Device.Name, "_"), "_")
	if dev == "" {
		dev = "device"
	}
	return time.Now().UTC().Format(backupTimeLayout) + "_" + dev + "_" + kind
}

// zipEntry is a file to put in a backup archive.
type zipEntry struct {
	name    string // "<slot>/<path>"
	src     string // local file
	modTime time.Time
}

// backupCloud saves the current cloud version (listed: files under current/,
// keyed "<slot>/<path>") as a backup before it's overwritten.
func (e *Engine) backupCloud(ctx context.Context, id string, m *Manifest, listed map[string]int64) error {
	name := e.backupName("cloud")
	e.emit(id, "backup", "Backing up cloud saves", cloud.Progress{})
	tmp, err := os.MkdirTemp("", "carryover-backup-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)
	files, total := sortedFiles(listed)
	if err := e.Client.Download(ctx, currentDir(id), filepath.Join(tmp, "files"), files, total, e.progress(id, "backup", "Backing up cloud saves")); err != nil {
		return err
	}
	entries := make([]zipEntry, 0, len(files))
	for _, f := range files {
		mt := time.Now()
		if fi, ok := m.Files[f]; ok {
			mt = fi.ModTime
		}
		entries = append(entries, zipEntry{name: f, src: filepath.Join(tmp, "files", filepath.FromSlash(f)), modTime: mt})
	}
	return e.storeBackup(ctx, id, name, tmp, entries, m)
}

// backupLocal saves this device's saves (m: the local scan) as a backup
// before they're overwritten.
func (e *Engine) backupLocal(ctx context.Context, job Job, m *Manifest) error {
	name := e.backupName("local")
	e.emit(job.GameID, "backup", "Backing up local saves", cloud.Progress{})
	tmp, err := os.MkdirTemp("", "carryover-backup-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)
	var entries []zipEntry
	for _, s := range job.Slots {
		for rel, f := range slotFiles(m, s.Key, nil) {
			entries = append(entries, zipEntry{name: s.Key + "/" + rel, src: filepath.Join(s.LocalPath, filepath.FromSlash(rel)), modTime: f.ModTime})
		}
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].name < entries[j].name })
	bm := *m
	bm.Device, bm.DeviceName, bm.Time = e.Device.ID, e.Device.Name, time.Now().UTC()
	return e.storeBackup(ctx, job.GameID, name, tmp, entries, &bm)
}

// storeBackup zips entries and the manifest into tmp and uploads the archive.
// A failed or cancelled upload leaves nothing behind. Callers prune old
// backups afterwards.
func (e *Engine) storeBackup(ctx context.Context, id, name, tmp string, entries []zipEntry, m *Manifest) error {
	zipName := name + backupExt
	if err := writeZip(filepath.Join(tmp, zipName), entries, m); err != nil {
		return fmt.Errorf("write backup archive: %w", err)
	}
	fi, err := os.Stat(filepath.Join(tmp, zipName))
	if err != nil {
		return err
	}
	if err := e.Client.Upload(ctx, tmp, backupsDir(id), []string{zipName}, fi.Size(), e.progress(id, "backup", "Uploading backup")); err != nil {
		e.discardBackup(id, name)
		return err
	}
	return nil
}

func writeZip(dst string, entries []zipEntry, m *Manifest) (err error) {
	f, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer func() {
		if cerr := f.Close(); err == nil {
			err = cerr
		}
	}()
	zw := zip.NewWriter(f)
	// Speed over size: saves are usually small, and this runs before every
	// overwrite.
	zw.RegisterCompressor(zip.Deflate, func(w io.Writer) (io.WriteCloser, error) {
		return flate.NewWriter(w, flate.BestSpeed)
	})
	mb, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	w, err := zw.CreateHeader(&zip.FileHeader{Name: backupManifestName, Method: zip.Deflate, Modified: time.Now()})
	if err != nil {
		return err
	}
	if _, err := w.Write(mb); err != nil {
		return err
	}
	for _, en := range entries {
		if err := addZipFile(zw, en); err != nil {
			return fmt.Errorf("%s: %w", en.name, err)
		}
	}
	return zw.Close()
}

func addZipFile(zw *zip.Writer, en zipEntry) error {
	src, err := os.Open(en.src)
	if err != nil {
		return err
	}
	defer src.Close()
	w, err := zw.CreateHeader(&zip.FileHeader{Name: en.name, Method: zip.Deflate, Modified: en.modTime})
	if err != nil {
		return err
	}
	_, err = io.Copy(w, src)
	return err
}

// discardBackup deletes a backup that couldn't be completed (e.g. the sync
// was cancelled), so a partial one is never offered for restore. It uses its
// own context because the sync's context may be the one that was cancelled.
func (e *Engine) discardBackup(gameID, name string) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	_ = e.Client.Delete(ctx, backupsDir(gameID)+"/"+name+backupExt)
}

// backupEntries lists a game's backups by name (sorted, oldest first) and
// whether each is an archive (true) or an older folder backup (false).
func (e *Engine) backupEntries(ctx context.Context, gameID string) ([]string, map[string]bool, error) {
	dirs, files, err := e.Client.List(ctx, backupsDir(gameID))
	if err != nil {
		return nil, nil, err
	}
	isZip := map[string]bool{}
	for _, d := range dirs {
		isZip[d] = false
	}
	for _, f := range files {
		if n, ok := strings.CutSuffix(f, backupExt); ok {
			isZip[n] = true
		}
	}
	names := make([]string, 0, len(isZip))
	for n := range isZip {
		names = append(names, n)
	}
	sort.Strings(names) // timestamp prefix sorts chronologically
	return names, isZip, nil
}

// pruneInBackground deletes backups beyond the retention while the caller
// carries on (it never touches current saves); the returned func waits for it.
// Errors are ignored: old backups are pruned again after the next backup.
func (e *Engine) pruneInBackground(ctx context.Context, id string) (wait func()) {
	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = e.prune(ctx, id)
	}()
	return func() { <-done }
}

func (e *Engine) prune(ctx context.Context, id string) error {
	names, isZip, err := e.backupEntries(ctx, id)
	if err != nil {
		return err
	}
	for len(names) > e.Retention {
		oldest := backupsDir(id) + "/" + names[0]
		if isZip[names[0]] {
			err = e.Client.Delete(ctx, oldest+backupExt)
		} else {
			err = e.Client.Purge(ctx, oldest)
		}
		if err != nil {
			return err
		}
		names = names[1:]
	}
	return nil
}

// Backup describes a stored backup.
type Backup struct {
	Name       string    `json:"name"`
	Time       time.Time `json:"time"`
	DeviceName string    `json:"deviceName"`
	Kind       string    `json:"kind"` // "local" (taken before a download) or "cloud" (taken before an upload)
}

// ListBackups returns backups for a game, newest first.
func (e *Engine) ListBackups(ctx context.Context, gameID string) ([]Backup, error) {
	names, _, err := e.backupEntries(ctx, gameID)
	if err != nil {
		return nil, err
	}
	var out []Backup
	for _, n := range slices.Backward(names) {
		b := Backup{Name: n}
		parts := strings.Split(n, "_")
		if len(parts) >= 3 {
			b.Time, _ = time.Parse(backupTimeLayout, parts[0])
			b.Kind = parts[len(parts)-1]
			b.DeviceName = strings.Join(parts[1:len(parts)-1], "_")
		}
		out = append(out, b)
	}
	return out, nil
}

// RestoreBackup replaces the local saves with a backup and uploads them as the
// new cloud version. The current local saves are backed up first.
func (e *Engine) RestoreBackup(ctx context.Context, job Job, name string) error {
	if err := validateJob(job); err != nil {
		return err
	}
	if strings.ContainsAny(name, `/\`) || name == "" || name == "." || name == ".." {
		return errors.New("invalid backup name")
	}
	names, isZip, err := e.backupEntries(ctx, job.GameID)
	if err != nil {
		return err
	}
	if !slices.Contains(names, name) {
		return fmt.Errorf("backup %s not found", name)
	}
	st, err := e.States.Load(job.GameID)
	if err != nil {
		return err
	}
	local, err := ScanLocal(ctx, job.Slots, st.LastSynced)
	if err != nil {
		return err
	}
	// A restore overwrites local saves, so it always backs them up first, even
	// when backups are off (Retention 0). Pruning is skipped then: it would
	// delete this very backup.
	if len(local.Files) > 0 {
		if err := e.backupLocal(ctx, job, local); err != nil {
			return fmt.Errorf("backup local saves: %w", err)
		}
		if e.Retention > 0 {
			if err := e.prune(ctx, job.GameID); err != nil {
				return err
			}
		}
	}
	if isZip[name] {
		err = e.restoreZip(ctx, job, name, local)
	} else {
		err = e.restoreFolder(ctx, job, name, local)
	}
	if err != nil {
		return err
	}
	_, _, err = e.Sync(ctx, job, ResolveLocal)
	return err
}

// restoreZip makes each slot's local folder match the archive: its files are
// written, and local files it lacks are deleted (files the slot's filters
// exclude are left alone on both sides).
func (e *Engine) restoreZip(ctx context.Context, job Job, name string, local *Manifest) error {
	tmp, err := os.MkdirTemp("", "carryover-restore-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)
	zipName := name + backupExt
	if err := e.Client.Download(ctx, backupsDir(job.GameID), tmp, []string{zipName}, 0, e.progress(job.GameID, "download", "Downloading backup")); err != nil {
		return err
	}
	zr, err := zip.OpenReader(filepath.Join(tmp, zipName))
	if err != nil {
		return fmt.Errorf("open backup archive: %w", err)
	}
	defer zr.Close()
	for _, s := range job.Slots {
		flt, err := s.Filters.NewFilter()
		if err != nil {
			return err
		}
		inBackup := map[string]bool{}
		for _, zf := range zr.File {
			rel, ok := strings.CutPrefix(zf.Name, s.Key+"/")
			if !ok || strings.HasSuffix(zf.Name, "/") || !flt.Include(rel, int64(zf.UncompressedSize64), time.Time{}, nil) {
				continue
			}
			if err := ctx.Err(); err != nil {
				return err
			}
			if !filepath.IsLocal(filepath.FromSlash(rel)) {
				return fmt.Errorf("backup contains an unsafe path: %s", zf.Name)
			}
			if err := extractFile(zf, filepath.Join(s.LocalPath, filepath.FromSlash(rel))); err != nil {
				return fmt.Errorf("restore %s: %w", zf.Name, err)
			}
			inBackup[rel] = true
		}
		if err := removeMissing(s, local, inBackup); err != nil {
			return err
		}
	}
	return nil
}

func extractFile(zf *zip.File, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	r, err := zf.Open()
	if err != nil {
		return err
	}
	defer r.Close()
	tmp := dst + ".carryover-restore"
	w, err := os.Create(tmp)
	if err != nil {
		return err
	}
	if _, err := io.Copy(w, r); err != nil {
		w.Close()
		os.Remove(tmp)
		return err
	}
	if err := w.Close(); err != nil {
		os.Remove(tmp)
		return err
	}
	if err := os.Rename(tmp, dst); err != nil {
		os.Remove(tmp)
		return err
	}
	if !zf.Modified.IsZero() {
		_ = os.Chtimes(dst, zf.Modified, zf.Modified)
	}
	return nil
}

// restoreFolder restores an older folder backup.
func (e *Engine) restoreFolder(ctx context.Context, job Job, name string, local *Manifest) error {
	for _, s := range job.Slots {
		if err := os.MkdirAll(s.LocalPath, 0o755); err != nil {
			return err
		}
		flt, err := s.Filters.NewFilter()
		if err != nil {
			return err
		}
		dir := backupsDir(job.GameID) + "/" + name + "/" + s.Key
		listed, err := e.Client.ListFiles(ctx, dir)
		if err != nil {
			return err
		}
		inBackup := map[string]bool{}
		for rel, size := range listed {
			if flt.Include(rel, size, time.Time{}, nil) {
				inBackup[rel] = true
			} else {
				delete(listed, rel)
			}
		}
		files, total := sortedFiles(listed)
		if err := e.Client.Download(ctx, dir, s.LocalPath, files, total, e.progress(job.GameID, "download", "Restoring "+s.Key)); err != nil {
			return fmt.Errorf("restore %s: %w", s.Key, err)
		}
		if err := removeMissing(s, local, inBackup); err != nil {
			return err
		}
	}
	return nil
}

// removeMissing deletes the slot's local files (from the scan) that the
// restored backup doesn't have.
func removeMissing(s Slot, local *Manifest, keep map[string]bool) error {
	for rel := range slotFiles(local, s.Key, nil) {
		if keep[rel] {
			continue
		}
		if err := os.Remove(filepath.Join(s.LocalPath, filepath.FromSlash(rel))); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	return nil
}

// sortedFiles returns the names of a path→size map, sorted, and their total size.
func sortedFiles(sizes map[string]int64) ([]string, int64) {
	files := make([]string, 0, len(sizes))
	var total int64
	for f, n := range sizes {
		files = append(files, f)
		total += n
	}
	sort.Strings(files)
	return files, total
}

// cloudSlotFiles returns the files of one slot from a listing of current/
// (keys "<slot>/<path>"), keyed by path relative to the slot.
func cloudSlotFiles(listed map[string]int64, slot string) map[string]int64 {
	out := map[string]int64{}
	for k, n := range listed {
		if rel, ok := strings.CutPrefix(k, slot+"/"); ok {
			out[rel] = n
		}
	}
	return out
}
