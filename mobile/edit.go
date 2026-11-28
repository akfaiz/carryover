package mobile

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/akfaiz/carryover/internal/app/msg"
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
func AddEmulatorGame(name, emulatorID, rom, savePath string) (_ string, err error) {
	defer wire(&err)
	s, err := get()
	if err != nil {
		return "", err
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return "", msg.EditNeedName.Err()
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
func RenameGame(gameID, name string) (_ string, err error) {
	defer wire(&err)
	s, err := get()
	if err != nil {
		return "", err
	}
	if name = strings.TrimSpace(name); name == "" {
		return "", msg.EditNeedName.Err()
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
func AddGameEmulator(gameID, emulatorID, shareWith string) (_ string, err error) {
	defer wire(&err)
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
		return "", msg.EditAlreadyPlayed.Err("Game", v.Name, "Emulator", emu.Name)
	}
	add := services.GameEmulatorInput{EmulatorID: emulatorID, Paths: map[string]string{}}
	if shareWith != "" {
		i := slices.IndexFunc(in.Emulators, func(e services.GameEmulatorInput) bool { return e.EmulatorID == shareWith })
		if i < 0 {
			return "", msg.EditNoEmulator.Err("Emulator", shareWith)
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
func RemoveGameEmulator(gameID, emulatorID string) (_ string, err error) {
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
	i := slices.IndexFunc(in.Emulators, func(e services.GameEmulatorInput) bool { return e.EmulatorID == emulatorID })
	if i < 0 {
		return "", msg.EditNoEmulator.Err("Emulator", emulatorID)
	}
	for _, e := range in.Emulators {
		if e.ShareWith == emulatorID {
			return "", msg.EditUsesSaves.Err("Emulator", s.core.ProfileName(e.EmulatorID))
		}
	}
	if len(in.Slots) == 0 && len(in.Emulators) == 1 {
		return "", msg.EditOnlyEmulator.Err()
	}
	if g, err := s.core.Library.Get(gameID); err == nil && s.core.SessionOpen(core.CloudKey(g, emulatorID)) {
		return "", core.ErrPlaying
	}
	in.Emulators = slices.Delete(in.Emulators, i, i+1)
	return encode(s.library.Save(in))
}

// SetSlots sets the save locations (slots) of one profile of a game, for every
// device: key is the game id (its own saves) or a sync key (an emulator
// profile), slotsJSON the new list of library.SaveSlot. A slot with a key must
// already exist (its key names its cloud folder, so only its label and filters
// change); one with an empty key is new, and its key is made from its label.
// A slot left out is removed from the profile and from this device's paths;
// its cloud files stay. Refused for a profile that uses another emulator's
// saves (change them there), for an empty list, and while the profile is being
// played. It returns the saved services.GameView.
func SetSlots(key, slotsJSON string) (_ string, err error) {
	defer wire(&err)
	s, err := get()
	if err != nil {
		return "", err
	}
	var slots []library.SaveSlot
	if err := json.Unmarshal([]byte(slotsJSON), &slots); err != nil {
		return "", err
	}
	gameID, emuID := core.ParseKey(key)
	v, err := s.library.Get(gameID)
	if err != nil {
		return "", err
	}
	in := inputOf(v)
	cur, paths := in.Slots, in.Device.Paths
	var edited *services.GameEmulatorInput
	if emuID != "" {
		i := slices.IndexFunc(in.Emulators, func(e services.GameEmulatorInput) bool { return e.EmulatorID == emuID })
		if i < 0 {
			return "", msg.EditNoEmulator.Err("Emulator", emuID)
		}
		edited = &in.Emulators[i]
		if edited.ShareWith != "" {
			return "", msg.EditSharedSlots.Err("Emulator", s.core.ProfileName(emuID), "Other", s.core.ProfileName(edited.ShareWith))
		}
		cur, paths = edited.Slots, edited.Paths
	}
	next, err := mergeSlots(cur, slots)
	if err != nil {
		return "", err
	}
	if g, err := s.core.Library.Get(gameID); err == nil && s.core.SessionOpen(core.CloudKey(g, emuID)) {
		return "", core.ErrPlaying
	}
	for _, old := range cur {
		if !slices.ContainsFunc(next, func(sl library.SaveSlot) bool { return sl.Key == old.Key }) {
			delete(paths, old.Key)
		}
	}
	if edited != nil {
		edited.Slots = next
	} else {
		in.Slots = next
	}
	return encode(s.library.Save(in))
}

// mergeSlots is the slot list a profile has after an edit: the entries of want
// in order, an existing one by its key (keeping the key) and a new one (empty
// key) with a key made from its label that no slot has.
func mergeSlots(cur, want []library.SaveSlot) ([]library.SaveSlot, error) {
	if len(want) == 0 {
		return nil, msg.EditNeedSlot.Err()
	}
	out := make([]library.SaveSlot, 0, len(want))
	for _, sl := range want {
		sl.Label = strings.TrimSpace(sl.Label)
		sl.Key = strings.TrimSpace(sl.Key)
		if sl.Key != "" {
			if !slices.ContainsFunc(cur, func(c library.SaveSlot) bool { return c.Key == sl.Key }) {
				return nil, msg.EditUnknownSlot.Err("Slot", sl.Key)
			}
			if slices.ContainsFunc(out, func(o library.SaveSlot) bool { return o.Key == sl.Key }) {
				return nil, msg.EditSlotTwice.Err("Slot", sl.Key)
			}
		} else if sl.Label == "" {
			return nil, msg.EditSlotName.Err()
		}
		out = append(out, sl)
	}
	// New keys avoid every key in use now (a removed slot's cloud folder may still
	// hold files) and every key chosen so far.
	taken := slices.Clone(cur)
	for _, sl := range out {
		taken = append(taken, library.SaveSlot{Key: sl.Key})
	}
	for i := range out {
		if out[i].Key == "" {
			out[i].Key = slotKeyFor(out[i].Label, taken)
			taken = append(taken, library.SaveSlot{Key: out[i].Key})
		}
	}
	return out, nil
}

// slotKeyFor makes a slot key from its label: lower-case letters and digits
// with dashes between words ("Game state" -> "game-state"), numbered when a
// slot already has it. A label with no such characters gives "saves-<n>".
func slotKeyFor(label string, taken []library.SaveSlot) string {
	var b strings.Builder
	for _, r := range strings.ToLower(label) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case b.Len() > 0 && !strings.HasSuffix(b.String(), "-"):
			b.WriteByte('-')
		}
	}
	base := strings.Trim(b.String(), "-")
	if base == "" {
		return uniqueSlotKey(taken, 1)
	}
	key := base
	for n := 2; slices.ContainsFunc(taken, func(sl library.SaveSlot) bool { return sl.Key == key }); n++ {
		key = fmt.Sprintf("%s-%d", base, n)
	}
	return key
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
