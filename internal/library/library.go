// Package library stores the game catalog.
package library

import (
	"errors"
	"fmt"
	"maps"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/akfaiz/carryover/internal/app/appdir"
)

// SaveSlot is a logical save location shared across devices. Each device maps
// the slot key to its own local path.
type SaveSlot struct {
	Key     string   `json:"key"`
	Label   string   `json:"label"`
	Include []string `json:"include"` // rclone filter globs; empty = everything
	Exclude []string `json:"exclude"`
}

// LaunchMode controls how the game is started.
type LaunchMode string

const (
	// LaunchDirect runs Exe directly.
	LaunchDirect LaunchMode = "direct"
	// LaunchSteamWrap is used with a Steam shortcut whose launch options are
	// `carryover run <id> -- %command%` (e.g. Proton games on Linux / Steam Deck).
	LaunchSteamWrap LaunchMode = "steamwrap"
	// LaunchHeroic starts a game Heroic installed through Heroic, so it keeps
	// Heroic's settings and, on Linux, runs in Heroic's Wine prefix. Exe is the game's executable inside
	// Heroic's install folder; it identifies the game and, by its name, the
	// process to wait for.
	LaunchHeroic LaunchMode = "heroic"
)

// DeviceConfig is the per-device part of a game.
type DeviceConfig struct {
	DeviceName  string            `json:"deviceName"`
	Paths       map[string]string `json:"paths"` // SaveSlot.Key -> path (may contain tokens)
	Exe         string            `json:"exe"`
	Args        string            `json:"args"`
	WorkDir     string            `json:"workDir"`
	Env         map[string]string `json:"env"`
	WaitProcess string            `json:"waitProcess"`
	LaunchMode  LaunchMode        `json:"launchMode"`
	SteamAppID  string            `json:"steamAppId"` // shortcut app id if added to Steam on this device
	// Emulators holds this device's data for each emulator the game is set up
	// with, by Emulator.ID. The fields above are the native (no emulator) profile.
	Emulators map[string]EmulatorConfig `json:"emulators,omitempty"`
	// LastEmulator is the profile last played on this device ("" = native).
	LastEmulator string    `json:"lastEmulator,omitempty"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

// EmulatorConfig is a game's data for one emulator on one device.
type EmulatorConfig struct {
	Rom        string            `json:"rom"`   // game file/ID passed to the emulator (may contain tokens)
	Paths      map[string]string `json:"paths"` // GameEmulator.Slots key -> path
	SteamAppID string            `json:"steamAppId,omitempty"`
}

// GameEmulator marks a game as playable with an emulator. Each emulator has
// its own save slots because the save formats differ, and its own cloud save.
type GameEmulator struct {
	EmulatorID string     `json:"emulatorId"`
	Slots      []SaveSlot `json:"slots"`
	// ShareWith, if set, is another emulator of the game whose cloud save this
	// one uses too (opt-in: only safe when both read the same save layout).
	// The save slots are then the other emulator's; each device still sets
	// this emulator's own local paths.
	ShareWith string `json:"shareWith,omitempty"`
}

// EmulatorDevice is how an emulator is started on one device.
type EmulatorDevice struct {
	Exe         string            `json:"exe"`
	Args        string            `json:"args"` // template; {rom} is replaced by the game's Rom
	WorkDir     string            `json:"workDir"`
	Env         map[string]string `json:"env"`
	WaitProcess string            `json:"waitProcess"`
	// DataDir is the emulator's data folder on this device (the root its save
	// locations are relative to); may contain path tokens.
	DataDir   string    `json:"dataDir,omitempty"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// Emulator is a named emulator shared by all games; each device says where it
// is installed.
type Emulator struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	// Preset is the kind of emulator (emumanifest.Preset.ID), which tells where
	// its games' saves are; "" if not set.
	Preset    string                    `json:"preset,omitempty"`
	Devices   map[string]EmulatorDevice `json:"devices"`
	Deleted   bool                      `json:"deleted,omitempty"`
	CreatedAt time.Time                 `json:"createdAt"`
	UpdatedAt time.Time                 `json:"updatedAt"`
}

// Device returns the emulator's setup on a device.
func (e Emulator) Device(id string) (EmulatorDevice, bool) {
	d, ok := e.Devices[id]
	return d, ok
}

