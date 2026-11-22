package mobile

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/akfaiz/carryover/internal/core"
	"github.com/akfaiz/carryover/internal/library"
	"github.com/akfaiz/carryover/services"
)

// savesSlot is the one save location of a profile the Android app creates;
// its folder is chosen per device.
var savesSlot = library.SaveSlot{Key: "saves", Label: "Saves"}

// AddEmulatorGame adds a game played with an emulator: name, the emulator's
// id, the game file on this device (a path or a content:// URI, "" = none)
// and its save folder here ("" = set later). It returns the new
// services.GameView.
func AddEmulatorGame(name, emulatorID, rom, savePath string) (string, error) {
	s, err := get()
	if err != nil {
		return "", err
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return "", errors.New("the game needs a name")
	}
	if _, err := s.core.Library.Emulator(emulatorID); err != nil {
		return "", err
	}
	paths := map[string]string{}
	if p := strings.TrimSpace(savePath); p != "" {
		paths[savesSlot.Key] = p
	}
	return encode(s.library.Save(services.GameInput{
		Name:   name,
		Device: services.DeviceInput{Paths: map[string]string{}},
		Emulators: []services.GameEmulatorInput{{
			EmulatorID: emulatorID,
			Slots:      []library.SaveSlot{savesSlot},
			Rom:        strings.TrimSpace(rom),
			Paths:      paths,
		}},
	}))
}

// RenameGame renames a game on every device. It returns the saved
// services.GameView.
func RenameGame(gameID, name string) (string, error) {
	s, err := get()
	if err != nil {
		return "", err
	}
	if name = strings.TrimSpace(name); name == "" {
		return "", errors.New("the game needs a name")
	}
	v, err := s.library.Get(gameID)
	if err != nil {
		return "", err
	}
	in := inputOf(v)
	in.Name = name
	return encode(s.library.Save(in))
}

// AddGameEmulator adds an emulator to a game, for every device. shareWith is
// another emulator of the game whose cloud saves this one uses (the same
// saves, such as Lemon next to Eden), or "" for saves of its own, kept in one
// folder chosen per device. It returns the saved services.GameView.
func AddGameEmulator(gameID, emulatorID, shareWith string) (string, error) {
	s, err := get()
	if err != nil {
		return "", err
	}
	emu, err := s.core.Library.Emulator(emulatorID)
	if err != nil {
		return "", err
	}
	v, err := s.library.Get(gameID)
	if err != nil {
		return "", err
	}
	in := inputOf(v)
	if slices.ContainsFunc(in.Emulators, func(e services.GameEmulatorInput) bool { return e.EmulatorID == emulatorID }) {
		return "", fmt.Errorf("%s is already played with %s", v.Name, emu.Name)
	}
	add := services.GameEmulatorInput{EmulatorID: emulatorID, Paths: map[string]string{}}
	if shareWith != "" {
		i := slices.IndexFunc(in.Emulators, func(e services.GameEmulatorInput) bool { return e.EmulatorID == shareWith })
		if i < 0 {
			return "", fmt.Errorf("the game has no emulator %q", shareWith)
		}
		// Saves are shared with the emulator that keeps them, never through another sharer.
		if target := in.Emulators[i].ShareWith; target != "" {
			shareWith = target
		}
		add.ShareWith = shareWith
	} else {
		add.Slots = []library.SaveSlot{savesSlot}
	}
	in.Emulators = append(in.Emulators, add)
	return encode(s.library.Save(in))
}

// RemoveGameEmulator removes an emulator from a game, for every device. Its
// cloud saves stay. It is refused while another emulator of the game uses its
// saves, while it is being played, and for the game's only way of keeping
// saves. It returns the saved services.GameView.
func RemoveGameEmulator(gameID, emulatorID string) (string, error) {
	s, err := get()
	if err != nil {
		return "", err
	}
	v, err := s.library.Get(gameID)
	if err != nil {
		return "", err
	}
	in := inputOf(v)
	i := slices.IndexFunc(in.Emulators, func(e services.GameEmulatorInput) bool { return e.EmulatorID == emulatorID })
	if i < 0 {
		return "", fmt.Errorf("the game has no emulator %q", emulatorID)
	}
	for _, e := range in.Emulators {
		if e.ShareWith == emulatorID {
			return "", fmt.Errorf("%s uses its saves: remove %s first", s.core.ProfileName(e.EmulatorID), s.core.ProfileName(e.EmulatorID))
		}
	}
	if len(in.Slots) == 0 && len(in.Emulators) == 1 {
		return "", errors.New("it is the game's only emulator")
	}
	if g, err := s.core.Library.Get(gameID); err == nil && s.core.SessionOpen(core.CloudKey(g, emulatorID)) {
		return "", core.ErrPlaying
	}
	in.Emulators = slices.Delete(in.Emulators, i, i+1)
	return encode(s.library.Save(in))
}

// GuessRomName suggests a game's name from its game file's name (for a
// Switch file, its title ID can give the real title).
func GuessRomName(emulatorID, fileName string) string {
	s, err := get()
	if err != nil {
		return ""
	}
	return s.emus.GuessRomName(emulatorID, fileName)
}
