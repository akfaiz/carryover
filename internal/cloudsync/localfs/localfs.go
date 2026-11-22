// Package localfs reaches this device's save folders. Most are plain file
// paths, handled by the os package exactly as before; on Android a save folder
// can also be one the user granted through the Storage Access Framework, which
// has no file path. Such a path starts with a prefix (":saf:") registered with
// an FS that the app implements. Only code that reads or writes save folders
// goes through here; Carryover's own files (state, logs, temp files) use os.
package localfs

import (
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// SAF is the path prefix of folders granted through Android's Storage Access
// Framework: ":saf:<authority>/<tree document id, URL-escaped>[/<sub/path>]".
// It is also an rclone on-the-fly remote (see the cloud package's saf backend).
const SAF = ":saf:"

// Info describes a file or folder.
type Info struct {
	Name    string
	Dir     bool
	Size    int64
	ModTime time.Time
}

// FS is a tree of save folders reached without file paths. Paths are full
// paths including the registered prefix, '/'-separated. A missing file or
// folder is reported with an error wrapping fs.ErrNotExist.
type FS interface {
	Stat(p string) (Info, error)
	// ReadDir lists a folder's entries.
	ReadDir(p string) ([]Info, error)
	Open(p string) (io.ReadCloser, error)
	// Create opens a file for writing, truncating it, and creates the folders
	// above it as needed.
	Create(p string) (io.WriteCloser, error)
	// Remove deletes a file or an empty folder.
	Remove(p string) error
	MkdirAll(p string) error
}

var (
	mu       sync.RWMutex
	registry = map[string]FS{}
)

// Register makes f handle the paths that start with prefix (nil removes it).
func Register(prefix string, f FS) {
	mu.Lock()
	defer mu.Unlock()
	if f == nil {
		delete(registry, prefix)
		return
	}
	registry[prefix] = f
}

// Lookup returns the FS of p, or false for a plain file path.
func Lookup(p string) (FS, bool) {
	mu.RLock()
	defer mu.RUnlock()
	for prefix, f := range registry {
		if strings.HasPrefix(p, prefix) {
			return f, true
		}
	}
	return nil, false
}

// IsFS reports whether p belongs to a registered FS. A path in the SAF form
// counts even before the app registered one, so it is never mistaken for a
// relative file path.
func IsFS(p string) bool {
	if strings.HasPrefix(p, SAF) {
		return true
	}
	_, ok := Lookup(p)
	return ok
}

// IsAbs reports whether p is a usable save folder location: an absolute file
// path or a path of an FS.
func IsAbs(p string) bool { return IsFS(p) || filepath.IsAbs(p) }

// Join appends rel ('/'-separated) to root: filepath.Join for a file path,
// a '/' join for an FS path.
func Join(root, rel string) string {
	if IsFS(root) {
		rel = strings.TrimPrefix(path.Clean("/"+rel), "/")
		if rel == "" {
			return root
		}
		return strings.TrimSuffix(root, "/") + "/" + rel
	}
	return filepath.Join(root, filepath.FromSlash(rel))
}

func lookup(p string) (FS, error) {
	if f, ok := Lookup(p); ok {
		return f, nil
	}
	return nil, &fs.PathError{Op: "open", Path: p, Err: errNoFS}
}

var errNoFS = noFSError{}

type noFSError struct{}

func (noFSError) Error() string {
	return "this folder was granted to Carryover on Android and can't be opened here"
}

// Stat is os.Stat, or the FS's Stat.
func Stat(p string) (Info, error) {
	if !IsFS(p) {
		fi, err := os.Stat(p)
		if err != nil {
			return Info{}, err
		}
		return Info{Name: fi.Name(), Dir: fi.IsDir(), Size: fi.Size(), ModTime: fi.ModTime()}, nil
	}
	f, err := lookup(p)
	if err != nil {
		return Info{}, err
	}
	return f.Stat(p)
}

// Open is os.Open, or the FS's Open.
func Open(p string) (io.ReadCloser, error) {
	if !IsFS(p) {
		return os.Open(p)
	}
	f, err := lookup(p)
	if err != nil {
		return nil, err
	}
	return f.Open(p)
}

// Create is the FS's Create; for a file path it is os.Create after creating
// the folders above it.
func Create(p string) (io.WriteCloser, error) {
	if !IsFS(p) {
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			return nil, err
		}
		return os.Create(p)
	}
	f, err := lookup(p)
	if err != nil {
		return nil, err
	}
	return f.Create(p)
}

// ReadDir is os.ReadDir, or the FS's ReadDir.
func ReadDir(p string) ([]Info, error) {
	if !IsFS(p) {
		entries, err := os.ReadDir(p)
		if err != nil {
			return nil, err
		}
		out := make([]Info, 0, len(entries))
		for _, e := range entries {
			fi, err := e.Info()
			if err != nil {
				return nil, err
			}
			out = append(out, Info{Name: fi.Name(), Dir: fi.IsDir(), Size: fi.Size(), ModTime: fi.ModTime()})
		}
		return out, nil
	}
	f, err := lookup(p)
	if err != nil {
		return nil, err
	}
	return f.ReadDir(p)
}

// Remove is os.Remove, or the FS's Remove.
func Remove(p string) error {
	if !IsFS(p) {
		return os.Remove(p)
	}
	f, err := lookup(p)
	if err != nil {
		return err
	}
	return f.Remove(p)
}

// MkdirAll is os.MkdirAll with mode 0o755, or the FS's MkdirAll.
func MkdirAll(p string) error {
	if !IsFS(p) {
		return os.MkdirAll(p, 0o755)
	}
	f, err := lookup(p)
	if err != nil {
		return err
	}
	return f.MkdirAll(p)
}

// Walk calls fn for every file under the FS folder root, in name order, with
// its path relative to root ('/'-separated). It stops at the first error.
func Walk(root string, fn func(rel string, info Info) error) error {
	f, err := lookup(root)
	if err != nil {
		return err
	}
	return walk(f, root, "", fn)
}

func walk(f FS, root, rel string, fn func(string, Info) error) error {
	dir := root
	if rel != "" {
		dir = Join(root, rel)
	}
	entries, err := f.ReadDir(dir)
	if err != nil {
		return err
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name < entries[j].Name })
	for _, e := range entries {
		r := e.Name
		if rel != "" {
			r = rel + "/" + e.Name
		}
		if e.Dir {
			err = walk(f, root, r, fn)
		} else {
			err = fn(r, e)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

// maxMatches bounds the folders Exists follows at once, so a pattern with
// wildcards in a big tree can't run away.
const maxMatches = 256

// Exists reports whether anything matches rel ('/'-separated, with the
// wildcards of path.Match in any segment) under root: a file path or an FS
// path. It is a cheap "is there a save here?" for a location that has
// wildcards, such as a Switch profile folder.
func Exists(root, rel string) bool {
	cands := []string{root}
	for seg := range strings.SplitSeq(rel, "/") {
		if seg == "" || seg == "." {
			continue
		}
		var next []string
		for _, c := range cands {
			if !strings.ContainsAny(seg, `*?[`) {
				p := Join(c, seg)
				if _, err := Stat(p); err == nil {
					next = append(next, p)
				}
				continue
			}
			infos, err := ReadDir(c)
			if err != nil {
				continue
			}
			for _, in := range infos {
				if ok, _ := path.Match(seg, in.Name); ok {
					next = append(next, Join(c, in.Name))
				}
			}
		}
		if len(next) == 0 {
			return false
		}
		if len(next) > maxMatches {
			next = next[:maxMatches]
		}
		cands = next
	}
	_, err := Stat(cands[0])
	return err == nil
}
