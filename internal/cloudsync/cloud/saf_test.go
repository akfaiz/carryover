package cloud

import (
	"context"
	"io"
	"strings"
	"testing"
	"time"

	rfs "github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/cache"
	"github.com/rclone/rclone/fs/object"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/akfaiz/carryover/internal/cloudsync/localfs"
)

// The saf backend reaches a localfs folder as an rclone remote.
func TestSafBackend(t *testing.T) {
	mem := localfs.NewMem()
	localfs.Register(localfs.SAF, mem)
	t.Cleanup(func() { localfs.Register(localfs.SAF, nil) })
	ctx := context.Background()
	root := localfs.SAF + "com.example/tree%3Aroot/saves"

	f, err := cache.Get(ctx, root)
	require.NoError(t, err)
	_, err = f.List(ctx, "")
	require.ErrorIs(t, err, rfs.ErrorDirNotFound)
	_, err = f.NewObject(ctx, "a.sav")
	require.ErrorIs(t, err, rfs.ErrorObjectNotFound)

	put := func(remote, content string) {
		t.Helper()
		info := object.NewStaticObjectInfo(remote, time.Now(), int64(len(content)), true, nil, nil)
		o, err := f.Put(ctx, strings.NewReader(content), info)
		require.NoError(t, err)
		assert.Equal(t, int64(len(content)), o.Size())
	}
	put("a.sav", "0123456789")
	put("dir/b.sav", "b")
	assert.Equal(t, map[string]string{root + "/a.sav": "0123456789", root + "/dir/b.sav": "b"}, mem.Files())

	entries, err := f.List(ctx, "")
	require.NoError(t, err)
	require.Len(t, entries, 2)
	assert.Equal(t, "a.sav", entries[0].Remote())
	_, isDir := entries[1].(rfs.Directory)
	assert.True(t, isDir, "dir is listed as a folder")
	_, err = f.NewObject(ctx, "dir")
	require.ErrorIs(t, err, rfs.ErrorIsDir)

	o, err := f.NewObject(ctx, "a.sav")
	require.NoError(t, err)
	read := func(opts ...rfs.OpenOption) string {
		t.Helper()
		r, err := o.Open(ctx, opts...)
		require.NoError(t, err)
		defer r.Close()
		b, err := io.ReadAll(r)
		require.NoError(t, err)
		return string(b)
	}
	assert.Equal(t, "0123456789", read())
	assert.Equal(t, "3456", read(&rfs.RangeOption{Start: 3, End: 6}))
	assert.Equal(t, "789", read(&rfs.SeekOption{Offset: 7}))

	// Update replaces the content (a shorter one too: the file is truncated).
	require.NoError(t, o.Update(ctx, strings.NewReader("new"), object.NewStaticObjectInfo("a.sav", time.Now(), 3, true, nil, nil)))
	assert.Equal(t, "new", read())
	require.ErrorIs(t, o.SetModTime(ctx, time.Now()), rfs.ErrorCantSetModTime)

	require.NoError(t, o.Remove(ctx))
	_, err = f.NewObject(ctx, "a.sav")
	require.ErrorIs(t, err, rfs.ErrorObjectNotFound)
	assert.False(t, IsSupported("saf"), "not a cloud provider")
}
