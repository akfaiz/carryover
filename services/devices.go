package services

import (
	"errors"
	"sort"
	"strings"
	"time"

	"github.com/akfaiz/carryover/internal/core"
	"github.com/akfaiz/carryover/internal/library"
)

// Errors of the device operations.
var (
	// ErrThisDevice: the operation names this device itself.
	ErrThisDevice = core.ErrThisDevice
	// ErrUnknownDevice: the library has no settings of that device.
	ErrUnknownDevice = library.ErrUnknownDevice
	// ErrSessionsOpen: a play session hasn't finished (ReuseDevice).
	ErrSessionsOpen = core.ErrSessionsOpen
	// ErrSyncRunning: a sync is running (ReuseDevice).
	ErrSyncRunning = errors.New("a sync is running; try again once it is done")
)

// DeviceView is a device the library holds settings of.
type DeviceView struct {
	ID string `json:"id"`
	// Name is the device's name; "" when unknown (a device that set up only
	// emulators, in a catalog written before devices registered).
	Name string `json:"name"`
	// Platform is the GOOS its Carryover runs on: windows, linux, darwin or
	// android ("" if unknown).
	Platform string `json:"platform,omitempty"`
	// LastSeen is when its settings last changed (zero for this device when
	// it has none yet).
	LastSeen time.Time `json:"lastSeen"`
	// Games and Emulators count what is set up on it.
	Games     int `json:"games"`
	Emulators int `json:"emulators"`
	// This marks this device.
	This bool `json:"this"`
	// SameName marks another device with this device's name: likely this
	// device before the app was reinstalled (or its data cleared), which gave
	// it a new id. ReuseDevice takes its settings back.
	SameName bool `json:"sameName"`
}

// Devices lists this device first, then the other devices the library holds
// settings of, most recently changed first.
func (s *SyncService) Devices() []DeviceView {
	st := s.c.Settings.Get()
	name := strings.TrimSpace(st.DeviceName)
	this := DeviceView{ID: st.DeviceID, Name: st.DeviceName, This: true}
	var others []DeviceView
	for _, d := range s.c.Library.Devices() {
		if d.ID == st.DeviceID {
			this.LastSeen, this.Games, this.Emulators, this.Platform = d.LastSeen, d.Games, d.Emulators, d.Platform
			continue
		}
		others = append(others, DeviceView{
			ID: d.ID, Name: d.Name, Platform: d.Platform, LastSeen: d.LastSeen, Games: d.Games, Emulators: d.Emulators,
			SameName: name != "" && strings.EqualFold(strings.TrimSpace(d.Name), name),
		})
	}
	sort.SliceStable(others, func(i, j int) bool { return others[i].SameName && !others[j].SameName })
	return append([]DeviceView{this}, others...)
}

// ForgetDevice removes another device's settings from the library on every
// device; its cloud saves and backups stay. For a device that is gone (sold,
// reset, or an earlier install of the app).
func (s *SyncService) ForgetDevice(id string) error {
	if err := s.c.ForgetDevice(id); err != nil {
		return err
	}
	s.bus.libraryChanged("")
	publishLibrary(s.c, s.bus, nil)
	return nil
}

// ReuseDevice makes this device take over the game and emulator settings of
// device id (the newer kept where both have some), and id is forgotten. For a
// device whose earlier install the library still lists (see
// DeviceView.SameName). Refused while a sync or play session runs.
func (s *SyncService) ReuseDevice(id string) error {
	if s.Active() {
		return ErrSyncRunning
	}
	if err := s.c.ReuseDevice(id); err != nil {
		return err
	}
	s.bus.libraryChanged("")
	publishLibrary(s.c, s.bus, nil)
	return nil
}
