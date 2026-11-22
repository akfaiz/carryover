package mobile

import (
	"github.com/akfaiz/carryover/services"
)

// TrashView is what was deleted and can still be restored.
type TrashView struct {
	// Days is how long a deleted game or emulator stays restorable.
	Days      int                      `json:"days"`
	Games     []services.TrashGame     `json:"games"`
	Emulators []services.TrashEmulator `json:"emulators"`
}

// Trash lists the deleted games and emulators that can still be restored
// (TrashView) as JSON, newest first. It first removes what has been in the
// trash too long.
func Trash() (string, error) {
	s, err := get()
	if err != nil {
		return "", err
	}
	return encode(TrashView{Days: services.TrashDays, Games: s.library.Trash(), Emulators: s.emus.Trash()}, nil)
}

// DeleteGame moves a game to the trash on all devices. Its cloud saves stay.
func DeleteGame(id string) error {
	s, err := get()
	if err != nil {
		return err
	}
	return s.library.Delete(id)
}

// RestoreGame takes a game out of the trash, with its settings as they were.
func RestoreGame(id string) error {
	s, err := get()
	if err != nil {
		return err
	}
	return s.library.Restore(id)
}

// DeleteGameForever deletes a game, live or in the trash, for good. With
// withSaves everything it has in the cloud goes too (saves, backups, lock);
// without, its cloud saves stay and show up in Leftovers.
func DeleteGameForever(id string, withSaves bool) error {
	s, err := get()
	if err != nil {
		return err
	}
	if withSaves {
		return s.library.DeleteWithSaves(id)
	}
	return s.library.DeleteForever(id)
}

// DeleteEmulator moves an emulator to the trash on all devices. Games keep
// their profile (it just can't be launched) and the cloud saves are kept.
func DeleteEmulator(id string) error {
	s, err := get()
	if err != nil {
		return err
	}
	return s.emus.Delete(id)
}

// RestoreEmulator takes an emulator out of the trash.
func RestoreEmulator(id string) error {
	s, err := get()
	if err != nil {
		return err
	}
	return s.emus.Restore(id)
}

// DeleteEmulatorForever deletes an emulator, live or in the trash, for good:
// it is taken out of every game that uses it. With withSaves the cloud saves
// and backups made with it go too.
func DeleteEmulatorForever(id string, withSaves bool) error {
	s, err := get()
	if err != nil {
		return err
	}
	if withSaves {
		return s.emus.DeleteWithSaves(id)
	}
	return s.emus.DeleteForever(id)
}

// Leftovers lists the cloud save folders no game in the library uses
// ([]services.LeftoverView) as JSON. It needs the cloud.
func Leftovers() (string, error) {
	s, err := get()
	if err != nil {
		return "", err
	}
	return encode(s.library.Leftovers())
}

// RestoreLeftover adds a leftover cloud folder back to the library and returns
// the game's id. The game still needs its save locations on this device.
func RestoreLeftover(key string) (string, error) {
	s, err := get()
	if err != nil {
		return "", err
	}
	return s.library.RestoreLeftover(key)
}

// DeleteLeftover permanently deletes a leftover cloud folder.
func DeleteLeftover(key string) error {
	s, err := get()
	if err != nil {
		return err
	}
	return s.library.DeleteLeftover(key)
}