// Game is a library entry.
type Game struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Cover string `json:"cover"` // file name in the covers dir
	// Art is extra Steam artwork by kind ("wide", "hero", "logo", "icon") ->
	// file name in the covers dir. Written to Steam's grid folder when the game
	// is added to Steam.
	Art map[string]string `json:"art,omitempty"`
	// StoreAppID is the game's Steam store app id (if it exists on Steam),
	// used for its title and cover art. Shared across devices. Not to be
	// confused with DeviceConfig.SteamAppID, the per-device shortcut id.
	StoreAppID string                  `json:"storeAppId,omitempty"`
	Slots      []SaveSlot              `json:"slots"`
	Emulators  []GameEmulator          `json:"emulators,omitempty"`
	Devices    map[string]DeviceConfig `json:"devices"`
	Deleted    bool                    `json:"deleted,omitempty"`
	CreatedAt  time.Time               `json:"createdAt"`
	UpdatedAt  time.Time               `json:"updatedAt"`
}

// Emulator returns the game's entry for an emulator id.
func (g Game) Emulator(id string) (GameEmulator, bool) {
	for _, e := range g.Emulators {
		if e.EmulatorID == id {
			return e, true
		}
	}
	return GameEmulator{}, false
}

// Device returns the config for a device (zero value if missing).
func (g Game) Device(id string) (DeviceConfig, bool) {
	d, ok := g.Devices[id]
	return d, ok
}

// CatalogVersion is the newest catalog format this build understands. A newer
// catalog is never read or rewritten, so an old client can't drop its fields.
const CatalogVersion = 2

// ErrNewerCatalog is returned for a catalog written by a newer Carryover.
var ErrNewerCatalog = errors.New("the library was written by a newer version of Carryover; update this device")

// Catalog is the serialized library.
type Catalog struct {
	Version   int        `json:"version"`
	Games     []Game     `json:"games"`
	Emulators []Emulator `json:"emulators,omitempty"`
}

// Store is a thread-safe library persisted to a JSON file.
type Store struct {
	mu        sync.Mutex
	path      string
	games     map[string]Game
	emulators map[string]Emulator
}

// Open loads the library file.
func Open(path string) (*Store, error) {
	s := &Store{path: path, games: map[string]Game{}, emulators: map[string]Emulator{}}
	var c Catalog
	if _, err := appdir.ReadJSON(path, &c); err != nil {
		return nil, fmt.Errorf("load library: %w", err)
	}
	if c.Version > CatalogVersion {
		return nil, ErrNewerCatalog
	}
	for _, g := range c.Games {
		s.games[g.ID] = normalize(g)
	}
	for _, e := range c.Emulators {
		s.emulators[e.ID] = normalizeEmulator(e)
	}
	return s, nil
}

func normalizeEmulator(e Emulator) Emulator {
	if e.Devices == nil {
		e.Devices = map[string]EmulatorDevice{}
	}
	return e
}

func normalize(g Game) Game {
	if g.Devices == nil {
		g.Devices = map[string]DeviceConfig{}
	}
	for k, d := range g.Devices {
		if d.Paths == nil {
			d.Paths = map[string]string{}
		}
		if d.LaunchMode == "" {
			d.LaunchMode = LaunchDirect
		}
		for id, ec := range d.Emulators {
			if ec.Paths == nil {
				ec.Paths = map[string]string{}
				d.Emulators[id] = ec
			}
		}
		g.Devices[k] = d
	}
	return g
}

// ErrNotFound is returned for unknown game ids.
var ErrNotFound = errors.New("game not found")

// List returns non-deleted games sorted by name.
func (s *Store) List() []Game {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []Game
	for _, g := range s.games {
		if !g.Deleted {
			out = append(out, clone(g))
		}
	}
	sort.Slice(out, func(i, j int) bool { return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name) })
	return out
}

// Get returns a game by id.
func (s *Store) Get(id string) (Game, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	g, ok := s.games[id]
	if !ok || g.Deleted {
		return Game{}, ErrNotFound
	}
	return clone(g), nil
}

