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

	"github.com/akfaiz/carryover/internal/app/msg"
	"github.com/akfaiz/carryover/internal/cloudsync/cloud"
	"github.com/akfaiz/carryover/internal/cloudsync/localfs"
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

// backupCloud saves the current cloud version (listed: objects under
// current/, keyed "<slot>/<path>") as a backup before it's overwritten. The
// archive holds the files as they are, whatever the cloud format (m.Format):
// compressed ones are decompressed, so any version can restore it. Objects
// named for another format (left by an interrupted push) aren't part of this
// version and are left out.
func (e *Engine) backupCloud(ctx context.Context, id string, m *Manifest, listed map[string]int64) error {
	name := e.backupName("cloud")
	e.emit(id, "backup", msg.SyncBackingUpCloud.Msg(), cloud.Progress{})
	tmp, err := os.MkdirTemp("", "carryover-backup-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)
	inCloud, _ := cloudView(listed, m.Format)
	files, total := sortedFiles(inCloud)
	objects := storedNames(files, m.Format)
	if err := e.Client.Download(ctx, currentDir(id), filepath.Join(tmp, "files"), objects, total, e.progress(id, "backup", msg.SyncBackingUpCloud.Msg())); err != nil {
		return err
	}
	entries := make([]zipEntry, 0, len(files))
	for i, f := range files {
		en := zipEntry{name: f, src: filepath.Join(tmp, "files", filepath.FromSlash(objects[i])), modTime: time.Now()}
		if m.Format == FormatZstd {
			plain := filepath.Join(tmp, "plain", filepath.FromSlash(f))
			if err := decompressFile(en.src, plain); err == nil {
				en.src = plain
			} else {
				// Keep what the cloud has: a backup must not fail on one file.
				en.name = objects[i]
			}
		}
		if fi, ok := m.Files[en.name]; ok {
			en.modTime = fi.ModTime
		}
		entries = append(entries, en)
	}
	return e.storeBackup(ctx, id, name, tmp, entries, m)
}

// backupLocal saves this device's saves (m: the local scan) as a backup of the
// given kind ("local": before they're overwritten; "manual": asked for by the
// user) and returns its name.
func (e *Engine) backupLocal(ctx context.Context, job Job, m *Manifest, kind string) (string, error) {
	name := e.backupName(kind)
	e.emit(job.GameID, "backup", msg.SyncBackingUpLocal.Msg(), cloud.Progress{})
	tmp, err := os.MkdirTemp("", "carryover-backup-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(tmp)
	return name, e.storeBackup(ctx, job.GameID, name, tmp, localEntries(job, m), e.deviceManifest(m))
}

// localEntries lists this device's save files (m: the local scan) as archive
// entries, sorted by name.
func localEntries(job Job, m *Manifest) []zipEntry {
	var entries []zipEntry
	for _, s := range job.Slots {
		for rel, f := range slotFiles(m, s.Key, nil) {
			entries = append(entries, zipEntry{name: s.Key + "/" + rel, src: localfs.Join(s.LocalPath, rel), modTime: f.ModTime})
		}
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].name < entries[j].name })
	return entries
}

