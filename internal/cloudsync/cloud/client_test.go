package cloud

import (
	"context"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"testing"
	"time"

	_ "github.com/rclone/rclone/backend/memory" // a named, non-local remote
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config"
	"github.com/rclone/rclone/fstest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fixedTime is the modification time given to every file the tests write, so
// listings can be compared exactly.
var fixedTime = time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)

// bases returns a client for each way a base can look: a plain local folder
// and a path inside a named remote (rclone's in-memory backend).
func bases(t *testing.T) map[string]*Client {
	t.Helper()
	require.NoError(t, Init(filepath.Join(t.TempDir(), "rclone.conf")))
	setRemote(t, "mem", map[string]string{"type": "memory"})
	local, err := NewClient(t.TempDir())
	require.NoError(t, err)
	// A fresh folder name per test keeps runs apart: the memory backend is
	// shared by the whole test process.
	remote, err := NewClient(fmt.Sprintf("mem:Carryover-%d", time.Now().UnixNano()))
	require.NoError(t, err)
	return map[string]*Client{"local folder": local, "named remote": remote}
}

// setRemote adds a remote to rclone's process-wide config and removes it when
// the test ends, so other tests don't see it listed.
func setRemote(t *testing.T, name string, values map[string]string) {
	t.Helper()
	for k, v := range values {
		config.FileSetValue(name, k, v)
	}
	t.Cleanup(func() { config.DeleteRemote(name) })
}

// writeLocal creates files (rel path -> content) under dir with fixedTime.
func writeLocal(t *testing.T, dir string, files map[string]string) {
	t.Helper()
	for rel, content := range files {
		p := filepath.Join(dir, filepath.FromSlash(rel))
		require.NoError(t, os.MkdirAll(filepath.Dir(p), 0o755))
		require.NoError(t, os.WriteFile(p, []byte(content), 0o644))
		require.NoError(t, os.Chtimes(p, fixedTime, fixedTime))
	}
}

func readLocal(t *testing.T, dir, rel string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(rel)))
	require.NoError(t, err)
	return string(b)
}

func TestNewClient(t *testing.T) {
	_, err := NewClient("")
	require.Error(t, err, "no remote configured")

	c, err := NewClient("gdrive:Carryover///")
	require.NoError(t, err)
	assert.Equal(t, "gdrive:Carryover", c.Base, "trailing slashes are trimmed")
}

func TestClientPath(t *testing.T) {
	tests := []struct {
		base string
		rel  []string
		want string
	}{
		{"mem:Carryover", nil, "mem:Carryover"},
		{"mem:Carryover", []string{"games", "g1"}, "mem:Carryover/games/g1"},
		{"mem:Carryover", []string{"."}, "mem:Carryover"},
		{"mem:", []string{"games"}, "mem:games"},
		{"/data/Carryover", []string{"a", "b.json"}, "/data/Carryover/a/b.json"},
	}
	for _, tt := range tests {
		c := &Client{Base: tt.base}
		assert.Equal(t, tt.want, c.Path(tt.rel...), "%s + %v", tt.base, tt.rel)
	}
}

func TestJSONRoundTrip(t *testing.T) {
	type doc struct {
		Name string `json:"name"`
		N    int    `json:"n"`
	}
	ctx := context.Background()
	for name, c := range bases(t) {
		t.Run(name, func(t *testing.T) {
			var got doc
			found, err := c.ReadJSON(ctx, "games/g1/lock.json", &got)
			require.NoError(t, err)
			require.False(t, found, "a missing file is not an error")

			require.NoError(t, c.WriteJSON(ctx, "games/g1/lock.json", doc{Name: "pc", N: 3}), "parent folders are created")
			found, err = c.ReadJSON(ctx, "games/g1/lock.json", &got)
			require.NoError(t, err)
			require.True(t, found)
			assert.Equal(t, doc{Name: "pc", N: 3}, got)

			// Writing again replaces the file.
			require.NoError(t, c.WriteJSON(ctx, "games/g1/lock.json", doc{Name: "deck", N: 4}))
			_, err = c.ReadJSON(ctx, "games/g1/lock.json", &got)
			require.NoError(t, err)
			assert.Equal(t, doc{Name: "deck", N: 4}, got)

			require.Error(t, c.WriteJSON(ctx, "x.json", make(chan int)), "an unmarshalable value")
		})
	}
}

