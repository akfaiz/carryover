package mobile

import (
	"encoding/json"
	"maps"
	"strings"

	"github.com/akfaiz/carryover/internal/app/msg"
	"github.com/akfaiz/carryover/internal/core"
	"github.com/akfaiz/carryover/internal/library"
	"github.com/akfaiz/carryover/services"
)

// Games lists the library ([]services.GameView) as JSON.
func Games() (_ string, err error) {
	defer wire(&err)
	s, err := get()
	if err != nil {
		return "", err
	}
	return encode(s.library.List(), nil)
}

// Game returns one game (services.GameView) as JSON.
func Game(id string) (_ string, err error) {
	defer wire(&err)
	s, err := get()
	if err != nil {
		return "", err
	}
	return encode(s.library.Get(id))
}

// SaveGame creates (empty id) or updates a game from a services.GameInput
// given as JSON, and returns the saved services.GameView.
func SaveGame(inputJSON string) (_ string, err error) {
	defer wire(&err)
	s, err := get()
	if err != nil {
		return "", err
	}
	var in services.GameInput
	if err := json.Unmarshal([]byte(inputJSON), &in); err != nil {
		return "", err
	}
	return encode(s.library.Save(in))
}

// SetPaths sets this device's save paths of one profile of a game: key is the
// game id (its own saves) or a sync key (an emulator profile), pathsJSON an
// object of slot key -> path (tokens allowed; "" clears it). Everything else
// about the game is kept. It returns the saved services.GameView.
func SetPaths(key, pathsJSON string) (_ string, err error) {
	defer wire(&err)
	s, err := get()
	if err != nil {
		return "", err
	}
	var paths map[string]string
	if err := json.Unmarshal([]byte(pathsJSON), &paths); err != nil {
		return "", err
	}
	gameID, emuID := core.ParseKey(key)
	v, err := s.library.Get(gameID)
	if err != nil {
		return "", err
	}
	in := inputOf(v)
	target := in.Device.Paths
	if emuID != "" {
		target = nil
		for i := range in.Emulators {
			if in.Emulators[i].EmulatorID == emuID {
				target = in.Emulators[i].Paths
			}
		}
		if target == nil {
			return "", msg.EditNoEmulator.Err("Emulator", emuID)
		}
	}
	for k, p := range paths {
		if p = strings.TrimSpace(p); p == "" {
			delete(target, k)
		} else {
			target[k] = p
		}
	}
	return encode(s.library.Save(in))
}

// SetRom sets the game file an emulator profile starts with on this device
// (a path or a content:// URI; "" = open the emulator without a game). key is
// the profile's sync key. It returns the saved services.GameView.
func SetRom(key, rom string) (_ string, err error) {
	defer wire(&err)
	s, err := get()
	if err != nil {
		return "", err
	}
	gameID, emuID := core.ParseKey(key)
	if emuID == "" {
		return "", msg.EditOnlyEmulatorFile.Err()
	}
	v, err := s.library.Get(gameID)
	if err != nil {
		return "", err
	}
	in := inputOf(v)
	found := false
	for i := range in.Emulators {
		if in.Emulators[i].EmulatorID == emuID {
			in.Emulators[i].Rom, found = strings.TrimSpace(rom), true
		}
	}
	if !found {
		return "", msg.EditNoEmulator.Err("Emulator", emuID)
	}
	return encode(s.library.Save(in))
}

// SetNativeApp sets the Android app a game's own profile starts on this
// device (its package name; "" = none). It returns the saved
// services.GameView.
func SetNativeApp(gameID, pkg string) (_ string, err error) {
	defer wire(&err)
	s, err := get()
	if err != nil {
		return "", err
	}
	v, err := s.library.Get(gameID)
	if err != nil {
		return "", err
	}
	in := inputOf(v)
	in.Device.Exe = strings.TrimSpace(pkg)
	in.Device.LaunchMode = library.LaunchDirect
	return encode(s.library.Save(in))
}

// SaveEmulator creates or updates an emulator and this device's setup of it
// from a services.EmulatorSetup given as JSON, and returns the saved
// services.EmulatorView. On Android, Exe is the emulator's package (or
// "package/activity") and Args an optional template that starts a game
// directly, in `am start` syntax with {rom}.
func SaveEmulator(setupJSON string) (_ string, err error) {
	defer wire(&err)
	s, err := get()
	if err != nil {
		return "", err
	}
	var in services.EmulatorSetup
	if err := json.Unmarshal([]byte(setupJSON), &in); err != nil {
		return "", err
	}
	return encode(s.emus.Save(in))
}

// SetCover makes the image file at path (PNG, JPEG or WebP) the game's cover;
// it is copied into Carryover's data and goes to the other devices with the
// next library sync. It returns the saved services.GameView.
func SetCover(gameID, path string) (_ string, err error) {
	defer wire(&err)
	s, err := get()
	if err != nil {
		return "", err
	}
	return encode(s.library.SetCover(gameID, path))
}

// UnsetEmulator removes this device's setup of an emulator (its app, start
// template and data folder), for an emulator no longer used here. The
// emulator, its setup on other devices, the games using it and their cloud
// saves stay.
func UnsetEmulator(id string) (err error) {
	defer wire(&err)
	s, err := get()
	if err != nil {
		return err
	}
	return s.emus.Unset(id)
}

// inputOf is the editor input that saves v unchanged.
func inputOf(v services.GameView) services.GameInput {
	d := v.Device
	in := services.GameInput{
		ID: v.ID, Name: v.Name, StoreAppID: v.StoreAppID, Slots: v.Slots,
		Device: services.DeviceInput{
			Paths: maps.Clone(d.Paths), Exe: d.Exe, Args: d.Args, WorkDir: d.WorkDir,
			Env: d.Env, WaitProcess: d.WaitProcess, LaunchMode: d.LaunchMode,
		},
	}
	if in.Device.Paths == nil {
		in.Device.Paths = map[string]string{}
	}
	for _, p := range v.Emulators {
		paths := maps.Clone(p.Paths)
		if paths == nil {
			paths = map[string]string{}
		}
		in.Emulators = append(in.Emulators, services.GameEmulatorInput{
			EmulatorID: p.EmulatorID, Slots: p.Slots, Rom: p.Rom, Paths: paths, ShareWith: p.ShareWith,
		})
	}
	return in
}

// Emulators lists the emulators ([]services.EmulatorView) as JSON.
func Emulators() (_ string, err error) {
	defer wire(&err)
	s, err := get()
	if err != nil {
		return "", err
	}
	return encode(s.emus.List(), nil)
}

// PreviewPath expands the path tokens of raw for a game on this device.
func PreviewPath(gameID, raw string) (_ string, err error) {
	defer wire(&err)
	s, err := get()
	if err != nil {
		return "", err
	}
	return s.library.PreviewPath(gameID, raw)
}

// ContractPath replaces the start of an absolute path with a path token
// where one applies.
func ContractPath(path string) (_ string, err error) {
	defer wire(&err)
	s, err := get()
	if err != nil {
		return "", err
	}
	return s.library.ContractPath(path), nil
}

// CheckFolder fails unless path is an existing folder.
func CheckFolder(path string) (err error) {
	defer wire(&err)
	s, err := get()
	if err != nil {
		return err
	}
	return s.library.CheckFolder(path)
}

// CheckLocalChanges looks for saves changed since their last sync and
// reports the games it flags through a "library" event.
func CheckLocalChanges() (err error) {
	defer wire(&err)
	s, err := get()
	if err != nil {
		return err
	}
	s.library.CheckLocalChanges()
	return nil
}
