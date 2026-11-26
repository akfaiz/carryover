package core

import (
	"errors"
	"log"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/akfaiz/carryover/internal/cloudsync/syncer"
	"github.com/akfaiz/carryover/internal/library"
)

// ErrThisDevice is returned when a device operation names this device itself.
var ErrThisDevice = errors.New("that is this device")

// ErrSessionsOpen is returned by ReuseDevice while a play session hasn't
// finished: the settings it plays with could change under it.
var ErrSessionsOpen = errors.New("a game is being played, or its saves are not uploaded yet; try again once that is done")

// RegisterDevice writes this device's record in the library (its name and
// platform), when it doesn't say so already; it reports whether it changed.
// It runs with every library sync, and after the name changes.
func (c *Core) RegisterDevice() bool {
	s := c.Settings.Get()
	changed, err := c.Library.RegisterDevice(s.DeviceID, s.DeviceName, runtime.GOOS, time.Now().UTC())
	if err != nil {
		log.Printf("register device: %v", err)
	}
	return changed
}

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

// ReuseDevice makes this device take over the settings of device id, which
// the library knows from before (typically this device before the app was
// reinstalled or its data cleared, which gave it a new id): they move to this
// device's id (the newer kept where both have some), and id is forgotten.
// This device keeps its own id, which on Android is derived from the system's
// identifier of the device, so the next clean install finds them again.
// Refused while a play session hasn't finished.
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
	return c.Library.MoveDevice(id, own, time.Now().UTC())
}

// InUse reports whether a game is being played or synced on this device, by
// this or another Carryover process (a "run" wrapper holds its game's local
// lock), or a session started with StartSession is open: the program must not
// be replaced then.
func (c *Core) InUse() bool {
	entries, err := os.ReadDir(c.Dirs.StateDir())
	if err != nil {
		return false
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if key, ok := strings.CutSuffix(e.Name(), sessionExt); ok && c.SessionOpen(key) {
			return true
		}
		if key, ok := strings.CutSuffix(e.Name(), localLockExt); ok {
			// Taking the lock is the test: a dead process's lock is taken over.
			lock, err := syncer.AcquireLocalLock(c.Dirs.StateDir(), key)
			if err != nil {
				return true
			}
			lock.Release()
		}
	}
	return false
}

// localLockExt ends the file name of a game's local lock (syncer.AcquireLocalLock).
const localLockExt = ".lock"

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
