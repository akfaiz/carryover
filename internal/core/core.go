// Package core wires PlayRelay's components together. It is used by both the
// GUI services and the command line modes.
package core

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"github.com/akfaiz/playrelay/internal/appdir"
	"github.com/akfaiz/playrelay/internal/cloud"
	"github.com/akfaiz/playrelay/internal/library"
	"github.com/akfaiz/playrelay/internal/pathvar"
	"github.com/akfaiz/playrelay/internal/steam"
	"github.com/akfaiz/playrelay/internal/syncer"
)

// Core holds application state.
type Core struct {
	Dirs     appdir.Dirs
	Settings *appdir.SettingsStore
	Library  *library.Store
	States   *syncer.StateStore

	steamOnce sync.Once
	steamRoot string

	evMu    sync.RWMutex
	onEvent func(syncer.Event)

	libMu sync.Mutex // serializes SyncLibrary (read, merge, write)
}

// New opens PlayRelay's data directory.
func New() (*Core, error) {
	dirs, err := appdir.Resolve()
	if err != nil {
		return nil, err
	}
	settings, err := appdir.OpenSettings(dirs.SettingsFile())
	if err != nil {
		return nil, err
	}
	lib, err := library.Open(dirs.LibraryFile())
	if err != nil {
		return nil, err
	}
	if err := cloud.Init(dirs.RcloneConfig()); err != nil {
		return nil, err
	}
	return &Core{
		Dirs:     dirs,
		Settings: settings,
		Library:  lib,
		States:   &syncer.StateStore{Dir: dirs.StateDir()},
	}, nil
}

// DeviceID returns this device's id.
func (c *Core) DeviceID() string { return c.Settings.Get().DeviceID }

// SetEventHandler sets the receiver of sync progress events.
func (c *Core) SetEventHandler(fn func(syncer.Event)) {
	c.evMu.Lock()
	c.onEvent = fn
	c.evMu.Unlock()
}

func (c *Core) emit(ev syncer.Event) {
	c.evMu.RLock()
	fn := c.onEvent
	c.evMu.RUnlock()
	if fn != nil {
		fn(ev)
	}
}

// SteamRoot returns the detected Steam installation ("" if none).
func (c *Core) SteamRoot() string {
	c.steamOnce.Do(func() { c.steamRoot = steam.FindRoot() })
	return c.steamRoot
}

// CloudConfigured reports whether a remote is selected.
func (c *Core) CloudConfigured() bool { return c.Settings.Get().CloudBase() != "" }

// Engine returns a sync engine for the configured remote.
func (c *Core) Engine() (*syncer.Engine, error) {
	s := c.Settings.Get()
	cl, err := cloud.NewClient(s.CloudBase())
	if err != nil {
		return nil, err
	}
	return &syncer.Engine{
		Client:    cl,
		States:    c.States,
		Device:    syncer.Device{ID: s.DeviceID, Name: s.DeviceName},
		Retention: s.BackupRetention,
		OnEvent:   c.emit,
	}, nil
}

// PathContext returns the token context for a game on this device.
func (c *Core) PathContext(g library.Game) pathvar.Context {
	ctx := pathvar.Context{SteamRoot: c.SteamRoot()}
	if d, ok := g.Device(c.DeviceID()); ok {
		ctx.ShortcutAppID = d.SteamAppID
	}
	return ctx
}

// Job builds the sync job for a game on this device.
func (c *Core) Job(g library.Game) (syncer.Job, error) {
	d, ok := g.Device(c.DeviceID())
	if !ok {
		return syncer.Job{}, fmt.Errorf("%s is not set up on this device yet", g.Name)
	}
	pctx := c.PathContext(g)
	job := syncer.Job{GameID: g.ID}
	for _, sl := range g.Slots {
		raw := d.Paths[sl.Key]
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

func labelOr(s library.SaveSlot) string {
	if s.Label != "" {
		return s.Label
	}
	return s.Key
}

const catalogPath = "library.json"

// SyncLibrary merges the cloud catalog into the local library and uploads the
// result. Calls are serialized so a background publish and a manual sync can't
// overwrite each other's upload.
func (c *Core) SyncLibrary(ctx context.Context) error {
	c.libMu.Lock()
	defer c.libMu.Unlock()
	s := c.Settings.Get()
	if !s.SyncLibrary || s.CloudBase() == "" {
		return nil
	}
	cl, err := cloud.NewClient(s.CloudBase())
	if err != nil {
		return err
	}
	var remote library.Catalog
	found, err := cl.ReadJSON(ctx, catalogPath, &remote)
	if err != nil {
		return fmt.Errorf("read cloud library: %w", err)
	}
	if found {
		if _, err := c.Library.MergeCatalog(remote); err != nil {
			return err
		}
	}
	local := c.Library.Catalog()
	if found && catalogsEqual(local, remote) {
		return nil
	}
	return cl.WriteJSON(ctx, catalogPath, local)
}

func catalogsEqual(a, b library.Catalog) bool {
	if len(a.Games) != len(b.Games) {
		return false
	}
	idx := map[string]library.Game{}
	for _, g := range b.Games {
		idx[g.ID] = g
	}
	for _, g := range a.Games {
		o, ok := idx[g.ID]
		if !ok || !o.UpdatedAt.Equal(g.UpdatedAt) || o.Deleted != g.Deleted || len(o.Devices) != len(g.Devices) {
			return false
		}
		for k, d := range g.Devices {
			if !o.Devices[k].UpdatedAt.Equal(d.UpdatedAt) {
				return false
			}
		}
	}
	return true
}

// SelfExecutable returns the path Steam should launch for PlayRelay.
func SelfExecutable() (string, error) {
	if p := os.Getenv("APPIMAGE"); p != "" && runtime.GOOS == "linux" {
		return p, nil
	}
	p, err := os.Executable()
	if err != nil {
		return "", err
	}
	if r, err := filepath.EvalSymlinks(p); err == nil {
		p = r
	}
	return p, nil
}
