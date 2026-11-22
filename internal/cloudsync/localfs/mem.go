package localfs

import (
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"sort"
	"strings"
	"sync"
	"time"
)

// Mem is an FS in memory, for tests: it behaves like a Storage Access
// Framework tree (no file paths, no settable modification times).
type Mem struct {
	mu    sync.Mutex
	files map[string]memFile
	dirs  map[string]bool
	// Now gives the modification time of written files (time.Now if nil).
	Now func() time.Time
}

type memFile struct {
	data    []byte
	modTime time.Time
}

// NewMem returns an empty in-memory FS.
func NewMem() *Mem { return &Mem{files: map[string]memFile{}, dirs: map[string]bool{}} }

func clean(p string) string { return strings.TrimSuffix(p, "/") }

func (m *Mem) now() time.Time {
	if m.Now != nil {
		return m.Now()
	}
	return time.Now()
}

func notExist(op, p string) error { return &fs.PathError{Op: op, Path: p, Err: fs.ErrNotExist} }

func base(p string) string { return p[strings.LastIndex(p, "/")+1:] }

// isDir reports whether p is a folder: made with MkdirAll, or above a file.
func (m *Mem) isDir(p string) bool {
	if m.dirs[p] {
		return true
	}
	for f := range m.files {
		if strings.HasPrefix(f, p+"/") {
			return true
		}
	}
	for d := range m.dirs {
		if strings.HasPrefix(d, p+"/") {
			return true
		}
	}
	return false
}

// Stat implements FS.
func (m *Mem) Stat(p string) (Info, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	p = clean(p)
	if f, ok := m.files[p]; ok {
		return Info{Name: base(p), Size: int64(len(f.data)), ModTime: f.modTime}, nil
	}
	if m.isDir(p) {
		return Info{Name: base(p), Dir: true}, nil
	}
	return Info{}, notExist("stat", p)
}

// ReadDir implements FS.
func (m *Mem) ReadDir(p string) ([]Info, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	p = clean(p)
	if !m.isDir(p) {
		return nil, notExist("readdir", p)
	}
	seen := map[string]Info{}
	for f, data := range m.files {
		if rest, ok := strings.CutPrefix(f, p+"/"); ok {
			if name, _, sub := strings.Cut(rest, "/"); sub {
				seen[name] = Info{Name: name, Dir: true}
			} else {
				seen[name] = Info{Name: name, Size: int64(len(data.data)), ModTime: data.modTime}
			}
		}
	}
	for d := range m.dirs {
		if rest, ok := strings.CutPrefix(d, p+"/"); ok {
			name, _, _ := strings.Cut(rest, "/")
			if _, ok := seen[name]; !ok {
				seen[name] = Info{Name: name, Dir: true}
			}
		}
	}
	out := make([]Info, 0, len(seen))
	for _, i := range seen {
		out = append(out, i)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// Open implements FS.
func (m *Mem) Open(p string) (io.ReadCloser, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	f, ok := m.files[clean(p)]
	if !ok {
		return nil, notExist("open", p)
	}
	return io.NopCloser(bytes.NewReader(bytes.Clone(f.data))), nil
}

// Create implements FS. The file is stored when the writer is closed.
func (m *Mem) Create(p string) (io.WriteCloser, error) {
	p = clean(p)
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.dirs[p] {
		return nil, fmt.Errorf("create %s: is a folder", p)
	}
	return &memWriter{m: m, p: p}, nil
}

type memWriter struct {
	m   *Mem
	p   string
	buf bytes.Buffer
}

func (w *memWriter) Write(b []byte) (int, error) { return w.buf.Write(b) }

func (w *memWriter) Close() error {
	w.m.mu.Lock()
	defer w.m.mu.Unlock()
	w.m.files[w.p] = memFile{data: bytes.Clone(w.buf.Bytes()), modTime: w.m.now()}
	return nil
}

// Remove implements FS.
func (m *Mem) Remove(p string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	p = clean(p)
	if _, ok := m.files[p]; ok {
		delete(m.files, p)
		return nil
	}
	if m.dirs[p] {
		for f := range m.files {
			if strings.HasPrefix(f, p+"/") {
				return fmt.Errorf("remove %s: folder not empty", p)
			}
		}
		delete(m.dirs, p)
		return nil
	}
	return notExist("remove", p)
}

// MkdirAll implements FS.
func (m *Mem) MkdirAll(p string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	p = clean(p)
	if _, ok := m.files[p]; ok {
		return fmt.Errorf("mkdir %s: is a file", p)
	}
	m.dirs[p] = true
	return nil
}

// Files returns the content of every file, by path (for test assertions).
func (m *Mem) Files() map[string]string {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := map[string]string{}
	for p, f := range m.files {
		out[p] = string(f.data)
	}
	return out
}
