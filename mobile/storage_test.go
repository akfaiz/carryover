package mobile

import (
	"encoding/json"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/akfaiz/carryover/internal/cloudsync/localfs"
)

// fakeStorage is the app's Storage over a folder: ":saf:<rest>" is
// <dir>/<rest>. Like the app, it hands out file descriptors that Go closes.
type fakeStorage struct {
	dir  string
	mu   sync.Mutex
	open []*os.File // kept from the garbage collector, which would close their descriptors
}

func (s *fakeStorage) real(p string) string {
	return filepath.Join(s.dir, filepath.FromSlash(strings.TrimPrefix(p, localfs.SAF)))
}

func entry(fi os.FileInfo) storageEntry {
	return storageEntry{Name: fi.Name(), Dir: fi.IsDir(), Size: fi.Size(), ModTime: fi.ModTime().UnixMilli()}
}

func (s *fakeStorage) Stat(p string) (string, error) {
	fi, err := os.Stat(s.real(p))
	if os.IsNotExist(err) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	b, _ := json.Marshal(entry(fi))
	return string(b), nil
}

func (s *fakeStorage) List(p string) (string, error) {
	des, err := os.ReadDir(s.real(p))
	if os.IsNotExist(err) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	out := []storageEntry{}
	for _, d := range des {
		fi, _ := d.Info()
		out = append(out, entry(fi))
	}
	b, _ := json.Marshal(out)
	return string(b), nil
}

func (s *fakeStorage) keep(f *os.File) int64 {
	s.mu.Lock()
	s.open = append(s.open, f)
	s.mu.Unlock()
	return int64(f.Fd())
}

func (s *fakeStorage) OpenRead(p string) (int64, error) {
	f, err := os.Open(s.real(p))
	if err != nil {
		return -1, err
	}
	return s.keep(f), nil
}

func (s *fakeStorage) OpenWrite(p string) (int64, error) {
	r := s.real(p)
	if err := os.MkdirAll(filepath.Dir(r), 0o755); err != nil {
		return -1, err
	}
	f, err := os.Create(r)
	if err != nil {
		return -1, err
	}
	return s.keep(f), nil
}

func (s *fakeStorage) Remove(p string) error   { return os.Remove(s.real(p)) }
func (s *fakeStorage) MkdirAll(p string) error { return os.MkdirAll(s.real(p), 0o755) }

// The adapter turns the app's Storage into a localfs.FS.
func TestStorageAdapter(t *testing.T) {
	st := &fakeStorage{dir: t.TempDir()}
	SetStorage(st)
	t.Cleanup(func() { SetStorage(nil) })
	root := localfs.SAF + "auth/tree%3Ax/saves"

	_, err := localfs.Stat(root)
	require.ErrorIs(t, err, fs.ErrNotExist)
	_, err = localfs.ReadDir(root)
	require.ErrorIs(t, err, fs.ErrNotExist)
	_, err = localfs.Open(root + "/missing.sav")
	require.ErrorIs(t, err, fs.ErrNotExist)
	require.ErrorIs(t, localfs.Remove(root+"/missing.sav"), fs.ErrNotExist)

	w, err := localfs.Create(root + "/dir/a.sav")
	require.NoError(t, err)
	_, _ = io.WriteString(w, "save")
	require.NoError(t, w.Close())
	info, err := localfs.Stat(root + "/dir/a.sav")
	require.NoError(t, err)
	assert.Equal(t, int64(4), info.Size)
	assert.False(t, info.Dir)
	assert.False(t, info.ModTime.IsZero())

	var walked []string
	require.NoError(t, localfs.Walk(root, func(rel string, _ localfs.Info) error {
		walked = append(walked, rel)
		return nil
	}))
	assert.Equal(t, []string{"dir/a.sav"}, walked)

	r, err := localfs.Open(root + "/dir/a.sav")
	require.NoError(t, err)
	b, _ := io.ReadAll(r)
	require.NoError(t, r.Close())
	assert.Equal(t, "save", string(b))

	require.NoError(t, localfs.Remove(root+"/dir/a.sav"))
	require.NoError(t, localfs.MkdirAll(root+"/empty"))
	info, err = localfs.Stat(root + "/empty")
	require.NoError(t, err)
	assert.True(t, info.Dir)
}
