package core

import (
	"errors"
	"os"
	"strings"
	"time"

	"github.com/akfaiz/carryover/internal/library"
)

// ErrThisDevice is returned when a device operation names this device itself.
var ErrThisDevice = errors.New("that is this device")

// ErrSessionsOpen is returned by ReuseDevice while a play session hasn't
// finished: its cloud lock is held under this device's current id.
var ErrSessionsOpen = errors.New("a game is being played, or its saves are not uploaded yet; try again once that is done")

// ForgetDevice removes another device's settings from the library, on every
// device (library.Store.ForgetDevice). Its cloud saves and backups stay.
func (c *Core) ForgetDevice(id string) error {
	if id == "" || id == c.DeviceID() {
		return ErrThisDevice
	}
	if !c.Library.HasDevice(id) {
		return library.ErrUnknownDevice
	}
	return c.Library.ForgetDevice(id, time.Now().UTC())
}

// ReuseDevice makes this device take over the identity of device id, which
// the library knows from before (typically this device before the app was
// reinstalled, which gave it a new id): this device's id becomes id, the
// settings it has under its current id move there (the newer kept where both
// have some), and the current id is forgotten. Refused while a play session
// hasn't finished, since its cloud lock carries the current id.
func (c *Core) ReuseDevice(id string) error {
	own := c.DeviceID()
	if id == "" || id == own {
		return ErrThisDevice
	}
	if !c.Library.HasDevice(id) {
		return library.ErrUnknownDevice
	}
	if c.anySession() {
		return ErrSessionsOpen
	}
	if err := c.Settings.SetDeviceID(id); err != nil {
		return err
	}
	if err := c.Library.MoveDevice(own, id, time.Now().UTC()); err != nil {
		_ = c.Settings.SetDeviceID(own) // nothing moved: stay who we were
		return err
	}
	return nil
}

// anySession reports whether a session marker exists: a game is being played
// on this device, or its session hasn't finished uploading.
func (c *Core) anySession() bool {
	entries, err := os.ReadDir(c.Dirs.StateDir())
	if err != nil {
		return false
	}
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), sessionExt) {
			return true
		}
	}
	return false
}
