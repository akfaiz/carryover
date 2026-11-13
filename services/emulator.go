package services

import (
	"context"
	"errors"
	"slices"
	"strings"
	"time"

	"github.com/akfaiz/carryover/internal/cloudsync/syncer"
	"github.com/akfaiz/carryover/internal/core"
	"github.com/akfaiz/carryover/internal/library"
	"github.com/akfaiz/carryover/internal/saves/emumanifest"
)

// EmulatorService manages the emulators games can be launched with. An
// emulator is defined once; each device says where it is installed.
type EmulatorService struct {
	c     *core.Core
	bus   *Bus
	store *emumanifest.Store // downloaded per-game save manifests
}

// NewEmulatorService creates the service. bus may be nil.
func NewEmulatorService(c *core.Core, bus *Bus) *EmulatorService {
	return &EmulatorService{c: c, bus: bus, store: &emumanifest.Store{Dir: c.Dirs.ManifestsDir()}}
}

// EmulatorView is an emulator from this device's perspective.
type EmulatorView struct {
	ID     string                 `json:"id"`
	Name   string                 `json:"name"`
	Preset string                 `json:"preset"`
	Device library.EmulatorDevice `json:"device"`
	// SetUp: it has an executable on this device.
	SetUp bool `json:"setUp"`
	// Games is how many games use it.
	Games int `json:"games"`
}

// EmulatorSetup is the editable part of an emulator: its name and this
// device's setup of it.
type EmulatorSetup struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Exe         string            `json:"exe"`
	Args        string            `json:"args"` // {rom} is replaced by the game's ROM
	WorkDir     string            `json:"workDir"`
	Env         map[string]string `json:"env"`
	WaitProcess string            `json:"waitProcess"`
	// Preset is the kind of emulator (see Presets) and DataDir its data folder
	// on this device; together they let Carryover suggest save locations.
	Preset  string `json:"preset"`
	DataDir string `json:"dataDir"`
}

func (s *EmulatorService) view(e library.Emulator, games []library.Game) EmulatorView {
	d := e.Devices[s.c.DeviceID()]
	v := EmulatorView{ID: e.ID, Name: e.Name, Preset: e.Preset, Device: d, SetUp: d.Exe != ""}
	for _, g := range games {
		if _, ok := g.Emulator(e.ID); ok {
			v.Games++
		}
	}
	return v
}

// List returns all emulators.
func (s *EmulatorService) List() []EmulatorView {
	games := s.c.Library.List()
	ems := s.c.Library.Emulators()
	out := make([]EmulatorView, 0, len(ems))
	for _, e := range ems {
		out = append(out, s.view(e, games))
	}
	return out
}

// Get returns one emulator.
func (s *EmulatorService) Get(id string) (EmulatorView, error) {
	e, err := s.c.Library.Emulator(id)
	if err != nil {
		return EmulatorView{}, err
	}
	return s.view(e, s.c.Library.List()), nil
}

// Save creates or updates an emulator and this device's setup of it.
func (s *EmulatorService) Save(in EmulatorSetup) (EmulatorView, error) {
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return EmulatorView{}, errors.New("emulator name is required")
	}
	if _, ok := emumanifest.PresetByID(in.Preset); in.Preset != "" && !ok {
		return EmulatorView{}, errors.New("unknown emulator type")
	}
	for _, o := range s.c.Library.Emulators() {
		if o.ID != in.ID && strings.EqualFold(o.Name, name) {
			return EmulatorView{}, errors.New("an emulator with this name already exists")
		}
	}
	devID := s.c.DeviceID()
	saved, err := s.c.Library.SaveEmulator(library.Emulator{
		ID: in.ID, Name: name, Preset: in.Preset,
		Devices: map[string]library.EmulatorDevice{devID: {
			Exe: strings.TrimSpace(in.Exe), Args: in.Args, WorkDir: strings.TrimSpace(in.WorkDir),
			Env: in.Env, WaitProcess: strings.TrimSpace(in.WaitProcess), DataDir: strings.TrimSpace(in.DataDir),
		}},
	}, devID)
	if err != nil {
		return EmulatorView{}, err
	}
	s.bus.libraryChanged("")
	s.publish()
	return s.view(saved, s.c.Library.List()), nil
}

// Delete moves an emulator to the trash on all devices. Games keep their
// profile (it just can't be launched) and the cloud saves are kept; Restore
// brings it back.
func (s *EmulatorService) Delete(id string) error {
	if err := s.c.Library.DeleteEmulator(id); err != nil {
		return err
	}
	s.bus.libraryChanged("")
	s.publish()
	return nil
}

// DeleteForever deletes an emulator, live or in the trash, for good: it is
// taken out of every game that uses it and its settings can't be restored.
// The games' cloud saves made with it stay and show up as leftovers.
func (s *EmulatorService) DeleteForever(id string) error { return s.deleteForever(id, false) }

// DeleteWithSaves is DeleteForever that also permanently deletes the cloud
// saves and backups made with the emulator, and their sync state and safety
// copies on this device. The emulator stays if the cloud saves can't be deleted.
//
// A game whose other emulator uses this one's cloud save keeps that cloud
// folder (it holds the other emulator's save too). The other emulator gets a
// cloud save of its own from then on and asks before syncing, like when it
// stops sharing in the game editor.
func (s *EmulatorService) DeleteWithSaves(id string) error { return s.deleteForever(id, true) }

func (s *EmulatorService) deleteForever(id string, saves bool) error {
	if _, err := s.c.Library.EmulatorAny(id); err != nil {
		return err
	}
	keep := map[string]bool{} // cloud folders another emulator still uses
	var reset []string        // sync keys of the emulators that shared this one's
	// Games in the trash lose the emulator too (they can be restored).
	for _, g := range append(s.c.Library.List(), s.c.Library.Trash()...) {
		if _, ok := g.Emulator(id); !ok {
			continue
		}
		for _, ge := range g.Emulators {
			if ge.ShareWith == id {
				keep[core.SyncKey(g.ID, id)] = true
				reset = append(reset, core.SyncKey(g.ID, ge.EmulatorID))
			}
		}
	}
	if saves {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()
		keys, err := s.c.SaveKeysOfEmulator(ctx, id)
		if err != nil {
			return err
		}
		keys = slices.DeleteFunc(keys, func(k string) bool { return keep[k] })
		if err := s.c.PurgeSaves(ctx, keys); err != nil {
			return err
		}
	}
	if _, err := s.c.Library.DeleteEmulatorEverywhere(id); err != nil {
		return err
	}
	for _, k := range reset {
		_ = s.c.States.Save(k, syncer.State{})
	}
	s.bus.libraryChanged("")
	s.publish()
	return nil
}

// publish uploads the library catalog in the background, like
// LibraryService.publish.
func (s *EmulatorService) publish() {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		if err := s.c.SyncLibrary(ctx); err == nil {
			s.bus.libraryChanged("")
		}
	}()
}
