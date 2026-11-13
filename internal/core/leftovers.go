package core

import (
	"context"
	"errors"
	"fmt"
	"log"
	"path"
	"sort"
	"strings"
	"time"

	"github.com/akfaiz/carryover/internal/cloudsync/cloud"
	"github.com/akfaiz/carryover/internal/cloudsync/syncer"
	"github.com/akfaiz/carryover/internal/library"
)

// Leftover is a cloud save folder that no game in the library uses: the game
// was deleted for good, or the emulator was removed from the game. Its saves
// and backups are still there.
type Leftover struct {
	Key          string // the sync key, also the folder name
	GameID       string
	GameName     string // "" if the library doesn't know the game
	EmulatorID   string // "" for the game's own saves
	EmulatorName string
	Slots        []string
	Files        int
	Bytes        int64
	Time         time.Time // of the last sync, zero if unknown
	Backups      int
}

// isLeftover reports whether no game in the library uses the cloud folder of a
// sync key. The folders of a game in the trash are not leftovers: restoring the
// game needs them.
func (c *Core) isLeftover(key string) bool {
	gameID, emuID := ParseKey(key)
	g, err := c.Library.GetAny(gameID)
	switch {
	case err != nil:
		return true
	case g.Deleted:
		return !g.Restorable()
	case emuID == "":
		// Only a game with save slots of its own syncs its bare-id folder (an
		// emulator-only game, e.g. one recovered from an emulator folder, doesn't).
		return len(g.Slots) == 0
	}
	_, ok := g.Emulator(emuID)
	return !ok
}

// Leftovers lists the cloud save folders no game uses. Without a cloud remote
// there are none.
func (c *Core) Leftovers(ctx context.Context) ([]Leftover, error) {
	if !c.CloudConfigured() {
		return nil, nil
	}
	cl, err := cloud.NewClient(c.Settings.Get().CloudBase())
	if err != nil {
		return nil, err
	}
	dirs, err := cl.ListDirs(ctx, gamesCloudDir)
	if err != nil {
		return nil, fmt.Errorf("list cloud saves: %w", err)
	}
	var out []Leftover
	for _, d := range dirs {
		if validKey(d) != nil || !c.isLeftover(d) {
			continue
		}
		gameID, emuID := ParseKey(d)
		l := Leftover{Key: d, GameID: gameID, EmulatorID: emuID}
		if g, err := c.Library.GetAny(gameID); err == nil {
			l.GameName = g.Name
		}
		if emuID != "" {
			if e, err := c.Library.EmulatorAny(emuID); err == nil {
				l.EmulatorName = e.Name
			}
		}
		var m syncer.Manifest
		if found, err := cl.ReadJSON(ctx, path.Join(gamesCloudDir, d, "manifest.json"), &m); err != nil {
			log.Printf("leftover %s: %v", d, err)
		} else if found {
			sum := m.Summarize()
			l.Files, l.Bytes, l.Time = sum.Files, sum.Bytes, sum.Time
			l.Slots = slotsOf(&m)
		}
		if dirs, files, err := cl.List(ctx, path.Join(gamesCloudDir, d, "backups")); err == nil {
			l.Backups = len(dirs) + len(files)
		}
		out = append(out, l)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Time.After(out[j].Time) })
	return out, nil
}

// CountLeftovers is len(Leftovers) without reading each folder: one cloud call.
func (c *Core) CountLeftovers(ctx context.Context) (int, error) {
	if !c.CloudConfigured() {
		return 0, nil
	}
	cl, err := cloud.NewClient(c.Settings.Get().CloudBase())
	if err != nil {
		return 0, err
	}
	dirs, err := cl.ListDirs(ctx, gamesCloudDir)
	if err != nil {
		return 0, fmt.Errorf("list cloud saves: %w", err)
	}
	n := 0
	for _, d := range dirs {
		if validKey(d) == nil && c.isLeftover(d) {
			n++
		}
	}
	return n, nil
}

// slotsOf returns the sorted save slot keys a manifest has files for.
func slotsOf(m *syncer.Manifest) []string {
	seen := map[string]bool{}
	for f := range m.Files {
		if slot, _, ok := strings.Cut(f, "/"); ok {
			seen[slot] = true
		}
	}
	out := make([]string, 0, len(seen))
	for s := range seen {
		out = append(out, s)
	}
	sort.Strings(out)
	return out
}