func TestReadJSONCorrupt(t *testing.T) {
	ctx := context.Background()
	for name, c := range bases(t) {
		t.Run(name, func(t *testing.T) {
			src := t.TempDir()
			writeLocal(t, src, map[string]string{"bad.json": "{not json"})
			require.NoError(t, c.Upload(ctx, src, "", []string{"bad.json"}, 9, nil))
			var v map[string]any
			found, err := c.ReadJSON(ctx, "bad.json", &v)
			require.Error(t, err)
			assert.True(t, found, "the file exists, it just can't be parsed")
		})
	}
}

func TestUploadListDownload(t *testing.T) {
	ctx := context.Background()
	files := map[string]string{"a.sav": "alpha", "sub/b.sav": "bravo!", "sub/deep/c.sav": "c"}
	names := []string{"a.sav", "sub/b.sav", "sub/deep/c.sav"}
	for name, c := range bases(t) {
		t.Run(name, func(t *testing.T) {
			src := t.TempDir()
			writeLocal(t, src, files)

			var last Progress
			require.NoError(t, c.Upload(ctx, src, "games/g1/current", names, 12, func(p Progress) { last = p }))
			assert.EqualValues(t, 12, last.TotalBytes, "progress reports the total")

			sizes, err := c.ListFiles(ctx, "games/g1/current")
			require.NoError(t, err)
			assert.Equal(t, map[string]int64{"a.sav": 5, "sub/b.sav": 6, "sub/deep/c.sav": 1}, sizes)

			info, err := c.ListFilesInfo(ctx, "games/g1/current")
			require.NoError(t, err)
			require.Contains(t, info, "sub/b.sav")
			assert.EqualValues(t, 6, info["sub/b.sav"].Size)
			assert.WithinDuration(t, fixedTime, info["sub/b.sav"].ModTime, 2*time.Second, "uploads keep the file's modification time")

			dirs, fileNames, err := c.List(ctx, "games/g1/current")
			require.NoError(t, err)
			assert.Equal(t, []string{"sub"}, dirs)
			assert.Equal(t, []string{"a.sav"}, fileNames)
			dirs, err = c.ListDirs(ctx, "games/g1")
			require.NoError(t, err)
			assert.Equal(t, []string{"current"}, dirs)

			dst := t.TempDir()
			require.NoError(t, c.Download(ctx, "games/g1/current", dst, names, 12, nil))
			for rel, want := range files {
				assert.Equal(t, want, readLocal(t, dst, rel), rel)
			}

			// Uploading again replaces the files in place.
			writeLocal(t, src, map[string]string{"a.sav": "alpha-2"})
			require.NoError(t, c.Upload(ctx, src, "games/g1/current", []string{"a.sav"}, 7, nil))
			sizes, err = c.ListFiles(ctx, "games/g1/current")
			require.NoError(t, err)
			assert.EqualValues(t, 7, sizes["a.sav"])
			assert.Len(t, sizes, 3, "nothing was duplicated")
		})
	}
}

// The listing of what a client wrote, compared with rclone's own fstest
// helpers: names, sizes, modification times and checksums together.
func TestUploadedFilesMatchExpectedListing(t *testing.T) {
	ctx := context.Background()
	for name, c := range bases(t) {
		t.Run(name, func(t *testing.T) {
			src := t.TempDir()
			writeLocal(t, src, map[string]string{"a.sav": "alpha", "sub/b.sav": "bravo!"})
			require.NoError(t, c.Upload(ctx, src, "cur", []string{"a.sav", "sub/b.sav"}, 11, nil))

			// The client addresses files relative to the remote's root, so the
			// expected paths carry the folder's own path.
			f, p, err := c.fs(ctx, "cur")
			require.NoError(t, err)
			fstest.CheckListingWithRoot(t, f, p, []fstest.Item{
				fstest.NewItem(path.Join(p, "a.sav"), "alpha", fixedTime),
				fstest.NewItem(path.Join(p, "sub/b.sav"), "bravo!", fixedTime),
			}, nil, time.Second)
		})
	}
}