// clone deep-copies the maps of a game so callers can't mutate store state.
func clone(g Game) Game {
	devs := make(map[string]DeviceConfig, len(g.Devices))
	for k, d := range g.Devices {
		paths := make(map[string]string, len(d.Paths))
		maps.Copy(paths, d.Paths)
		d.Paths = paths
		if d.Env != nil {
			env := make(map[string]string, len(d.Env))
			maps.Copy(env, d.Env)
			d.Env = env
		}
		if d.Emulators != nil {
			ems := make(map[string]EmulatorConfig, len(d.Emulators))
			for id, ec := range d.Emulators {
				ec.Paths = maps.Clone(ec.Paths)
				ems[id] = ec
			}
			d.Emulators = ems
		}
		devs[k] = d
	}
	g.Devices = devs
	g.Slots = cloneSlots(g.Slots)
	if g.Emulators != nil {
		ems := make([]GameEmulator, len(g.Emulators))
		for i, e := range g.Emulators {
			e.Slots = cloneSlots(e.Slots)
			ems[i] = e
		}
		g.Emulators = ems
	}
	if g.Art != nil {
		g.Art = maps.Clone(g.Art)
	}
	return g
}

func cloneSlots(in []SaveSlot) []SaveSlot {
	out := append([]SaveSlot(nil), in...)
	for i := range out {
		out[i].Include = slices.Clone(out[i].Include)
		out[i].Exclude = slices.Clone(out[i].Exclude)
	}
	return out
}

// Find looks up a game by id or (case-insensitive) exact name.
func (s *Store) Find(idOrName string) (Game, error) {
	if g, err := s.Get(idOrName); err == nil {
		return g, nil
	}
	for _, g := range s.List() {
		if strings.EqualFold(g.Name, idOrName) {
			return g, nil
		}
	}
	return Game{}, ErrNotFound
}

