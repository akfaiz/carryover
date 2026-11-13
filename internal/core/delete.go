package core

import (
	"context"
	"fmt"
	"log"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/akfaiz/carryover/internal/cloudsync/cloud"
	"github.com/akfaiz/carryover/internal/cloudsync/syncer"
	"github.com/akfaiz/carryover/internal/library"
)

// gamesCloudDir is the cloud folder with one sub-folder per sync key.
const gamesCloudDir = "games"

// SaveKeysOf returns every sync key a game has saves under: its own and one per
// emulator. The cloud folder is listed too, so the folders of emulators that
// were removed from the game earlier are found as well.
func (c *Core) SaveKeysOf(ctx context.Context, gameID string) ([]string, error) {
	keys := map[string]bool{gameID: true}
	if g, err := c.Library.GetAny(gameID); err == nil {
		for _, ge := range g.Emulators {
			keys[SyncKey(gameID, ge.EmulatorID)] = true
		}
	}
	if c.CloudConfigured() {
		cl, err := cloud.NewClient(c.Settings.Get().CloudBase())
		if err != nil {
			return nil, err
		}
		dirs, err := cl.ListDirs(ctx, gamesCloudDir)
		if err != nil {
			return nil, fmt.Errorf("list cloud saves: %w", err)
		}
		for _, d := range dirs {
			if GameOf(d) == gameID {
				keys[d] = true
			}
		}
	}
	out := make([]string, 0, len(keys))
	for k := range keys {
		out = append(out, k)
	}
	sort.Strings(out)
	return out, nil
}

// SaveKeysOfEmulator returns the sync keys of an emulator's saves, one per
// game: those of the games that use it and, found by listing the cloud folder,
// those left by games that no longer do.
func (c *Core) SaveKeysOfEmulator(ctx context.Context, emulatorID string) ([]string, error) {
	keys := map[string]bool{}
	for _, g := range append(c.Library.List(), c.Library.Trash()...) {
		if _, ok := g.Emulator(emulatorID); ok {
			keys[SyncKey(g.ID, emulatorID)] = true
		}
	}
	if c.CloudConfigured() {
		cl, err := cloud.NewClient(c.Settings.Get().CloudBase())
		if err != nil {
			return nil, err
		}
		dirs, err := cl.ListDirs(ctx, gamesCloudDir)
		if err != nil {
			return nil, fmt.Errorf("list cloud saves: %w", err)
		}
		for _, d := range dirs {
			if _, emu := ParseKey(d); emu == emulatorID {
				keys[d] = true
			}
		}
	}
	out := make([]string, 0, len(keys))
	for k := range keys {
		out = append(out, k)
	}
	sort.Strings(out)
	return out, nil
}

// PurgeSaves deletes everything stored under the sync keys: the cloud folders
// (saves, backups, lock) and, on this device, the sync state, session marker
// and safety copies. Nothing local is touched if a cloud folder can't be
// removed, so the call can be repeated.
func (c *Core) PurgeSaves(ctx context.Context, keys []string) error {
	if len(keys) == 0 {
		return nil
	}
	if c.CloudConfigured() {
		cl, err := cloud.NewClient(c.Settings.Get().CloudBase())
		if err != nil {
			return err
		}
		for _, k := range keys {
			if err := validKey(k); err != nil {
				return err
			}
			if err := cl.Purge(ctx, path.Join(gamesCloudDir, k)); err != nil {
				return fmt.Errorf("delete cloud saves: %w", err)
			}
		}
	}
	for _, k := range keys {
		c.forgetLocal(k)
	}
	return nil
}

// validKey guards against a key that would address something outside games/<key>.
func validKey(k string) error {
	if k == "" || k == "." || k == ".." || strings.ContainsAny(k, `/\`) {
		return fmt.Errorf("invalid save folder %q", k)
	}
	return nil
}

// forgetLocal removes this device's own records of a sync key.
func (c *Core) forgetLocal(key string) {
	if err := c.States.Delete(key); err != nil {
		log.Printf("forget state %s: %v", key, err)
	}
	c.clearSession(key)
	if err := os.RemoveAll(syncer.SafetyGameDir(c.Dirs.SafetyDir(), key)); err != nil {
		log.Printf("forget safety copies %s: %v", key, err)
	}
}

// DropImages deletes the cover and artwork files of a game that was removed
// from the library, here and, when the library is synced, in the cloud. Images
// another game still uses are kept. Cloud failures are only logged: a stray
// image costs nothing.
func (c *Core) DropImages(ctx context.Context, g library.Game) {
	// The game is a deleted one (from the trash), which imageFiles would skip;
	// its images are the ones to drop.
	g.Deleted = false
	names := imageFiles([]library.Game{g})
	if len(names) == 0 {
		return
	}
	inUse := map[string]bool{}
	for _, n := range imageFiles(c.Library.List()) {
		inUse[n] = true
	}
	var drop []string
	for _, n := range names {
		if !inUse[n] {
			drop = append(drop, n)
		}
	}
	for _, n := range drop {
		_ = os.Remove(filepath.Join(c.Dirs.CoversDir(), n))
	}
	if s := c.Settings.Get(); len(drop) == 0 || !s.SyncLibrary || s.CoversLocalOnly || s.CloudBase() == "" {
		return
	}
	cl, err := cloud.NewClient(c.Settings.Get().CloudBase())
	if err == nil {
		err = cl.DeleteFiles(ctx, coversCloudDir, drop)
	}
	if err != nil {
		log.Printf("delete cloud images: %v", err)
	}
}
