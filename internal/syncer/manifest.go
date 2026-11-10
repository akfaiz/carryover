// Package syncer decides whether to push or pull a game's saves and performs
// the transfer through rclone, keeping backups of whatever gets overwritten.
package syncer

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/akfaiz/playrelay/internal/cloud"
)

// FileInfo describes one save file.
type FileInfo struct {
	Size    int64     `json:"size"`
	Hash    string    `json:"sha256"`
	ModTime time.Time `json:"mtime"`
}

// Manifest describes a full save snapshot. File keys are "<slot>/<relative path>" using '/'.
type Manifest struct {
	Revision   string              `json:"revision"`
	Device     string              `json:"device"`
	DeviceName string              `json:"deviceName"`
	Time       time.Time           `json:"time"`
	Files      map[string]FileInfo `json:"files"`
}

// Summary is a human-oriented overview of a manifest.
type Summary struct {
	Files      int       `json:"files"`
	Bytes      int64     `json:"bytes"`
	Newest     time.Time `json:"newest"`
	DeviceName string    `json:"deviceName"`
	Time       time.Time `json:"time"`
}

// Summarize returns a Summary of m (zero value for nil).
func (m *Manifest) Summarize() Summary {
	if m == nil {
		return Summary{}
	}
	s := Summary{Files: len(m.Files), DeviceName: m.DeviceName, Time: m.Time}
	for _, f := range m.Files {
		s.Bytes += f.Size
		if f.ModTime.After(s.Newest) {
			s.Newest = f.ModTime
		}
	}
	return s
}

// SameFiles reports whether a and b contain identical file sets (by size and hash).
func SameFiles(a, b *Manifest) bool {
	var af, bf map[string]FileInfo
	if a != nil {
		af = a.Files
	}
	if b != nil {
		bf = b.Files
	}
	if len(af) != len(bf) {
		return false
	}
	for k, x := range af {
		y, ok := bf[k]
		if !ok || x.Size != y.Size || x.Hash != y.Hash {
			return false
		}
	}
	return true
}

// Slot is a save location on this device.
type Slot struct {
	Key       string
	LocalPath string
	Filters   cloud.Filters
}

// ScanLocal hashes the files of all slots. prev, if given, is used to skip
// re-hashing files whose size and modification time are unchanged. It stops
// early with ctx's error when ctx is cancelled.
func ScanLocal(ctx context.Context, slots []Slot, prev *Manifest) (*Manifest, error) {
	m := &Manifest{Files: map[string]FileInfo{}}
	for _, sl := range slots {
		flt, err := sl.Filters.NewFilter()
		if err != nil {
			return nil, err
		}
		root := sl.LocalPath
		fi, err := os.Stat(root)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, err
		}
		if !fi.IsDir() {
			return nil, errors.New("save path is not a directory: " + root)
		}
		err = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if err := ctx.Err(); err != nil {
				return err
			}
			if d.IsDir() || !d.Type().IsRegular() {
				return nil
			}
			rel, err := filepath.Rel(root, p)
			if err != nil {
				return err
			}
			rel = filepath.ToSlash(rel)
			info, err := d.Info()
			if err != nil {
				return err
			}
			if !flt.Include(rel, info.Size(), info.ModTime(), nil) {
				return nil
			}
			key := sl.Key + "/" + rel
			mt := info.ModTime().UTC().Truncate(time.Millisecond)
			if prev != nil {
				if pf, ok := prev.Files[key]; ok && pf.Size == info.Size() && pf.ModTime.Equal(mt) && pf.Hash != "" {
					m.Files[key] = pf
					return nil
				}
			}
			h, err := hashFile(p)
			if err != nil {
				return err
			}
			m.Files[key] = FileInfo{Size: info.Size(), Hash: h, ModTime: mt}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	return m, nil
}

func hashFile(p string) (string, error) {
	f, err := os.Open(p)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
