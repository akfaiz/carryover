// Package library stores the game catalog.
package library

import (
	"errors"
	"fmt"
	"maps"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/akfaiz/carryover/internal/appdir"
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
	UpdatedAt   time.Time         `json:"updatedAt"`
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
	Devices    map[string]DeviceConfig `json:"devices"`
	Deleted    bool                    `json:"deleted,omitempty"`
	CreatedAt  time.Time               `json:"createdAt"`
	UpdatedAt  time.Time               `json:"updatedAt"`
}

// Device returns the config for a device (zero value if missing).
func (g Game) Device(id string) (DeviceConfig, bool) {
	d, ok := g.Devices[id]
	return d, ok
}

// Catalog is the serialized library.
type Catalog struct {
	Version int    `json:"version"`
	Games   []Game `json:"games"`
}

// Store is a thread-safe library persisted to a JSON file.
type Store struct {
	mu    sync.Mutex
	path  string
	games map[string]Game
}

// Open loads the library file.
func Open(path string) (*Store, error) {
	s := &Store{path: path, games: map[string]Game{}}
	var c Catalog
	if _, err := appdir.ReadJSON(path, &c); err != nil {
		return nil, fmt.Errorf("load library: %w", err)
	}
	for _, g := range c.Games {
		s.games[g.ID] = normalize(g)
	}
	return s, nil
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
		devs[k] = d
	}
	g.Devices = devs
	g.Slots = append([]SaveSlot(nil), g.Slots...)
	if g.Art != nil {
		g.Art = maps.Clone(g.Art)
	}
	return g
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
	c := Catalog{Version: 1}
	for _, g := range s.games {
		c.Games = append(c.Games, g)
	}
	sort.Slice(c.Games, func(i, j int) bool { return c.Games[i].ID < c.Games[j].ID })
	return c
}

// MergeCatalog merges a remote catalog into the local store and returns whether anything changed.
func (s *Store) MergeCatalog(remote Catalog) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	changed := false
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

func (s *Store) persist() error {
	return appdir.WriteJSONAtomic(s.path, s.catalogLocked())
}