// DeleteLeftover permanently deletes the saves and backups in a leftover folder.
func (c *Core) DeleteLeftover(ctx context.Context, key string) error {
	if err := validKey(key); err != nil {
		return err
	}
	if !c.isLeftover(key) {
		return errors.New("a game in the library uses these saves")
	}
	return c.PurgeSaves(ctx, []string{key})
}

// RestoreLeftover brings a leftover folder back into the library and returns
// the id of the game it belongs to. The game is re-created (under its old id)
// with the save slots found in the cloud folder, or the emulator is added to the
// game again. Where the saves live on this device isn't known: the game has to
// be edited to set that before it syncs, and the first sync then downloads the
// cloud saves.
//
// An emulator that was deleted for good can't come back this way: its settings
// are gone, so create it again on the Emulators page and add it to the game.
func (c *Core) RestoreLeftover(ctx context.Context, key string) (string, error) {
	if err := validKey(key); err != nil {
		return "", err
	}
	if !c.isLeftover(key) {
		return "", errors.New("a game in the library uses these saves")
	}
	gameID, emuID := ParseKey(key)
	slots, err := c.cloudSlots(ctx, key)
	if err != nil {
		return "", err
	}
	g, err := c.Library.GetAny(gameID)
	switch {
	case errors.Is(err, library.ErrNotFound):
		g = library.Game{ID: gameID, CreatedAt: time.Now().UTC()}
	case err != nil:
		return "", err
	}
	if g.Name == "" {
		g.Name = "Recovered game " + gameID[:min(8, len(gameID))]
	}
	g.Deleted, g.DeletedAt = false, time.Time{}
	if emuID == "" {
		if len(g.Slots) == 0 {
			g.Slots = slots
		}
	} else {
		e, err := c.Library.EmulatorAny(emuID)
		if err != nil {
			return "", fmt.Errorf("the emulator of these saves no longer exists: create it again and add it to the game")
		}
		if e.Deleted {
			if _, err := c.Library.RestoreEmulator(emuID); err != nil {
				return "", fmt.Errorf("the emulator %s was deleted for good: create it again and add it to the game", e.Name)
			}
		}
		if _, ok := g.Emulator(emuID); !ok {
			g.Emulators = append(g.Emulators, library.GameEmulator{EmulatorID: emuID, Slots: slots})
		}
	}
	dev := c.DeviceID()
	if g.Devices == nil {
		g.Devices = map[string]library.DeviceConfig{}
	}
	if _, ok := g.Devices[dev]; !ok {
		g.Devices[dev] = library.DeviceConfig{DeviceName: c.Settings.Get().DeviceName, Paths: map[string]string{}}
	}
	saved, err := c.Library.Save(g, dev)
	if err != nil {
		return "", err
	}
	return saved.ID, nil
}

// cloudSlots returns the save slots a cloud folder holds, from its manifest or,
// when that is missing, from the folders under current/.
func (c *Core) cloudSlots(ctx context.Context, key string) ([]library.SaveSlot, error) {
	cl, err := cloud.NewClient(c.Settings.Get().CloudBase())
	if err != nil {
		return nil, err
	}
	var m syncer.Manifest
	found, err := cl.ReadJSON(ctx, path.Join(gamesCloudDir, key, "manifest.json"), &m)
	if err != nil {
		return nil, err
	}
	var names []string
	if found {
		names = slotsOf(&m)
	}
	if len(names) == 0 {
		if names, err = cl.ListDirs(ctx, path.Join(gamesCloudDir, key, "current")); err != nil {
			return nil, err
		}
	}
	slots := make([]library.SaveSlot, 0, len(names))
	for _, n := range names {
		slots = append(slots, library.SaveSlot{Key: n, Label: n})
	}
	return slots, nil
}

// ExpireTrash reduces what was deleted longer than library.TrashRetention ago
// to its tombstone, and removes the cover images of those games. Failures are
// only logged: it is housekeeping that runs again next time.
func (c *Core) ExpireTrash(ctx context.Context) {
	expired, err := c.Library.ExpireTrash(time.Now().UTC())
	if err != nil {
		log.Printf("expire trash: %v", err)
	}
	for _, g := range expired {
		c.DropImages(ctx, g)
	}
}
