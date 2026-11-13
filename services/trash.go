package services

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/akfaiz/carryover/internal/library"
)

// TrashDays is how many days a deleted game or emulator can be restored.
const TrashDays = int(library.TrashRetention / (24 * time.Hour))

// TrashGame is a deleted game that can still be restored.
type TrashGame struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	DeletedAt time.Time `json:"deletedAt"`
	// ExpiresAt is when its settings are removed for good.
	ExpiresAt time.Time `json:"expiresAt"`
	Emulators int       `json:"emulators"`
}

// TrashEmulator is a deleted emulator that can still be restored.
type TrashEmulator struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	DeletedAt time.Time `json:"deletedAt"`
	ExpiresAt time.Time `json:"expiresAt"`
}

// LeftoverView is a cloud save folder no game uses (see core.Leftover).
type LeftoverView struct {
	Key          string    `json:"key"`
	GameID       string    `json:"gameId"`
	GameName     string    `json:"gameName"`
	EmulatorID   string    `json:"emulatorId"`
	EmulatorName string    `json:"emulatorName"`
	Slots        []string  `json:"slots"`
	Files        int       `json:"files"`
	Bytes        int64     `json:"bytes"`
	Time         time.Time `json:"time"`
	Backups      int       `json:"backups"`
}

// Trash lists the deleted games that can still be restored, newest first. It
// first removes what has been in the trash too long.
func (s *LibraryService) Trash() []TrashGame {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	s.c.ExpireTrash(ctx)
	games := s.c.Library.Trash()
	out := make([]TrashGame, 0, len(games))
	for _, g := range games {
		out = append(out, TrashGame{
			ID: g.ID, Name: g.Name, DeletedAt: g.DeletedTime(), ExpiresAt: g.DeletedTime().Add(library.TrashRetention), Emulators: len(g.Emulators),
		})
	}
	return out
}

// Restore takes a game out of the trash, with its settings as they were.
func (s *LibraryService) Restore(id string) error {
	if _, err := s.c.Library.Restore(id); err != nil {
		return err
	}
	s.bus.libraryChanged(id)
	s.publish()
	return nil
}

// TrashCount is how many deleted games and emulators wait in the trash.
func (s *LibraryService) TrashCount() int {
	return len(s.c.Library.Trash()) + len(s.c.Library.TrashedEmulators())
}

// LeftoverCount is how many leftover cloud save folders there are (one cloud call).
func (s *LibraryService) LeftoverCount() (int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	return s.c.CountLeftovers(ctx)
}

// Leftovers lists the cloud save folders no game in the library uses: games
// deleted for good and emulators removed from their game. It needs the cloud.
func (s *LibraryService) Leftovers() ([]LeftoverView, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	ls, err := s.c.Leftovers(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]LeftoverView, 0, len(ls))
	for _, l := range ls {
		out = append(out, LeftoverView{
			Key: l.Key, GameID: l.GameID, GameName: l.GameName, EmulatorID: l.EmulatorID, EmulatorName: l.EmulatorName,
			Slots: l.Slots, Files: l.Files, Bytes: l.Bytes, Time: l.Time, Backups: l.Backups,
		})
	}
	return out, nil
}

// RestoreLeftover adds a leftover cloud folder back to the library and returns
// the game's id. The game still needs its save locations on this device, then
// the first sync downloads the saves.
func (s *LibraryService) RestoreLeftover(key string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	id, err := s.c.RestoreLeftover(ctx, key)
	if err != nil {
		return "", err
	}
	s.bus.libraryChanged(id)
	s.publish()
	return id, nil
}

// DeleteLeftover permanently deletes a leftover cloud folder.
func (s *LibraryService) DeleteLeftover(key string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	return s.c.DeleteLeftover(ctx, key)
}

// Trash lists the deleted emulators that can still be restored, newest first.
func (s *EmulatorService) Trash() []TrashEmulator {
	ems := s.c.Library.TrashedEmulators()
	out := make([]TrashEmulator, 0, len(ems))
	for _, e := range ems {
		out = append(out, TrashEmulator{ID: e.ID, Name: e.Name, DeletedAt: e.DeletedTime(), ExpiresAt: e.DeletedTime().Add(library.TrashRetention)})
	}
	return out
}

// Restore takes an emulator out of the trash. The games that still list it can
// launch with it again.
func (s *EmulatorService) Restore(id string) error {
	e, err := s.c.Library.EmulatorAny(id)
	if err != nil {
		return err
	}
	for _, o := range s.c.Library.Emulators() {
		if o.ID != id && strings.EqualFold(o.Name, e.Name) {
			return errors.New("an emulator with this name already exists: rename it first")
		}
	}
	if _, err := s.c.Library.RestoreEmulator(id); err != nil {
		return err
	}
	s.bus.libraryChanged("")
	s.publish()
	return nil
}
