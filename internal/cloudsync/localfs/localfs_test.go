package localfs

import (
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const root = SAF + "com.example.docs/primary%3ASaves"

func TestJoin(t *testing.T) {
	assert.Equal(t, root+"/a/b.sav", Join(root, "a/b.sav"))
	assert.Equal(t, root+"/a/b.sav", Join(root+"/", "/a/./b.sav"))
	assert.Equal(t, root, Join(root, ""))
	assert.Equal(t, filepath.Join("x", "a", "b.sav"), Join("x", "a/b.sav"), "file paths use filepath.Join")
	assert.True(t, IsAbs(root))
	assert.False(t, IsAbs("relative/path"))
}

// Paths without a registered FS are plain files.
func TestOSPaths(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "sub", "a.sav")
	w, err := Create(p)
	require.NoError(t, err)
	_, _ = w.Write([]byte("save"))
	require.NoError(t, w.Close())
	info, err := Stat(p)
	require.NoError(t, err)
	assert.Equal(t, Info{Name: "a.sav", Size: 4, ModTime: info.ModTime}, info)
	r, err := Open(p)
	require.NoError(t, err)
	b, _ := io.ReadAll(r)
	_ = r.Close()
	assert.Equal(t, "save", string(b))
	require.NoError(t, Remove(p))
	_, err = Stat(p)
	assert.ErrorIs(t, err, os.ErrNotExist)
}

// A SAF path without a registered FS fails clearly instead of being read as
// a relative file path.
func TestUnregisteredSAF(t *testing.T) {
	_, err := Open(root + "/a.sav")
	require.Error(t, err)
	assert.True(t, errors.Is(err, errNoFS))
}

func TestMemAndWalk(t *testing.T) {
	m := NewMem()
	Register(SAF, m)
	t.Cleanup(func() { Register(SAF, nil) })

	_, err := Stat(root)
	require.ErrorIs(t, err, fs.ErrNotExist)
	require.NoError(t, MkdirAll(root))
	info, err := Stat(root)
	require.NoError(t, err)
	assert.True(t, info.Dir)

	for p, s := range map[string]string{"b.sav": "b", "dir/a.sav": "a", "dir/deep/c.sav": "cc"} {
		w, err := Create(Join(root, p))
		require.NoError(t, err)
		_, _ = io.WriteString(w, s)
		require.NoError(t, w.Close())
	}
	var got []string
	require.NoError(t, Walk(root, func(rel string, info Info) error {
		got = append(got, rel)
		return nil
	}))
	assert.Equal(t, []string{"b.sav", "dir/a.sav", "dir/deep/c.sav"}, got)

	require.Error(t, Remove(Join(root, "dir")), "folder not empty")
	require.NoError(t, Remove(Join(root, "dir/a.sav")))
	require.ErrorIs(t, Remove(Join(root, "dir/a.sav")), fs.ErrNotExist)
	assert.Equal(t, map[string]string{root + "/b.sav": "b", root + "/dir/deep/c.sav": "cc"}, m.Files())
}

// Exists follows wildcards segment by segment, in an FS and in plain folders.
func TestExists(t *testing.T) {
	write := func(base, rel string) {
		w, err := Create(Join(base, rel))
		require.NoError(t, err)
		_, _ = io.WriteString(w, "x")
		require.NoError(t, w.Close())
	}
	check := func(base string) {
		write(base, "nand/user/save/0000000000000000/AD2E/0100000000010000/progress.bin")
		write(base, "GC/USA/Card A/game.gci")
		assert.True(t, Exists(base, "nand/user/save/0000000000000000/*/0100000000010000"), "a wildcard profile folder")
		assert.True(t, Exists(base, "nand/user/save/0000000000000000/AD2E/0100000000010000"))
		assert.True(t, Exists(base, "GC/*/Card ?"))
		assert.True(t, Exists(base, ""), "the root itself")
		assert.False(t, Exists(base, "nand/user/save/0000000000000000/*/0100000000010001"), "another title")
		assert.False(t, Exists(base, "Wii/*"))
		assert.False(t, Exists(base, "nand/user/save/0000000000000000/*/0100000000010000/progress.bin/deeper"), "below a file")
	}
	m := NewMem()
	Register(SAF, m)
	t.Cleanup(func() { Register(SAF, nil) })
	require.NoError(t, MkdirAll(root))
	check(root)
	check(t.TempDir())
	assert.False(t, Exists(t.TempDir()+"/missing", "a"), "a root that is not there")
}
