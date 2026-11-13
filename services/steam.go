package services

import (
	"fmt"
	"strconv"

	"github.com/akfaiz/carryover/internal/core"
	"github.com/akfaiz/carryover/internal/library"
	"github.com/akfaiz/carryover/internal/stores/steam"
)

// SteamService integrates with the Steam client.
type SteamService struct {
	c   *core.Core
	bus *Bus
}

// SteamRunURL is the steam:// link that starts g in Steam on this device: its
// non-Steam shortcut if Carryover added one, else its Steam app. It is "" when
// Steam can't start it.
func SteamRunURL(g GameView) string {
	if id, err := strconv.ParseUint(g.Device.SteamAppID, 10, 32); err == nil && id != 0 {
		// A shortcut's game id is its app id in the high half with the
		// "shortcut" type bit set.
		return fmt.Sprintf("steam://rungameid/%d", id<<32|0x02000000)
	}
	if _, err := strconv.ParseUint(g.StoreAppID, 10, 32); err == nil {
		return "steam://rungameid/" + g.StoreAppID
	}
	return ""
}

// NewSteamService creates the service. bus may be nil.
func NewSteamService(c *core.Core, bus *Bus) *SteamService { return &SteamService{c: c, bus: bus} }

// SteamInfo describes the local Steam installation.
type SteamInfo struct {
	Found   bool         `json:"found"`
	Root    string       `json:"root"`
	Users   []steam.User `json:"users"`
	Running bool         `json:"running"`
	OS      string       `json:"os"`
}

// Info returns Steam installation details.
func (s *SteamService) Info() SteamInfo {
	info := SteamInfo{Root: s.c.SteamRoot(), Users: []steam.User{}, OS: goos()}
	if info.Root == "" {
		return info
	}
	info.Found = true
	if u, err := steam.Users(info.Root); err == nil && u != nil {
		info.Users = u
	}
	info.Running = steam.IsRunning()
	return info
}

// AddToSteam adds/updates a non-Steam shortcut. key is a game id (the game's
// own launch config) or a sync key (core.SyncKey) for an emulator profile.
// mode: "direct" or "steamwrap" (emulator profiles are always direct).
func (s *SteamService) AddToSteam(key, userID, mode string) (core.AddToSteamResult, error) {
	gameID, emuID := core.ParseKey(key)
	res, err := s.c.AddToSteamFor(gameID, emuID, userID, library.LaunchMode(mode))
	if err == nil {
		s.bus.libraryChanged(gameID)
	}
	return res, err
}

// RemoveFromSteam removes the shortcut of a game or, with a sync key, of one
// of its emulator profiles.
func (s *SteamService) RemoveFromSteam(key, userID string) (int, error) {
	gameID, emuID := core.ParseKey(key)
	n, err := s.c.RemoveFromSteamFor(gameID, emuID, userID)
	if err == nil {
		s.bus.libraryChanged(gameID)
	}
	return n, err
}

// WrapLaunchOptions returns launch options for wrapping a real Steam game.
func (s *SteamService) WrapLaunchOptions(gameID string) (string, error) {
	return core.SteamWrapLaunchOptions(gameID)
}