func TestCopyWithin(t *testing.T) {
	ctx := context.Background()
	for name, c := range bases(t) {
		t.Run(name, func(t *testing.T) {
			src := t.TempDir()
			writeLocal(t, src, map[string]string{"a.sav": "alpha", "sub/b.sav": "bravo!"})
			require.NoError(t, c.Upload(ctx, src, "current", []string{"a.sav", "sub/b.sav"}, 11, nil))

			require.NoError(t, c.CopyWithin(ctx, "current", "backup", []string{"a.sav", "sub/b.sav"}, 11, nil))
			sizes, err := c.ListFiles(ctx, "backup")
			require.NoError(t, err)
			assert.Equal(t, map[string]int64{"a.sav": 5, "sub/b.sav": 6}, sizes)
			// The source is untouched.
			sizes, err = c.ListFiles(ctx, "current")
			require.NoError(t, err)
			assert.Len(t, sizes, 2)

			require.Error(t, c.CopyWithin(ctx, "current", "backup2", []string{"missing.sav"}, 1, nil), "a file that isn't there")
		})
	}
}

func TestDeleteAndPurge(t *testing.T) {
	ctx := context.Background()
	for name, c := range bases(t) {
		t.Run(name, func(t *testing.T) {
			src := t.TempDir()
			writeLocal(t, src, map[string]string{"a.sav": "a", "b.sav": "b", "sub/c.sav": "c"})
			require.NoError(t, c.Upload(ctx, src, "g", []string{"a.sav", "b.sav", "sub/c.sav"}, 3, nil))

			// Missing files are ignored; the listed ones go.
			require.NoError(t, c.DeleteFiles(ctx, "g", []string{"a.sav", "never-there.sav"}))
			require.NoError(t, c.DeleteFiles(ctx, "g", nil), "nothing to delete")
			sizes, err := c.ListFiles(ctx, "g")
			require.NoError(t, err)
			assert.Equal(t, map[string]int64{"b.sav": 1, "sub/c.sav": 1}, sizes)

			require.NoError(t, c.Delete(ctx, "g/b.sav"))
			require.NoError(t, c.Delete(ctx, "g/b.sav"), "deleting a file twice")
			sizes, _ = c.ListFiles(ctx, "g")
			assert.Equal(t, map[string]int64{"sub/c.sav": 1}, sizes)

			require.NoError(t, c.Purge(ctx, "g"))
			sizes, err = c.ListFiles(ctx, "g")
			require.NoError(t, err)
			assert.Empty(t, sizes)
			require.NoError(t, c.Purge(ctx, "g"), "purging a missing folder")
		})
	}
}

// A folder that doesn't exist lists as empty instead of failing.
func TestMissingFolderListsEmpty(t *testing.T) {
	ctx := context.Background()
	for name, c := range bases(t) {
		t.Run(name, func(t *testing.T) {
			dirs, files, err := c.List(ctx, "nope")
			require.NoError(t, err)
			assert.Empty(t, dirs)
			assert.Empty(t, files)
			sizes, err := c.ListFiles(ctx, "nope")
			require.NoError(t, err)
			assert.NotNil(t, sizes)
			assert.Empty(t, sizes)
			dirs, err = c.ListDirs(ctx, "nope")
			require.NoError(t, err)
			assert.Empty(t, dirs)
		})
	}
}

func TestTransfersWithNoFilesDoNothing(t *testing.T) {
	ctx := context.Background()
	for name, c := range bases(t) {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			require.NoError(t, c.Upload(ctx, dir, "x", nil, 0, nil))
			require.NoError(t, c.Download(ctx, "x", dir, nil, 0, nil))
			require.NoError(t, c.CopyWithin(ctx, "x", "y", nil, 0, nil))
			sizes, _ := c.ListFiles(ctx, "x")
			assert.Empty(t, sizes, "an empty upload created nothing")
		})
	}
}

// A cancelled context stops a transfer before it starts any file.
func TestCancelledTransfer(t *testing.T) {
	for name, c := range bases(t) {
		t.Run(name, func(t *testing.T) {
			src := t.TempDir()
			writeLocal(t, src, map[string]string{"a.sav": "a"})
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			err := c.Upload(ctx, src, "g", []string{"a.sav"}, 1, nil)
			require.ErrorIs(t, err, context.Canceled)
		})
	}
}

// Uploading a file that doesn't exist locally reports the error.
func TestUploadMissingSourceFails(t *testing.T) {
	for name, c := range bases(t) {
		t.Run(name, func(t *testing.T) {
			err := c.Upload(context.Background(), t.TempDir(), "g", []string{"missing.sav"}, 1, nil)
			require.Error(t, err)
		})
	}
}

