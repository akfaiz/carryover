package core

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

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
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("imageFiles = %v, want %v", got, want)
	}
}

func TestCoverDelta(t *testing.T) {
	dir := t.TempDir()
	put := func(name, data string, mt time.Time) {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte(data), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(p, mt, mt); err != nil {
			t.Fatal(err)
		}
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
	if len(up) != 2 || up[0] != "local-newer.png" || up[1] != "only-local.png" || upBytes != 7 {
		t.Errorf("up = %v (%d bytes)", up, upBytes)
	}
	if len(down) != 2 || down[0] != "cloud-newer.png" || down[1] != "only-cloud.png" || downBytes != 12 {
		t.Errorf("down = %v (%d bytes)", down, downBytes)
	}
}

// A cover set on one device reaches the cloud, and a device that lacks the
// file gets it back.
func TestSyncLibraryCopiesCovers(t *testing.T) {
	c, gameID, _, cloudDir := newPlayable(t)
	g, err := c.Library.Get(gameID)
	if err != nil {
		t.Fatal(err)
	}
	g.Cover = gameID + ".png"
	if _, err := c.Library.Save(g, ""); err != nil {
		t.Fatal(err)
	}
	local := filepath.Join(c.Dirs.CoversDir(), g.Cover)
	if err := os.WriteFile(local, []byte("PNGDATA"), 0o644); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if err := c.SyncLibrary(ctx); err != nil {
		t.Fatal(err)
	}
	if b, err := os.ReadFile(filepath.Join(cloudDir, "covers", g.Cover)); err != nil || string(b) != "PNGDATA" {
		t.Fatalf("cloud cover = %q, %v", b, err)
	}
	// Another device: no local file yet.
	if err := os.Remove(local); err != nil {
		t.Fatal(err)
	}
	if err := c.SyncLibrary(ctx); err != nil {
		t.Fatal(err)
	}
	if b, err := os.ReadFile(local); err != nil || string(b) != "PNGDATA" {
		t.Fatalf("downloaded cover = %q, %v", b, err)
	}
}
