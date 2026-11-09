package core

import (
	"errors"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/akfaiz/playrelay/internal/library"
	"github.com/akfaiz/playrelay/internal/steam"
)

func quote(p string) string { return `"` + p + `"` }

// SteamWrapLaunchOptions returns the launch options to paste into a Steam game's
// properties so PlayRelay wraps it.
func SteamWrapLaunchOptions(gameID string) (string, error) {
	self, err := SelfExecutable()
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s run %s -- %%command%%", quote(self), gameID), nil
}

func shortcutMatcher(gameID string) func(steam.Shortcut) bool {
	return func(s steam.Shortcut) bool {
		return strings.Contains(s.LaunchOptions, " "+gameID) || strings.HasPrefix(s.LaunchOptions, "run "+gameID)
	}
}

// AddToSteamResult describes what was written.
type AddToSteamResult struct {
	AppID        string `json:"appId"`
	SteamRunning bool   `json:"steamRunning"`
}

// AddToSteam creates (or updates) a non-Steam shortcut for the game.
// mode LaunchDirect: Steam runs PlayRelay, which launches the configured exe.
// mode LaunchSteamWrap: Steam runs the game exe itself (so Proton can be chosen
// in Steam) with launch options that route it through PlayRelay.
func (c *Core) AddToSteam(gameID, userID string, mode library.LaunchMode) (AddToSteamResult, error) {
	var res AddToSteamResult
	root := c.SteamRoot()
	if root == "" {
		return res, errors.New("no Steam installation found on this device")
	}
	game, err := c.Library.Get(gameID)
	if err != nil {
		return res, err
	}
	dev, _ := game.Device(c.DeviceID())
	self, err := SelfExecutable()
	if err != nil {
		return res, err
	}
	sc := steam.Shortcut{AppName: game.Name, Tags: []string{"PlayRelay"}}
	switch mode {
	case library.LaunchSteamWrap:
		if dev.Exe == "" {
			return res, errors.New("set the game executable for this device first")
		}
		exe, err := c.PathContext(game).Expand(dev.Exe)
		if err != nil {
			return res, err
		}
		wd := filepath.Dir(exe)
		if dev.WorkDir != "" {
			if wd, err = c.PathContext(game).Expand(dev.WorkDir); err != nil {
				return res, err
			}
		}
		sc.Exe, sc.StartDir = quote(exe), quote(wd)
		sc.LaunchOptions = fmt.Sprintf("%s run %s", quote(self), game.ID)
		if dev.Args != "" {
			sc.LaunchOptions += " -- %command% " + dev.Args
		} else {
			sc.LaunchOptions += " -- %command%"
		}
	default:
		mode = library.LaunchDirect
		sc.Exe, sc.StartDir = quote(self), quote(filepath.Dir(self))
		sc.LaunchOptions = "run " + game.ID
	}
	appID, err := steam.UpsertShortcut(root, userID, sc, shortcutMatcher(game.ID))
	if err != nil {
		return res, err
	}
	if game.Cover != "" {
		_ = steam.SetGridImage(root, userID, appID, filepath.Join(c.Dirs.CoversDir(), game.Cover))
	}
	res.AppID = strconv.FormatUint(uint64(appID), 10)
	if _, err := c.Library.UpdateDevice(game.ID, c.DeviceID(), func(d *library.DeviceConfig) {
		d.SteamAppID = res.AppID
		d.LaunchMode = mode
	}); err != nil {
		return res, err
	}
	res.SteamRunning = steam.IsRunning()
	return res, nil
}

// RemoveFromSteam deletes the game's shortcut(s) for a Steam user.
func (c *Core) RemoveFromSteam(gameID, userID string) (int, error) {
	root := c.SteamRoot()
	if root == "" {
		return 0, errors.New("no Steam installation found on this device")
	}
	n, err := steam.RemoveShortcuts(root, userID, shortcutMatcher(gameID))
	if err != nil {
		return 0, err
	}
	_, _ = c.Library.UpdateDevice(gameID, c.DeviceID(), func(d *library.DeviceConfig) { d.SteamAppID = "" })
	return n, nil
}