// A base that points at a file instead of a folder is reported, not used.
func TestBaseIsAFile(t *testing.T) {
	require.NoError(t, Init(filepath.Join(t.TempDir(), "rclone.conf")))
	file := filepath.Join(t.TempDir(), "f")
	require.NoError(t, os.WriteFile(file, []byte("x"), 0o644))
	c, err := NewClient(file)
	require.NoError(t, err)
	_, err = c.ListFiles(context.Background(), "")
	require.ErrorContains(t, err, "is a file")
}

func TestTuneRemote(t *testing.T) {
	require.NoError(t, Init(filepath.Join(t.TempDir(), "rclone.conf")))
	setRemote(t, "dbx", map[string]string{"type": "dropbox"})
	setRemote(t, "dbx-batch", map[string]string{"type": "dropbox", "batch_mode": "sync"})
	setRemote(t, "other", map[string]string{"type": "memory"})

	assert.Equal(t, "dbx,batch_mode=off:", tuneRemote("dbx:"), "Dropbox upload batching is turned off")
	assert.Equal(t, "dbx-batch:", tuneRemote("dbx-batch:"), "a batch mode the user chose is kept")
	assert.Equal(t, "other:", tuneRemote("other:"), "other backends are left alone")
	assert.Equal(t, "unknown:", tuneRemote("unknown:"), "a remote that isn't configured")
}

func TestFilters(t *testing.T) {
	assert.True(t, Filters{}.Empty())
	assert.False(t, Filters{Include: []string{"*.sav"}}.Empty())
	assert.False(t, Filters{Exclude: []string{"*.log"}}.Empty())

	now := time.Now()
	inc, err := Filters{Include: []string{"*.sav"}}.NewFilter()
	require.NoError(t, err)
	assert.True(t, inc.Include("slot1.sav", 1, now, nil))
	assert.False(t, inc.Include("notes.txt", 1, now, nil), "not matched by the include list")

	exc, err := Filters{Exclude: []string{"*.log"}}.NewFilter()
	require.NoError(t, err)
	assert.True(t, exc.Include("slot1.sav", 1, now, nil))
	assert.False(t, exc.Include("debug.log", 1, now, nil), "excluded")

	// Both lists: an exclude wins over an include that also matches the file.
	both, err := Filters{Include: []string{"*"}, Exclude: []string{"*.log"}}.NewFilter()
	require.NoError(t, err)
	assert.True(t, both.Include("slot1.sav", 1, now, nil))
	assert.False(t, both.Include("debug.log", 1, now, nil), "excluded although the include list matches it")

	narrow, err := Filters{Include: []string{"*.sav"}, Exclude: []string{"debug.sav"}}.NewFilter()
	require.NoError(t, err)
	assert.True(t, narrow.Include("slot1.sav", 1, now, nil))
	assert.False(t, narrow.Include("debug.sav", 1, now, nil), "excluded although the include list matches it")
	assert.False(t, narrow.Include("notes.txt", 1, now, nil), "not on the include list")

	all, err := Filters{}.NewFilter()
	require.NoError(t, err)
	assert.True(t, all.Include("anything.bin", 1, now, nil), "no rules: everything passes")

	_, err = Filters{Include: []string{"[bad"}}.NewFilter()
	require.Error(t, err, "an invalid glob")
}

func TestIsNotFound(t *testing.T) {
	assert.False(t, isNotFound(nil))
	assert.False(t, isNotFound(os.ErrPermission))
	for _, err := range []error{fs.ErrorObjectNotFound, fs.ErrorDirNotFound, fs.ErrorIsFile, fmt.Errorf("wrapped: %w", fs.ErrorObjectNotFound)} {
		assert.True(t, isNotFound(err), err.Error())
	}
}

// Folder names that start or end with a dot (a ".carryover" remote folder, a
// slot called "saves.") are kept as they are.
func TestDotsInPathsAreKept(t *testing.T) {
	for name, c := range bases(t) {
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			require.NoError(t, c.WriteJSON(ctx, ".hidden/x.json", map[string]int{"a": 1}))
			require.NoError(t, c.WriteJSON(ctx, "saves./y.json", map[string]int{"a": 2}))
			dirs, err := c.ListDirs(ctx, "")
			require.NoError(t, err)
			require.Equal(t, []string{".hidden", "saves."}, dirs)
			var v map[string]int
			found, err := c.ReadJSON(ctx, ".hidden/x.json", &v)
			require.NoError(t, err)
			require.True(t, found)
			require.Equal(t, 1, v["a"])
			_, files, err := c.List(ctx, "")
			require.NoError(t, err)
			require.Empty(t, files)
		})
	}
}
