package library

import (
	"errors"
	"sort"
	"time"
)

// DeviceInfo is a device the library holds settings of.
type DeviceInfo struct {
	ID string `json:"id"`
	// Name is the device's name as its newest game settings carry it; "" when
	// it has set up only emulators.
	Name string `json:"name"`
	// LastSeen is the newest change to its settings.
	LastSeen time.Time `json:"lastSeen"`
	// Games and Emulators count the games (not those in the trash) and the
	// emulators set up on it.
	Games     int `json:"games"`
	Emulators int `json:"emulators"`
}

// ErrUnknownDevice is returned for a device the library has no settings of.
var ErrUnknownDevice = errors.New("the library has no settings of that device")

// Devices lists the devices that have settings in the library, most recently
// changed first.
func (s *Store) Devices() []DeviceInfo {
	s.mu.Lock()
	defer s.mu.Unlock()
	byID := map[string]*DeviceInfo{}
	info := func(id string) *DeviceInfo {
		d := byID[id]
		if d == nil {
			d = &DeviceInfo{ID: id}
			byID[id] = d
		}
		return d
	}
	named := map[string]time.Time{} // when the name in DeviceInfo was set
	for _, g := range s.games {
		for id, dc := range g.Devices {
			d := info(id)
			if dc.UpdatedAt.After(d.LastSeen) {
				d.LastSeen = dc.UpdatedAt
			}
			if dc.DeviceName != "" && !named[id].After(dc.UpdatedAt) {
				d.Name, named[id] = dc.DeviceName, dc.UpdatedAt
			}
			if !g.Deleted {
				d.Games++
			}
		}
	}
	for _, e := range s.emulators {
		for id, ed := range e.Devices {
			d := info(id)
			if ed.UpdatedAt.After(d.LastSeen) {
				d.LastSeen = ed.UpdatedAt
			}
			if !e.Deleted {
				d.Emulators++
			}
		}
	}
	out := make([]DeviceInfo, 0, len(byID))
	for _, d := range byID {
		out = append(out, *d)
	}
	sort.Slice(out, func(i, j int) bool {
		if !out[i].LastSeen.Equal(out[j].LastSeen) {
			return out[i].LastSeen.After(out[j].LastSeen)
		}
		return out[i].ID < out[j].ID
	})
	return out
}

// HasDevice reports whether the library holds settings of a device.
func (s *Store) HasDevice(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.hasDeviceLocked(id)
}

func (s *Store) hasDeviceLocked(id string) bool {
	for _, g := range s.games {
		if _, ok := g.Devices[id]; ok {
			return true
		}
	}
	for _, e := range s.emulators {
		if _, ok := e.Devices[id]; ok {
			return true
		}
	}
	return false
}

// ForgetDevice removes a device's settings from every game and emulator, on
// all devices: the catalog keeps a marker (ForgottenDevices) that drops the
// settings it had at now from every copy merged with it. Settings the device
// writes after now are kept, so a device forgotten by mistake comes back as
// soon as it changes something.
func (s *Store) ForgetDevice(id string, now time.Time) error {
	if id == "" {
		return ErrUnknownDevice
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.mergeForgottenLocked(map[string]time.Time{id: now})
	s.pruneForgottenLocked()
	return s.persist()
}

// MoveDevice gives device to the settings of device from, for a device that
// takes over the identity it had before (a reinstall made it a new device):
// where only from has settings they move to to; where both have, the newer
// one is kept (from's on a tie). from is then forgotten (see ForgetDevice).
// The moved settings are stamped now, so they win over the copies of to
// elsewhere.
func (s *Store) MoveDevice(from, to string, now time.Time) error {
	if from == "" || to == "" || from == to {
		return ErrUnknownDevice
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for id, g := range s.games {
		d, ok := g.Devices[from]
		if !ok {
			continue
		}
		if cur, has := g.Devices[to]; !has || !cur.UpdatedAt.After(d.UpdatedAt) {
			d.UpdatedAt = now
			g.Devices[to] = d
		}
		delete(g.Devices, from)
		s.games[id] = g
	}
	for id, e := range s.emulators {
		d, ok := e.Devices[from]
		if !ok {
			continue
		}
		if cur, has := e.Devices[to]; !has || !cur.UpdatedAt.After(d.UpdatedAt) {
			d.UpdatedAt = now
			e.Devices[to] = d
		}
		delete(e.Devices, from)
		s.emulators[id] = e
	}
	s.mergeForgottenLocked(map[string]time.Time{from: now})
	s.pruneForgottenLocked()
	return s.persist()
}

// mergeForgottenLocked adds forgotten devices, keeping the later time of a
// device forgotten twice. It reports whether anything changed.
func (s *Store) mergeForgottenLocked(in map[string]time.Time) bool {
	changed := false
	for id, at := range in {
		if cur, ok := s.forgotten[id]; !ok || at.After(cur) {
			s.forgotten[id] = at
			changed = true
		}
	}
	return changed
}

// pruneForgottenLocked drops the settings of forgotten devices that are not
// newer than when the device was forgotten. It reports whether anything was
// dropped.
func (s *Store) pruneForgottenLocked() bool {
	if len(s.forgotten) == 0 {
		return false
	}
	changed := false
	for id, g := range s.games {
		if pruneDevices(g.Devices, s.forgotten, func(d DeviceConfig) time.Time { return d.UpdatedAt }) {
			s.games[id] = g
			changed = true
		}
	}
	for id, e := range s.emulators {
		if pruneDevices(e.Devices, s.forgotten, func(d EmulatorDevice) time.Time { return d.UpdatedAt }) {
			s.emulators[id] = e
			changed = true
		}
	}
	return changed
}

func pruneDevices[T any](devices map[string]T, forgotten map[string]time.Time, updated func(T) time.Time) bool {
	changed := false
	for id, d := range devices {
		if at, ok := forgotten[id]; ok && !updated(d).After(at) {
			delete(devices, id)
			changed = true
		}
	}
	return changed
}
