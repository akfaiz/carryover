package syncer

import (
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/rclone/rclone/fs/filter"
)

// delta is the work for one slot: files to copy and files to delete, as
// '/'-separated paths relative to the slot folder.
type delta struct {
	copy  []string
	del   []string
	bytes int64 // size of the files to copy
}

func (d delta) empty() bool { return len(d.copy) == 0 && len(d.del) == 0 }

// slotFiles returns the files of one slot in m, keyed by path relative to the
// slot, keeping only those the slot's filter includes (nil filter: all).
func slotFiles(m *Manifest, slot string, flt *filter.Filter) map[string]FileInfo {
	out := map[string]FileInfo{}
	if m == nil {
		return out
	}
	prefix := slot + "/"
	for k, f := range m.Files {
		rel, ok := strings.CutPrefix(k, prefix)
		if !ok || (flt != nil && !flt.Include(rel, f.Size, time.Time{}, nil)) {
			continue
		}
		out[rel] = f
	}
	return out
}

// diff compares by size and SHA-256: it copies the files of src that dst
// lacks or has with different content, and deletes the files dst has that
// src doesn't. Unchanged files are left alone.
func diff(src, dst map[string]FileInfo) delta {
	var d delta
	for k, s := range src {
		if t, ok := dst[k]; ok && t.Size == s.Size && t.Hash == s.Hash {
			continue
		}
		d.copy = append(d.copy, k)
		d.bytes += s.Size
	}
	for k := range dst {
		if _, ok := src[k]; !ok {
			d.del = append(d.del, k)
		}
	}
	sort.Strings(d.copy)
	sort.Strings(d.del)
	return d
}

// filesWord is "1 file" / "n files".
func filesWord(n int) string {
	if n == 1 {
		return "1 file"
	}
	return strconv.Itoa(n) + " files"
}
