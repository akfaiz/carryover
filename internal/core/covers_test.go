package core

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/akfaiz/carryover/internal/cloud"
	"github.com/akfaiz/carryover/internal/library"
)

func TestImageFiles(t *testing.T) {
	got := imageFiles([]library.Game{
		{Cover: "a.png", Art: map[string]string{"hero": "a_hero.jpg", "logo": ""}},
		{Cover: "b.png", Deleted: true},
		{Cover: "../evil.png", Art: map[string]string{"icon": `x\y.png`, "wide": "a.png"}},
	})
	want := []string{"a.png", "a_hero.jpg"}
	require.Equal(t, want, got, "imageFiles")
}

func TestCoverDelta(t *testing.T) {
	dir := t.TempDir()
	put := func(name, data string, mt time.Time) {
		p := filepath.Join(dir, name)
		require.NoError(t, os.WriteFile(p, []byte(data), 0o644))
		require.NoError(t, os.Chtimes(p, mt, mt))
	}
	old, recent := time.Now().Add(-time.Hour), time.Now()
	put("only-local.png", "aaa", recent)
	put("same.png", "bb", recent)
	put("local-newer.png", "cccc", recent)
	put("cloud-newer.png", "dd", old)
	cloudFiles := map[string]cloud.FileStat{
		"only-cloud.png":  {Size: 5, ModTime: recent},
		"same.png":        {Size: 2, ModTime: old},
		"local-newer.png": {Size: 9, ModTime: old},
		"cloud-newer.png": {Size: 7, ModTime: recent},
	}
	names := []string{"cloud-newer.png", "local-newer.png", "only-cloud.png", "only-local.png", "same.png"}
	up, down, upBytes, downBytes := coverDelta(names, dir, cloudFiles)
	assert.Equal(t, []string{"local-newer.png", "only-local.png"}, up)
	assert.EqualValues(t, 7, upBytes)
	assert.Equal(t, []string{"cloud-newer.png", "only-cloud.png"}, down)
	assert.EqualValues(t, 12, downBytes)
}

// A cover set on one device reaches the cloud, and a device that lacks the
// file gets it back.
func TestSyncLibraryCopiesCovers(t *testing.T) {
	c, gameID, _, cloudDir := newPlayable(t)
	g, err := c.Library.Get(gameID)
	require.NoError(t, err)
	g.Cover = gameID + ".png"
	_, err = c.Library.Save(g, "")
	require.NoError(t, err)
	local := filepath.Join(c.Dirs.CoversDir(), g.Cover)
	require.NoError(t, os.WriteFile(local, []byte("PNGDATA"), 0o644))
	ctx := context.Background()
	require.NoError(t, c.SyncLibrary(ctx))
	b, err := os.ReadFile(filepath.Join(cloudDir, "covers", g.Cover))
	require.NoError(t, err, "cloud cover")
	require.Equal(t, "PNGDATA", string(b))
	// Another device: no local file yet.
	require.NoError(t, os.Remove(local))
	require.NoError(t, c.SyncLibrary(ctx))
	b, err = os.ReadFile(local)
	require.NoError(t, err, "downloaded cover")
	require.Equal(t, "PNGDATA", string(b))
}
