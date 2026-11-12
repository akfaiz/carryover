package core

import (
	"errors"
	"fmt"
	"strings"

	"github.com/akfaiz/carryover/internal/cloudsync/cloud"
	"github.com/akfaiz/carryover/internal/cloudsync/syncer"
	"github.com/akfaiz/carryover/internal/library"
	"github.com/akfaiz/carryover/internal/saves/pathvar"
)

// NativeProfile names the game's own launch config (no emulator) where an
// emulator id is expected. An empty emulator id means "pick the default".
const NativeProfile = "native"

// SyncKey is the id a profile's saves are stored under: the syncer keys the
// cloud folder, state, locks, backups and safety copies by it. The native
// profile keeps the bare game id, so existing cloud data stays valid; an
// emulator profile gets its own folder. Both ids are UUIDs, so the key is safe
// in file names.
func SyncKey(gameID, emulatorID string) string {
	if emulatorID == "" || emulatorID == NativeProfile {
		return gameID
	}
	return gameID + "." + emulatorID
}

// CloudKey is the id the cloud folder, cloud lock, backups and session marker
// of a profile use. It is the profile's own SyncKey unless the profile shares
// the saves of another emulator of the game (library.GameEmulator.ShareWith).
func CloudKey(g library.Game, emulatorID string) string {
	if ge, ok := g.Emulator(emulatorID); ok && ge.ShareWith != "" {
		return SyncKey(g.ID, ge.ShareWith)
	}
	return SyncKey(g.ID, emulatorID)
}

// CloudKeyOf is CloudKey for a sync key (a game id or "<game id>.<emulator id>").
func (c *Core) CloudKeyOf(key string) string {
	gameID, emuID := ParseKey(key)
	if emuID == "" {
		return key
	}
	g, err := c.Library.Get(gameID)
	if err != nil {
		return key
	}
	return CloudKey(g, emuID)
}

// ParseKey is the inverse of SyncKey: it splits a sync key into the game id
// and the emulator id ("" = native). A plain game id is its own native key.
func ParseKey(key string) (gameID, emulatorID string) {
	gameID, emulatorID, _ = strings.Cut(key, ".")
	return gameID, emulatorID
}

// GameOf returns the game id of a sync key.
func GameOf(key string) string {
	id, _ := ParseKey(key)
	return id
}

// Profiles lists the profiles of a game that are set up on this device: the
// native profile ("" ) when the game has save slots of its own, then each emulator.
func (c *Core) Profiles(g library.Game) []string {
	var out []string
	if len(g.Slots) > 0 {
		out = append(out, "")
	}
	d, _ := g.Device(c.DeviceID())
	for _, ge := range g.Emulators {
		if _, ok := d.Emulators[ge.EmulatorID]; ok {
			out = append(out, ge.EmulatorID)
		}
	}
	return out
}

// ResolveProfile turns what the user asked for (an emulator id or name,
// NativeProfile, or "" for the default) into an emulator id ("" = native).
// The default is the profile played last on this device, else the native one,
// else the only emulator.
func (c *Core) ResolveProfile(g library.Game, want string) (string, error) {
	if want == NativeProfile {
		return "", nil
	}
	if want != "" {
		e, err := c.Library.FindEmulator(want)
		if err != nil {
			return "", fmt.Errorf("unknown emulator %q", want)
		}
		if _, ok := g.Emulator(e.ID); !ok {
			return "", fmt.Errorf("%s is not set up with %s", g.Name, e.Name)
		}
		return e.ID, nil
	}
	d, _ := g.Device(c.DeviceID())
	if last := d.LastEmulator; last != "" {
		if _, ok := g.Emulator(last); ok {
			if _, set := d.Emulators[last]; set {
				return last, nil
			}
		}
	}
	profiles := c.Profiles(g)
	switch {
	case len(profiles) == 0:
		return "", nil // native: Play/Job report what is missing
	case profiles[0] == "" || len(profiles) == 1:
		return profiles[0], nil
	}
	return "", fmt.Errorf("%s has several emulators; choose one: %s", g.Name, strings.Join(c.profileNames(profiles), ", "))
}

