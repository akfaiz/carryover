package services

import (
	"github.com/akfaiz/playrelay/internal/core"
	"github.com/akfaiz/playrelay/internal/library"
	"github.com/akfaiz/playrelay/internal/steam"
)

// SteamService integrates with the Steam client.
type SteamService struct {
	c   *core.Core
	bus *Bus
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

// AddToSteam adds/updates the game's non-Steam shortcut. mode: "direct" or "steamwrap".
func (s *SteamService) AddToSteam(gameID, userID, mode string) (core.AddToSteamResult, error) {
	res, err := s.c.AddToSteam(gameID, userID, library.LaunchMode(mode))
	if err == nil {
		s.bus.libraryChanged(gameID)
	}
	return res, err
}

// RemoveFromSteam removes the game's shortcut.
func (s *SteamService) RemoveFromSteam(gameID, userID string) (int, error) {
	n, err := s.c.RemoveFromSteam(gameID, userID)
	if err == nil {
		s.bus.libraryChanged(gameID)
	}
	return n, err
}

// WrapLaunchOptions returns launch options for wrapping a real Steam game.
func (s *SteamService) WrapLaunchOptions(gameID string) (string, error) {
	return core.SteamWrapLaunchOptions(gameID)
}
