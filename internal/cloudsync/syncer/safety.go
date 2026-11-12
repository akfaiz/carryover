package syncer

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/akfaiz/carryover/internal/cloudsync/cloud"
)

// Safety copies: when cloud backups are off (Retention 0) and a download is
// about to replace this device's saves, the saves are first zipped to
// <SafetyDir>/<game>/<time>_<device>_safety.zip, in the same format as a cloud
// backup. Writing a local zip costs no upload, so this is always on; the
// newest SafetyKeep copies per game are kept. They are listed and restored
// with the cloud backups (Backup.Local).

// SafetyKeep is how many safety copies are kept per game.
const SafetyKeep = 5

// kindSafety is the backup kind of a safety copy.
const kindSafety = "safety"

func (e *Engine) safetyPath(gameID, name string) string {
	return filepath.Join(SafetyGameDir(e.SafetyDir, gameID), name+backupExt)
}

// SafetyGameDir is the folder holding a game's safety copies under root
// (an Engine's SafetyDir).
func SafetyGameDir(root, gameID string) string { return filepath.Join(root, gameID) }

// HasSafetyCopies reports whether a game has any safety copies under root.
func HasSafetyCopies(root, gameID string) bool {
	names, err := (&Engine{SafetyDir: root}).safetyNames(gameID)
	return err == nil && len(names) > 0
}

// safetyCopy zips this device's saves (m: the local scan) into the game's
// safety folder and prunes the oldest copies. It does nothing without a
// SafetyDir.
func (e *Engine) safetyCopy(job Job, m *Manifest) error {
	if e.SafetyDir == "" {
		return nil
	}
	e.emit(job.GameID, "backup", "Keeping a copy of this device's saves", cloud.Progress{})
	dst := e.safetyPath(job.GameID, e.backupName(kindSafety))
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	// Written under a temporary name so a cut-short copy is never listed.
	tmp := dst + ".tmp"
	if err := writeZip(tmp, localEntries(job, m), e.deviceManifest(m)); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	if err := os.Rename(tmp, dst); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	e.pruneSafety(job.GameID)
	return nil
}

// safetyNames lists a game's safety copies by name, oldest first.
func (e *Engine) safetyNames(gameID string) ([]string, error) {
	if e.SafetyDir == "" {
		return nil, nil
	}
	entries, err := os.ReadDir(SafetyGameDir(e.SafetyDir, gameID))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var names []string
	for _, en := range entries {
		if n, ok := strings.CutSuffix(en.Name(), backupExt); ok && en.Type().IsRegular() {
			names = append(names, n)
		}
	}
	sort.Strings(names) // timestamp prefix sorts chronologically
	return names, nil
}

func (e *Engine) isSafetyCopy(gameID, name string) bool {
	if strings.ContainsAny(name, `/\`) || name == "" || name == "." || name == ".." {
		return false
	}
	names, err := e.safetyNames(gameID)
	return err == nil && slices.Contains(names, name)
}

// pruneSafety deletes the oldest safety copies beyond SafetyKeep. Errors are
// ignored: they are pruned again after the next copy.
func (e *Engine) pruneSafety(gameID string) {
	names, err := e.safetyNames(gameID)
	if err != nil {
		return
	}
	for len(names) > SafetyKeep {
		_ = os.Remove(e.safetyPath(gameID, names[0]))
		names = names[1:]
	}
}

// LocalChanges reports whether this device's saves changed since its last
// successful sync (last), without touching the cloud: e.g. the game was
// played without Carryover. A game never synced here reports false. prev, an
// earlier result of this check, saves re-hashing unchanged files; the scan is
// returned for the next call.
func LocalChanges(ctx context.Context, slots []Slot, last, prev *Manifest) (bool, *Manifest, error) {
	if last == nil {
		return false, nil, nil
	}
	if prev == nil {
		prev = last
	}
	local, err := ScanLocal(ctx, slots, prev)
	if err != nil {
		return false, nil, fmt.Errorf("scan local saves: %w", err)
	}
	return !SameFiles(local, last), local, nil
}

// deviceManifest is m stamped with this device and the current time, as
// stored in a backup of this device's saves.
func (e *Engine) deviceManifest(m *Manifest) *Manifest {
	bm := *m
	bm.Device, bm.DeviceName, bm.Time = e.Device.ID, e.Device.Name, time.Now().UTC()
	return &bm
}