func (c *Core) profileNames(ids []string) []string {
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		if id == "" {
			out = append(out, NativeProfile)
		} else if e, err := c.Library.Emulator(id); err == nil {
			out = append(out, e.Name)
		} else {
			out = append(out, id)
		}
	}
	return out
}

// ProfileName is the display name of a profile ("" for the native one).
func (c *Core) ProfileName(emulatorID string) string {
	if emulatorID == "" {
		return ""
	}
	if e, err := c.Library.Emulator(emulatorID); err == nil {
		return e.Name
	}
	return emulatorID
}

// profileData returns the save slots and this device's paths of a profile.
func (c *Core) profileData(g library.Game, emulatorID string) ([]library.SaveSlot, map[string]string, error) {
	d, ok := g.Device(c.DeviceID())
	if !ok {
		return nil, nil, fmt.Errorf("%s is not set up on this device yet", g.Name)
	}
	if emulatorID == "" {
		return g.Slots, d.Paths, nil
	}
	ge, ok := g.Emulator(emulatorID)
	if !ok {
		return nil, nil, fmt.Errorf("%s is not set up with this emulator", g.Name)
	}
	if ge.ShareWith != "" {
		if target, ok := g.Emulator(ge.ShareWith); ok {
			ge.Slots = target.Slots // the shared cloud folder has the target's layout
		}
	}
	ec, ok := d.Emulators[emulatorID]
	if !ok {
		return nil, nil, fmt.Errorf("%s is not set up with %s on this device yet", g.Name, c.ProfileName(emulatorID))
	}
	return ge.Slots, ec.Paths, nil
}

// PathContextFor is PathContext for a profile: {proton:shortcut} resolves to
// the shortcut that profile has on this device.
func (c *Core) PathContextFor(g library.Game, emulatorID string) pathvar.Context {
	ctx := pathvar.Context{SteamRoot: c.SteamRoot()}
	d, ok := g.Device(c.DeviceID())
	if !ok {
		return ctx
	}
	if emulatorID == "" {
		ctx.ShortcutAppID = d.SteamAppID
		return ctx
	}
	ctx.ShortcutAppID = d.Emulators[emulatorID].SteamAppID
	// {ryujinx:...} looks in the emulator's data folder on this device.
	if emu, err := c.Library.Emulator(emulatorID); err == nil {
		if dir := emu.Devices[c.DeviceID()].DataDir; dir != "" {
			if p, err := (pathvar.Context{SteamRoot: ctx.SteamRoot}).Expand(dir); err == nil {
				ctx.RyujinxDir = p
			}
		}
	}
	return ctx
}

// JobFor builds the sync job of one profile of a game on this device.
func (c *Core) JobFor(g library.Game, emulatorID string) (syncer.Job, error) {
	slots, paths, err := c.profileData(g, emulatorID)
	if err != nil {
		return syncer.Job{}, err
	}
	pctx := c.PathContextFor(g, emulatorID)
	job := syncer.Job{GameID: CloudKey(g, emulatorID), StateKey: SyncKey(g.ID, emulatorID)}
	for _, sl := range slots {
		raw := paths[sl.Key]
		if strings.TrimSpace(raw) == "" {
			return syncer.Job{}, fmt.Errorf("save location %q has no path on this device", labelOr(sl))
		}
		p, err := pctx.Expand(raw)
		if err != nil {
			return syncer.Job{}, fmt.Errorf("save location %q: %w", labelOr(sl), err)
		}
		job.Slots = append(job.Slots, syncer.Slot{
			Key:       sl.Key,
			LocalPath: p,
			Filters:   cloud.Filters{Include: sl.Include, Exclude: sl.Exclude},
		})
	}
	if len(job.Slots) == 0 {
		return syncer.Job{}, errors.New("game has no save locations")
	}
	return job, nil
}
