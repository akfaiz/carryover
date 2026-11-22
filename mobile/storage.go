package mobile

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"time"

	"github.com/akfaiz/carryover/internal/cloudsync/localfs"
)

// Storage is implemented by the app: it reaches save folders granted through
// Android's Storage Access Framework, whose paths have the form
// ":saf:<authority>/<tree document id, URL-escaped>[/<sub/path>]" (no file
// path exists for them). Its methods are called on Go goroutines.
type Storage interface {
	// Stat describes a file or folder as JSON {"name", "dir", "size",
	// "modTime" (Unix milliseconds)}, or returns "" when it doesn't exist.
	Stat(path string) (string, error)
	// List lists a folder's entries as a JSON array of what Stat returns, or
	// returns "" when the folder doesn't exist.
	List(path string) (string, error)
	// OpenRead opens a file for reading and returns its file descriptor,
	// detached (ParcelFileDescriptor.detachFd): Go closes it.
	OpenRead(path string) (int64, error)
	// OpenWrite opens a file for writing, truncating it and creating it and
	// the folders above it as needed, and returns its detached descriptor.
	OpenWrite(path string) (int64, error)
	// Remove deletes a file or an empty folder.
	Remove(path string) error
	// MkdirAll creates a folder and the folders above it.
	MkdirAll(path string) error
}

// SetStorage makes save folders of the ":saf:" form reachable through s (nil
// removes it). Call it after Init, before syncing such folders.
func SetStorage(s Storage) {
	if s == nil {
		localfs.Register(localfs.SAF, nil)
		return
	}
	localfs.Register(localfs.SAF, storageFS{s})
}

// storageFS is a localfs.FS over the app's Storage.
type storageFS struct{ s Storage }

type storageEntry struct {
	Name    string `json:"name"`
	Dir     bool   `json:"dir"`
	Size    int64  `json:"size"`
	ModTime int64  `json:"modTime"`
}

func (e storageEntry) info() localfs.Info {
	return localfs.Info{Name: e.Name, Dir: e.Dir, Size: e.Size, ModTime: time.UnixMilli(e.ModTime)}
}

func notExist(op, p string) error { return &fs.PathError{Op: op, Path: p, Err: fs.ErrNotExist} }

func (a storageFS) Stat(p string) (localfs.Info, error) {
	j, err := a.s.Stat(p)
	if err != nil {
		return localfs.Info{}, err
	}
	if j == "" {
		return localfs.Info{}, notExist("stat", p)
	}
	var e storageEntry
	if err := json.Unmarshal([]byte(j), &e); err != nil {
		return localfs.Info{}, fmt.Errorf("stat %s: %w", p, err)
	}
	return e.info(), nil
}

func (a storageFS) ReadDir(p string) ([]localfs.Info, error) {
	j, err := a.s.List(p)
	if err != nil {
		return nil, err
	}
	if j == "" {
		return nil, notExist("readdir", p)
	}
	var entries []storageEntry
	if err := json.Unmarshal([]byte(j), &entries); err != nil {
		return nil, fmt.Errorf("list %s: %w", p, err)
	}
	out := make([]localfs.Info, len(entries))
	for i, e := range entries {
		out[i] = e.info()
	}
	return out, nil
}

func file(fd int64, p string) (*os.File, error) {
	if fd < 0 {
		return nil, errors.New("no file descriptor for " + p)
	}
	return os.NewFile(uintptr(fd), p), nil
}

func (a storageFS) Open(p string) (io.ReadCloser, error) {
	fd, err := a.s.OpenRead(p)
	if err != nil {
		if _, serr := a.Stat(p); errors.Is(serr, fs.ErrNotExist) {
			return nil, notExist("open", p)
		}
		return nil, err
	}
	return file(fd, p)
}

func (a storageFS) Create(p string) (io.WriteCloser, error) {
	fd, err := a.s.OpenWrite(p)
	if err != nil {
		return nil, err
	}
	return file(fd, p)
}

func (a storageFS) Remove(p string) error {
	if _, err := a.Stat(p); err != nil {
		return err
	}
	return a.s.Remove(p)
}

func (a storageFS) MkdirAll(p string) error { return a.s.MkdirAll(p) }
