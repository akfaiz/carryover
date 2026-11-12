package services

import (
	"context"
	"errors"
	"strings"
	"time"

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

// Delete removes an emulator. Games keep their profile (it just can't be
// launched) and the cloud saves are kept.
func (s *EmulatorService) Delete(id string) error {
	if err := s.c.Library.DeleteEmulator(id); err != nil {
		return err
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
