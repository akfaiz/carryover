package core

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/akfaiz/carryover/internal/cloudsync/cloud"
	"github.com/akfaiz/carryover/internal/library"
)

// coversCloudDir is where cover and artwork images live in the cloud folder,
// by the same file names as in the local covers directory.
const coversCloudDir = "covers"

// imageFiles returns the sorted, de-duplicated image file names that the
// games (not deleted) refer to as cover or artwork. Names that aren't plain
// file names are dropped: the catalog comes from the cloud.
func imageFiles(games []library.Game) []string {
	seen := map[string]bool{}
	add := func(n string) {
		if n != "" && !strings.ContainsAny(n, `/\`) && filepath.IsLocal(n) {
			seen[n] = true
		}
	}
	for _, g := range games {
		if g.Deleted {
			continue
		}
		add(g.Cover)
		for _, n := range g.Art {
			add(n)
		}
	}
	out := make([]string, 0, len(seen))
	for n := range seen {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}

// coverDelta decides which images to upload and download. An image missing on
// one side is copied there; when both sides have it with different sizes the
// newer one wins. After a transfer the sizes match, so devices don't ping-pong.
func coverDelta(names []string, localDir string, cloudFiles map[string]cloud.FileStat) (up, down []string, upBytes, downBytes int64) {
	for _, n := range names {
		cs, inCloud := cloudFiles[n]
		fi, err := os.Stat(filepath.Join(localDir, n))
		local := err == nil && fi.Mode().IsRegular()
		switch {
		case local && !inCloud:
			up, upBytes = append(up, n), upBytes+fi.Size()
		case !local && inCloud:
			down, downBytes = append(down, n), downBytes+cs.Size
		case local && inCloud && fi.Size() != cs.Size:
			if fi.ModTime().After(cs.ModTime) {
				up, upBytes = append(up, n), upBytes+fi.Size()
			} else {
				down, downBytes = append(down, n), downBytes+cs.Size
			}
		}
	}
	return up, down, upBytes, downBytes
}

// syncCovers uploads the library's cover and artwork images that the cloud
// lacks and downloads the ones this device lacks, so a game looks the same on
// every device. Images no game refers to any more are left in the cloud.
func (c *Core) syncCovers(ctx context.Context, cl *cloud.Client) error {
	names := imageFiles(c.Library.Catalog().Games)
	if len(names) == 0 {
		return nil
	}
	listed, err := cl.ListFilesInfo(ctx, coversCloudDir)
	if err != nil {
		return fmt.Errorf("list cloud images: %w", err)
	}
	dir := c.Dirs.CoversDir()
	up, down, upBytes, downBytes := coverDelta(names, dir, listed)
	if err := cl.Upload(ctx, dir, coversCloudDir, up, upBytes, nil); err != nil {
		return fmt.Errorf("upload images: %w", err)
	}
	if err := cl.Download(ctx, coversCloudDir, dir, down, downBytes, nil); err != nil {
		return fmt.Errorf("download images: %w", err)
	}
	return nil
}