// Save creates or updates a game; deviceID's config is stamped with the current time.
func (s *Store) Save(g Game, deviceID string) (Game, error) {
	if strings.TrimSpace(g.Name) == "" {
		return Game{}, errors.New("game name is required")
	}
	if err := validateSlots(g.Slots); err != nil {
		return Game{}, err
	}
	if err := validateEmulators(g.Emulators); err != nil {
		return Game{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().UTC()
	if g.ID == "" {
		g.ID = uuid.NewString()
		g.CreatedAt = now
	} else if old, ok := s.games[g.ID]; ok {
		g.CreatedAt = old.CreatedAt
		// Keep other devices' configs as stored; only this device's entry comes from the caller.
		merged := map[string]DeviceConfig{}
		maps.Copy(merged, old.Devices)
		if d, ok := g.Devices[deviceID]; ok {
			merged[deviceID] = d
		}
		g.Devices = merged
	}
	g.UpdatedAt = now
	g = normalize(g)
	if d, ok := g.Devices[deviceID]; ok {
		d.UpdatedAt = now
		g.Devices[deviceID] = d
	}
	s.games[g.ID] = g
	return g, s.persist()
}

// UpdateDevice applies fn to this device's config of a game.
func (s *Store) UpdateDevice(id, deviceID string, fn func(*DeviceConfig)) (Game, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	g, ok := s.games[id]
	if !ok || g.Deleted {
		return Game{}, ErrNotFound
	}
	d := g.Devices[deviceID]
	if d.Paths == nil {
		d.Paths = map[string]string{}
	}
	fn(&d)
	d.UpdatedAt = time.Now().UTC()
	g.Devices[deviceID] = d
	s.games[id] = g
	return g, s.persist()
}

// Delete marks a game deleted (tombstone, so the deletion syncs to other devices).
func (s *Store) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	g, ok := s.games[id]
	if !ok {
		return ErrNotFound
	}
	g.Deleted = true
	g.UpdatedAt = time.Now().UTC()
	s.games[id] = g
	return s.persist()
}

// Catalog returns the full catalog including tombstones.
func (s *Store) Catalog() Catalog {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.catalogLocked()
}

func (s *Store) catalogLocked() Catalog {
	c := Catalog{Version: CatalogVersion}
	for _, g := range s.games {
		c.Games = append(c.Games, g)
	}
	sort.Slice(c.Games, func(i, j int) bool { return c.Games[i].ID < c.Games[j].ID })
	for _, e := range s.emulators {
		c.Emulators = append(c.Emulators, e)
	}
	sort.Slice(c.Emulators, func(i, j int) bool { return c.Emulators[i].ID < c.Emulators[j].ID })
	return c
}

// MergeCatalog merges a remote catalog into the local store and returns whether anything changed.
func (s *Store) MergeCatalog(remote Catalog) (bool, error) {
	if remote.Version > CatalogVersion {
		return false, ErrNewerCatalog
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	changed := false
	for _, re := range remote.Emulators {
		re = normalizeEmulator(re)
		le, ok := s.emulators[re.ID]
		if !ok {
			s.emulators[re.ID] = re
			changed = true
			continue
		}
		if m := MergeEmulator(le, re); !emulatorEqual(m, le) {
			s.emulators[re.ID] = m
			changed = true
		}
	}
	for _, rg := range remote.Games {
		rg = normalize(rg)
		lg, ok := s.games[rg.ID]
		if !ok {
			s.games[rg.ID] = rg
			changed = true
			continue
		}
		m := MergeGame(lg, rg)
		if !gameEqual(m, lg) {
			s.games[rg.ID] = m
			changed = true
		}
	}
	if !changed {
		return false, nil
	}
	return true, s.persist()
}

// MergeGame merges two versions of the same game: game-level fields from the
// newer one, and each device config independently by its own timestamp.
func MergeGame(a, b Game) Game {
	out := a
	if b.UpdatedAt.After(a.UpdatedAt) {
		out = b
	}
	out.Devices = map[string]DeviceConfig{}
	maps.Copy(out.Devices, a.Devices)
	for k, v := range b.Devices {
		if cur, ok := out.Devices[k]; !ok || v.UpdatedAt.After(cur.UpdatedAt) {
			out.Devices[k] = v
		}
	}
	if a.CreatedAt.Before(b.CreatedAt) && !a.CreatedAt.IsZero() {
		out.CreatedAt = a.CreatedAt
	}
	return out
}

func gameEqual(a, b Game) bool {
	if !a.UpdatedAt.Equal(b.UpdatedAt) || a.Deleted != b.Deleted || len(a.Devices) != len(b.Devices) {
		return false
	}
	for k, v := range a.Devices {
		if w, ok := b.Devices[k]; !ok || !w.UpdatedAt.Equal(v.UpdatedAt) {
			return false
		}
	}
	return true
}

// MergeEmulator merges two versions of an emulator like MergeGame does for games.
func MergeEmulator(a, b Emulator) Emulator {
	out := a
	if b.UpdatedAt.After(a.UpdatedAt) {
		out = b
	}
	out.Devices = map[string]EmulatorDevice{}
	maps.Copy(out.Devices, a.Devices)
	for k, v := range b.Devices {
		if cur, ok := out.Devices[k]; !ok || v.UpdatedAt.After(cur.UpdatedAt) {
			out.Devices[k] = v
		}
	}
	if a.CreatedAt.Before(b.CreatedAt) && !a.CreatedAt.IsZero() {
		out.CreatedAt = a.CreatedAt
	}
	return out
}

func emulatorEqual(a, b Emulator) bool {
	if !a.UpdatedAt.Equal(b.UpdatedAt) || a.Deleted != b.Deleted || len(a.Devices) != len(b.Devices) {
		return false
	}
	for k, v := range a.Devices {
		if w, ok := b.Devices[k]; !ok || !w.UpdatedAt.Equal(v.UpdatedAt) {
			return false
		}
	}
	return true
}

func cloneEmulator(e Emulator) Emulator {
	devs := make(map[string]EmulatorDevice, len(e.Devices))
	for k, d := range e.Devices {
		d.Env = maps.Clone(d.Env)
		devs[k] = d
	}
	e.Devices = devs
	return e
}

// Emulators returns the non-deleted emulators sorted by name.
func (s *Store) Emulators() []Emulator {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []Emulator
	for _, e := range s.emulators {
		if !e.Deleted {
			out = append(out, cloneEmulator(e))
		}
	}
	sort.Slice(out, func(i, j int) bool { return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name) })
	return out
}

// Emulator returns an emulator by id.
func (s *Store) Emulator(id string) (Emulator, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.emulators[id]
	if !ok || e.Deleted {
		return Emulator{}, ErrNotFound
	}
	return cloneEmulator(e), nil
}

// FindEmulator looks up an emulator by id or (case-insensitive) exact name.
func (s *Store) FindEmulator(idOrName string) (Emulator, error) {
	if e, err := s.Emulator(idOrName); err == nil {
		return e, nil
	}
	for _, e := range s.Emulators() {
		if strings.EqualFold(e.Name, idOrName) {
			return e, nil
		}
	}
	return Emulator{}, ErrNotFound
}

// SaveEmulator creates or updates an emulator; deviceID's setup is stamped
// with the current time and other devices' setups are kept as stored.
func (s *Store) SaveEmulator(e Emulator, deviceID string) (Emulator, error) {
	if strings.TrimSpace(e.Name) == "" {
		return Emulator{}, errors.New("emulator name is required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().UTC()
	if e.ID == "" {
		e.ID = uuid.NewString()
		e.CreatedAt = now
	} else if old, ok := s.emulators[e.ID]; ok {
		e.CreatedAt = old.CreatedAt
		merged := map[string]EmulatorDevice{}
		maps.Copy(merged, old.Devices)
		if d, ok := e.Devices[deviceID]; ok {
			merged[deviceID] = d
		}
		e.Devices = merged
	}
	e.UpdatedAt = now
	e = normalizeEmulator(cloneEmulator(e))
	if d, ok := e.Devices[deviceID]; ok {
		d.UpdatedAt = now
		e.Devices[deviceID] = d
	}
	s.emulators[e.ID] = e
	return cloneEmulator(e), s.persist()
}

// DeleteEmulator marks an emulator deleted (tombstone). Games keep their
// profile and cloud saves; the profile just can't be launched.
func (s *Store) DeleteEmulator(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.emulators[id]
	if !ok {
		return ErrNotFound
	}
	e.Deleted = true
	e.UpdatedAt = time.Now().UTC()
	s.emulators[id] = e
	return s.persist()
}

// validateEmulators checks a game's emulator entries: no duplicates, an id
// that is safe inside a file name, and valid slots.
func validateEmulators(ems []GameEmulator) error {
	seen := map[string]bool{}
	for _, e := range ems {
		if e.EmulatorID == "" || strings.ContainsAny(e.EmulatorID, `/\:*?"<>|. `) {
			return fmt.Errorf("invalid emulator id %q", e.EmulatorID)
		}
		if seen[e.EmulatorID] {
			return fmt.Errorf("emulator %q is added twice", e.EmulatorID)
		}
		seen[e.EmulatorID] = true
		if err := validateSlots(e.Slots); err != nil {
			return err
		}
	}
	for _, e := range ems {
		if e.ShareWith == "" {
			continue
		}
		var target *GameEmulator
		for i := range ems {
			if ems[i].EmulatorID == e.ShareWith {
				target = &ems[i]
			}
		}
		if target == nil || target.EmulatorID == e.EmulatorID || target.ShareWith != "" {
			return fmt.Errorf("emulator %q can't share saves with %q", e.EmulatorID, e.ShareWith)
		}
	}
	return nil
}

func validateSlots(slots []SaveSlot) error {
	seen := map[string]bool{}
	for _, sl := range slots {
		k := strings.TrimSpace(sl.Key)
		if k == "" {
			return errors.New("save slot key is required")
		}
		if strings.ContainsAny(k, `/\:*?"<>|`) {
			return fmt.Errorf("save slot key %q contains invalid characters", k)
		}
		if seen[k] {
			return fmt.Errorf("duplicate save slot key %q", k)
		}
		seen[k] = true
	}
	return nil
}

// persist writes the library. Another Carryover process (the GUI and a `run`
// wrapper are separate processes) may have written the file since this one
// loaded it, so what is on disk is merged in first, by timestamp, instead of
// being overwritten.
func (s *Store) persist() error {
	var disk Catalog
	if ok, err := appdir.ReadJSON(s.path, &disk); err == nil && ok {
		if disk.Version > CatalogVersion {
			return ErrNewerCatalog
		}
		for _, de := range disk.Emulators {
			de = normalizeEmulator(de)
			if me, ok := s.emulators[de.ID]; ok {
				s.emulators[de.ID] = MergeEmulator(me, de)
			} else {
				s.emulators[de.ID] = de
			}
		}
		for _, dg := range disk.Games {
			dg = normalize(dg)
			if mg, ok := s.games[dg.ID]; ok {
				s.games[dg.ID] = MergeGame(mg, dg)
			} else {
				s.games[dg.ID] = dg
			}
		}
	}
	return appdir.WriteJSONAtomic(s.path, s.catalogLocked())
}
