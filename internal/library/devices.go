package library

import (
	"errors"
	"sort"
	"time"
)

// Device is a device's own record in the catalog (Catalog.Devices): what is
// true of the device rather than of a game. Each device writes its own record
// (Store.RegisterDevice); records merge by UpdatedAt, except ForgottenAt,
// which keeps the later of both.
type Device struct {
	Name string `json:"name"`
	// Platform is the GOOS its Carryover runs on: windows, linux, darwin or
	// android ("" if unknown).
	Platform  string    `json:"platform,omitempty"`
	UpdatedAt time.Time `json:"updatedAt"`
	// ForgottenAt is when the device was removed from the library
	// (Store.ForgetDevice): its settings not newer than that are dropped from
	// every game and emulator, in every copy merged with this one. The record
	// stays, so a copy that still has them can't bring them back.
	ForgottenAt time.Time `json:"forgottenAt,omitzero"`
}

// forgotten reports whether the device was forgotten and hasn't been back
// since (a device that registers again after it was forgotten is in use).
func (d Device) forgotten() bool {
	return !d.ForgottenAt.IsZero() && !d.UpdatedAt.After(d.ForgottenAt)
}

func deviceEqual(a, b Device) bool {
	return a.Name == b.Name && a.Platform == b.Platform && a.UpdatedAt.Equal(b.UpdatedAt) && a.ForgottenAt.Equal(b.ForgottenAt)
}

// DeviceInfo is a device the library knows: one with settings in a game or an
// emulator, or one that registered itself.
type DeviceInfo struct {
	ID string `json:"id"`
	// Name is the device's registered name, else the name its newest game
	// settings carry (catalogs written before devices registered); "" when
	// neither is known.
	Name     string `json:"name"`
	Platform string `json:"platform,omitempty"`
	// LastSeen is the newest change to its settings or its record.
	LastSeen time.Time `json:"lastSeen"`
	// Games and Emulators count the games (not those in the trash) and the
	// emulators set up on it.
	Games     int `json:"games"`
	Emulators int `json:"emulators"`
}

// ErrUnknownDevice is returned for a device the library doesn't know.
var ErrUnknownDevice = errors.New("the library has no settings of that device")

// RegisterDevice writes this device's record: its name and platform. It does
// nothing when the record already says so (and the device isn't forgotten),
// so calling it on every library sync costs nothing. It reports whether the
// record changed.
func (s *Store) RegisterDevice(id, name, platform string, now time.Time) (bool, error) {
	if id == "" {
		return false, ErrUnknownDevice
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	cur, ok := s.devices[id]
	if ok && cur.Name == name && cur.Platform == platform && !cur.forgotten() {
		return false, nil
	}
	cur.Name, cur.Platform, cur.UpdatedAt = name, platform, now
	s.devices[id] = cur
	return true, s.persist()
}

// Devices lists the devices the library knows, most recently seen first;
// forgotten ones are left out.
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
	seen := func(d *DeviceInfo, t time.Time) {
		if t.After(d.LastSeen) {
			d.LastSeen = t
		}
	}
	named := map[string]time.Time{} // when the name taken from a game entry was written
	for _, g := range s.games {
		for id, dc := range g.Devices {
			d := info(id)
			seen(d, dc.UpdatedAt)
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
			seen(d, ed.UpdatedAt)
			if !e.Deleted {
				d.Emulators++
			}
		}
	}
	for id, rec := range s.devices {
		d, has := byID[id]
		if !has {
			if rec.forgotten() {
				continue
			}
			d = info(id)
		}
		seen(d, rec.UpdatedAt)
		if rec.Name != "" {
			d.Name = rec.Name
		}
		d.Platform = rec.Platform
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

// DeviceName returns a device's registered name, or fallback when it has
// none (a catalog written before devices registered).
func (s *Store) DeviceName(id, fallback string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if d, ok := s.devices[id]; ok && d.Name != "" {
		return d.Name
	}
	return fallback
}

// HasDevice reports whether the library knows a device: it has settings in a
// game or an emulator, or a record that isn't forgotten.
func (s *Store) HasDevice(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if d, ok := s.devices[id]; ok && !d.forgotten() {
		return true
	}
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
// all devices: its record is marked forgotten at now, which drops the
// settings it had then from every copy merged with this one. Settings the
// device writes after now are kept, so a device forgotten by mistake comes
// back as soon as it changes something or syncs the library.
func (s *Store) ForgetDevice(id string, now time.Time) error {
	if id == "" {
		return ErrUnknownDevice
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.forgetLocked(id, now)
	s.pruneForgottenLocked()
	return s.persist()
}

func (s *Store) forgetLocked(id string, now time.Time) {
	d := s.devices[id]
	if now.After(d.ForgottenAt) {
		d.ForgottenAt = now
	}
	s.devices[id] = d
}

// MoveDevice gives device to the settings of device from, for a device that
// takes over the settings of an earlier install of itself (a reinstall made
// it a new device): where only from has settings they move to to; where both
// have, the newer one is kept (from's on a tie). from is then forgotten (see
// ForgetDevice). The moved settings are stamped now, so they win over the
// copies of to elsewhere.
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
	s.forgetLocked(from, now)
	s.pruneForgottenLocked()
	return s.persist()
}

// mergeDevicesLocked merges device records: the newer record wins, and
// ForgottenAt keeps the later of both. It reports whether anything changed.
func (s *Store) mergeDevicesLocked(in map[string]Device) bool {
	changed := false
	for id, d := range in {
		cur, ok := s.devices[id]
		m := cur
		if !ok || d.UpdatedAt.After(cur.UpdatedAt) {
			m = d
		}
		if cur.ForgottenAt.After(m.ForgottenAt) {
			m.ForgottenAt = cur.ForgottenAt
		}
		if d.ForgottenAt.After(m.ForgottenAt) {
			m.ForgottenAt = d.ForgottenAt
		}
		if !ok || !deviceEqual(m, cur) {
			s.devices[id] = m
			changed = true
		}
	}
	return changed
}

// anyForgottenLocked reports whether a device was ever forgotten: such a
// catalog needs version 3, so an older client can't drop the records and
// bring the forgotten settings back.
func (s *Store) anyForgottenLocked() bool {
	for _, d := range s.devices {
		if !d.ForgottenAt.IsZero() {
			return true
		}
	}
	return false
}

// pruneForgottenLocked drops the settings of forgotten devices that are not
// newer than when the device was forgotten. It reports whether anything was
// dropped.
func (s *Store) pruneForgottenLocked() bool {
	forgotten := map[string]time.Time{}
	for id, d := range s.devices {
		if !d.ForgottenAt.IsZero() {
			forgotten[id] = d.ForgottenAt
		}
	}
	if len(forgotten) == 0 {
		return false
	}
	changed := false
	for id, g := range s.games {
		if pruneDevices(g.Devices, forgotten, func(d DeviceConfig) time.Time { return d.UpdatedAt }) {
			s.games[id] = g
			changed = true
		}
	}
	for id, e := range s.emulators {
		if pruneDevices(e.Devices, forgotten, func(d EmulatorDevice) time.Time { return d.UpdatedAt }) {
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