// storeBackup zips entries and the manifest into tmp and uploads the archive.
// A failed or cancelled upload leaves nothing behind. Callers prune old
// backups afterwards.
func (e *Engine) storeBackup(ctx context.Context, id, name, tmp string, entries []zipEntry, m *Manifest) error {
	zipName := name + backupExt
	if err := writeZip(filepath.Join(tmp, zipName), entries, m); err != nil {
		return msg.BackupWriteFailed.Wrap(err)
	}
	fi, err := os.Stat(filepath.Join(tmp, zipName))
	if err != nil {
		return err
	}
	if err := e.Client.Upload(ctx, tmp, backupsDir(id), []string{zipName}, fi.Size(), e.progress(id, "backup", msg.SyncUploadingBackup.Msg())); err != nil {
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
	src, err := localfs.Open(en.src)
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

// prune deletes the oldest backups beyond the retention. Pinned backups are
// never deleted and don't count towards it. Retention 0 means backups are off
// and nothing is pruned.
func (e *Engine) prune(ctx context.Context, id string) error {
	if e.Retention <= 0 {
		return nil
	}
	names, isZip, err := e.backupEntries(ctx, id)
	if err != nil {
		return err
	}
	pins, err := e.readPins(ctx, id)
	if err != nil {
		return err
	}
	names = slices.DeleteFunc(names, func(n string) bool { return pins[n] })
	for len(names) > e.Retention {
		if err := e.removeBackup(ctx, id, names[0], isZip[names[0]]); err != nil {
			return err
		}
		names = names[1:]
	}
	return nil
}

// removeBackup deletes one backup: an archive, or an older folder backup.
func (e *Engine) removeBackup(ctx context.Context, id, name string, isZip bool) error {
	p := backupsDir(id) + "/" + name
	if isZip {
		return e.Client.Delete(ctx, p+backupExt)
	}
	return e.Client.Purge(ctx, p)
}

// DeleteBackup permanently deletes a backup (pinned or not) for every device,
// or a safety copy on this device.
func (e *Engine) DeleteBackup(ctx context.Context, gameID, name string) error {
	if strings.ContainsAny(name, `/\`) || name == "" || name == "." || name == ".." {
		return msg.BackupInvalidName.Err()
	}
	if e.isSafetyCopy(gameID, name) {
		return os.Remove(e.safetyPath(gameID, name))
	}
	names, isZip, err := e.backupEntries(ctx, gameID)
	if err != nil {
		return err
	}
	if !slices.Contains(names, name) {
		return msg.BackupNotFound.Err("Name", name)
	}
	pins, err := e.readPins(ctx, gameID)
	if err != nil {
		return err
	}
	if err := e.removeBackup(ctx, gameID, name, isZip[name]); err != nil {
		return err
	}
	if !pins[name] {
		return nil
	}
	list := []string{} // the pins of the backups that remain
	for _, n := range names {
		if n != name && pins[n] {
			list = append(list, n)
		}
	}
	return e.Client.WriteJSON(ctx, pinsPath(gameID), list)
}

// pinsPath holds the names of a game's pinned backups.
func pinsPath(id string) string { return gameDir(id) + "/pinned.json" }

func (e *Engine) readPins(ctx context.Context, id string) (map[string]bool, error) {
	var names []string
	if _, err := e.Client.ReadJSON(ctx, pinsPath(id), &names); err != nil {
		return nil, msg.BackupPinsFailed.Wrap(err)
	}
	pins := make(map[string]bool, len(names))
	for _, n := range names {
		pins[n] = true
	}
	return pins, nil
}

// SetBackupPinned pins or unpins a backup. Pruning never deletes a pinned one.
func (e *Engine) SetBackupPinned(ctx context.Context, gameID, name string, pinned bool) error {
	if e.isSafetyCopy(gameID, name) {
		return msg.BackupPinLocal.Err()
	}
	names, _, err := e.backupEntries(ctx, gameID)
	if err != nil {
		return err
	}
	if !slices.Contains(names, name) {
		return msg.BackupNotFound.Err("Name", name)
	}
	pins, err := e.readPins(ctx, gameID)
	if err != nil {
		return err
	}
	if pinned {
		pins[name] = true
	} else {
		delete(pins, name)
	}
	// Keep only backups that still exist.
	list := []string{}
	for _, n := range names {
		if pins[n] {
			list = append(list, n)
		}
	}
	return e.Client.WriteJSON(ctx, pinsPath(gameID), list)
}

// BackupNow stores a backup of this device's current saves, whatever the
// retention setting, and pins it: a backup made on purpose is kept until
// unpinned. It returns the backup's name.
func (e *Engine) BackupNow(ctx context.Context, job Job) (string, error) {
	if err := validateJob(job); err != nil {
		return "", err
	}
	st, err := e.States.Load(job.StateID())
	if err != nil {
		return "", err
	}
	local, err := ScanLocal(ctx, job.Slots, st.LastSynced)
	if err != nil {
		return "", msg.SyncScanFailed.Wrap(err)
	}
	if len(local.Files) == 0 {
		return "", msg.BackupNothing.Err()
	}
	name, err := e.backupLocal(ctx, job, local, "manual")
	if err != nil {
		return "", msg.BackupLocalFailed.Wrap(err)
	}
	if err := e.SetBackupPinned(ctx, job.GameID, name, true); err != nil {
		return name, msg.BackupPinFailed.Wrap(err)
	}
	_ = e.prune(ctx, job.GameID) // the backup is pinned, so it stays
	return name, nil
}

// Backup describes a stored backup.
type Backup struct {
	Name       string    `json:"name"`
	Time       time.Time `json:"time"`
	DeviceName string    `json:"deviceName"`
	// Kind is "local" (taken before a download), "cloud" (taken before an
	// upload), "manual" (taken on request) or "safety" (a safety copy).
	Kind   string `json:"kind"`
	Pinned bool   `json:"pinned"`
	// Local marks a safety copy, stored only on this device.
	Local bool `json:"local"`
}

// ListBackups returns a game's cloud backups and this device's safety
// copies, newest first.
func (e *Engine) ListBackups(ctx context.Context, gameID string) ([]Backup, error) {
	names, _, err := e.backupEntries(ctx, gameID)
	if err != nil {
		return nil, err
	}
	pins, err := e.readPins(ctx, gameID)
	if err != nil {
		return nil, err
	}
	safety, err := e.safetyNames(gameID)
	if err != nil {
		return nil, err
	}
	isLocal := map[string]bool{}
	for _, n := range safety {
		if !slices.Contains(names, n) {
			isLocal[n] = true
			names = append(names, n)
		}
	}
	sort.Strings(names)
	var out []Backup
	for _, n := range slices.Backward(names) {
		b := Backup{Name: n, Pinned: pins[n], Local: isLocal[n]}
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
		return msg.BackupInvalidName.Err()
	}
	names, isZip, err := e.backupEntries(ctx, job.GameID)
	if err != nil {
		return err
	}
	inCloud := slices.Contains(names, name)
	if !inCloud && !e.isSafetyCopy(job.GameID, name) {
		return msg.BackupNotFound.Err("Name", name)
	}
	st, err := e.States.Load(job.StateID())
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
		if _, err := e.backupLocal(ctx, job, local, "local"); err != nil {
			return msg.BackupLocalFailed.Wrap(err)
		}
		if e.Retention > 0 {
			if err := e.prune(ctx, job.GameID); err != nil {
				return err
			}
		}
	}
	switch {
	case !inCloud:
		err = restoreZipFile(ctx, job, e.safetyPath(job.GameID, name), local)
	case isZip[name]:
		err = e.restoreZip(ctx, job, name, local)
	default:
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
	if err := e.Client.Download(ctx, backupsDir(job.GameID), tmp, []string{zipName}, 0, e.progress(job.GameID, "download", msg.SyncDownloadBackup.Msg())); err != nil {
		return err
	}
	return restoreZipFile(ctx, job, filepath.Join(tmp, zipName), local)
}

// restoreZipFile restores a backup archive on this device (see restoreZip).
func restoreZipFile(ctx context.Context, job Job, path string, local *Manifest) error {
	zr, err := zip.OpenReader(path)
	if err != nil {
		return msg.BackupOpenFailed.Wrap(err)
	}
	defer zr.Close()
	known := zipManifestFiles(zr)
	for _, s := range job.Slots {
		flt, err := s.Filters.NewFilter()
		if err != nil {
			return err
		}
		inBackup := map[string]bool{}
		for _, zf := range zr.File {
			entry, compressed := zf.Name, false
			// A version that predates compressed cloud saves backs up their
			// objects as they are: "<file>.zst" where its manifest has <file>.
			if plain, ok := strings.CutSuffix(entry, zstdExt); ok && known[plain] && !known[entry] {
				entry, compressed = plain, true
			}
			rel, ok := strings.CutPrefix(entry, s.Key+"/")
			if !ok || strings.HasSuffix(entry, "/") || !flt.Include(rel, int64(zf.UncompressedSize64), time.Time{}, nil) {
				continue
			}
			if err := ctx.Err(); err != nil {
				return err
			}
			if !filepath.IsLocal(filepath.FromSlash(rel)) {
				return msg.BackupUnsafePath.Err("Path", zf.Name)
			}
			if err := extractFile(zf, localfs.Join(s.LocalPath, rel), compressed); err != nil {
				return msg.BackupRestoreFailed.Wrap(err, "Name", zf.Name)
			}
			inBackup[rel] = true
		}
		if err := removeMissing(s, local, inBackup); err != nil {
			return err
		}
	}
	return nil
}

// zipManifestFiles returns the file keys of a backup archive's manifest (nil
// without a readable one).
func zipManifestFiles(zr *zip.ReadCloser) map[string]bool {
	for _, zf := range zr.File {
		if zf.Name != backupManifestName {
			continue
		}
		r, err := zf.Open()
		if err != nil {
			return nil
		}
		defer r.Close()
		var m Manifest
		if err := json.NewDecoder(r).Decode(&m); err != nil {
			return nil
		}
		out := make(map[string]bool, len(m.Files))
		for k := range m.Files {
			out[k] = true
		}
		return out
	}
	return nil
}

// openEntry opens an archive entry, decompressing it when it holds a
// zstd-compressed cloud object.
func openEntry(zf *zip.File, compressed bool) (io.ReadCloser, error) {
	r, err := zf.Open()
	if err != nil || !compressed {
		return r, err
	}
	dec, err := newDecoder(r)
	if err != nil {
		r.Close()
		return nil, err
	}
	return struct {
		io.Reader
		io.Closer
	}{dec, closerFunc(func() error { dec.Close(); return r.Close() })}, nil
}

type closerFunc func() error

func (f closerFunc) Close() error { return f() }

func extractFile(zf *zip.File, dst string, compressed bool) error {
	if localfs.IsFS(dst) {
		return extractFS(zf, dst, compressed)
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	r, err := openEntry(zf, compressed)
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

// extractFS is extractFile for a save folder without a file path (localfs):
// the file is written in place, as such folders have no reliable rename, and
// keeps the time it was written, as their modification times can't be set.
// A restore that fails halfway can be undone from the safety copy or backup
// taken before it.
func extractFS(zf *zip.File, dst string, compressed bool) error {
	r, err := openEntry(zf, compressed)
	if err != nil {
		return err
	}
	defer r.Close()
	w, err := localfs.Create(dst)
	if err != nil {
		return err
	}
	if _, err := io.Copy(w, r); err != nil {
		_ = w.Close()
		return err
	}
	return w.Close()
}

// restoreFolder restores an older folder backup.
func (e *Engine) restoreFolder(ctx context.Context, job Job, name string, local *Manifest) error {
	for _, s := range job.Slots {
		if err := localfs.MkdirAll(s.LocalPath); err != nil {
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
		if err := e.Client.Download(ctx, dir, s.LocalPath, files, total, e.progress(job.GameID, "download", msg.SyncRestoring.Msg("Slot", s.Key))); err != nil {
			return msg.BackupRestoreFailed.Wrap(err, "Name", s.Key)
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
		if err := localfs.Remove(localfs.Join(s.LocalPath, rel)); err != nil && !errors.Is(err, os.ErrNotExist) {
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
