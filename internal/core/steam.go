package core

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/akfaiz/carryover/internal/library"
	"github.com/akfaiz/carryover/internal/stores/steam"
)

func quote(p string) string { return `"` + p + `"` }

// SteamWrapLaunchOptions returns the launch options to paste into a Steam game's
// properties so Carryover wraps it.
func SteamWrapLaunchOptions(gameID string) (string, error) {
	self, err := SelfExecutable()
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s run %s -- %%command%%", quote(self), gameID), nil
}

// parseRunOptions reads the game id and emulator id ("" = native profile) out
// of the launch options of a shortcut Carryover wrote: `[exe] run <id>
// [--emulator <id>] [-- %command% ...]`.
func parseRunOptions(launchOptions string) (gameID, emuID string, ok bool) {
	f := strings.Fields(launchOptions)
	for i, t := range f {
		if t != "run" || i+1 >= len(f) {
			continue
		}
		gameID = f[i+1]
		for j := i + 2; j < len(f) && f[j] != "--"; j++ {
			if f[j] == "--emulator" && j+1 < len(f) {
				emuID = f[j+1]
				break
			}
		}
		return gameID, emuID, true
	}
	return "", "", false
}

// shortcutMatcher matches the shortcut of one profile of a game, so a game's
// native and emulator shortcuts can be added and removed independently.
func shortcutMatcher(gameID, emuID string) func(steam.Shortcut) bool {
	return func(s steam.Shortcut) bool {
		g, e, ok := parseRunOptions(s.LaunchOptions)
		return ok && g == gameID && e == emuID
	}
}

// artFile returns the stored image of an extra artwork kind, or "" if none.
func (c *Core) artFile(g library.Game, kind string) string {
	name := g.Art[kind]
	if name == "" {
		return ""
	}
	p := filepath.Join(c.Dirs.CoversDir(), name)
	if _, err := os.Stat(p); err != nil {
		return ""
	}
	return p
}

// AddToSteamResult describes what was written.
type AddToSteamResult struct {
	AppID        string `json:"appId"`
	SteamRunning bool   `json:"steamRunning"`
}

// AddToSteam creates (or updates) a non-Steam shortcut for the game.
// mode LaunchDirect: Steam runs Carryover, which launches the configured exe.
// mode LaunchSteamWrap: Steam runs the game exe itself (so Proton can be chosen
// in Steam) with launch options that route it through Carryover.
// mode LaunchHeroic: like LaunchDirect, but Carryover starts the game through
// Heroic, so it keeps Heroic's Wine prefix instead of getting a Proton prefix.
func (c *Core) AddToSteam(gameID, userID string, mode library.LaunchMode) (AddToSteamResult, error) {
	return c.AddToSteamFor(gameID, "", userID, mode)
}

// AddToSteamFor is AddToSteam for one profile. An emulator profile always gets
// a direct shortcut named "<game> (<emulator>)" that runs `run <id> --emulator <id>`.
func (c *Core) AddToSteamFor(gameID, emuID, userID string, mode library.LaunchMode) (AddToSteamResult, error) {
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
	// A Heroic game's direct shortcut must stay a Heroic launch: started
	// directly, it would leave Heroic's prefix.
	if mode == library.LaunchDirect && dev.LaunchMode == library.LaunchHeroic {
		mode = library.LaunchHeroic
	}
	sc := steam.Shortcut{AppName: game.Name, Tags: []string{"Carryover"}}
	switch {
	case emuID != "":
		if _, ok := game.Emulator(emuID); !ok {
			return res, errors.New("the game is not set up with this emulator")
		}
		if _, ok := dev.Emulators[emuID]; !ok {
			return res, errors.New("set the emulator up for this game on this device first")
		}
		emu, err := c.Library.Emulator(emuID)
		if err != nil {
			return res, err
		}
		mode = library.LaunchDirect
		sc.AppName = game.Name + " (" + emu.Name + ")"
		sc.Exe, sc.StartDir = quote(self), quote(filepath.Dir(self))
		sc.LaunchOptions = "run " + game.ID + " --emulator " + emuID
	case mode == library.LaunchSteamWrap:
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
		if mode != library.LaunchHeroic {
			mode = library.LaunchDirect
		} else if dev.Exe == "" {
			return res, errors.New("set the game executable for this device first")
		}
		sc.Exe, sc.StartDir = quote(self), quote(filepath.Dir(self))
		sc.LaunchOptions = "run " + game.ID
	}
	// The icon is referenced from the shortcut itself, so it is written first.
	if icon := c.artFile(game, "icon"); icon != "" {
		id := steam.ShortcutAppID(sc.Exe, sc.AppName)
		if dst, err := steam.SetGridArt(root, userID, id, "icon", icon); err == nil {
			sc.Icon = dst
		}
	}
	appID, err := steam.UpsertShortcut(root, userID, sc, shortcutMatcher(game.ID, emuID))
	if err != nil {
		return res, err
	}
	if game.Cover != "" {
		_ = steam.SetGridImage(root, userID, appID, filepath.Join(c.Dirs.CoversDir(), game.Cover))
	}
	for _, kind := range []string{"wide", "hero", "logo"} {
		if f := c.artFile(game, kind); f != "" {
			_, _ = steam.SetGridArt(root, userID, appID, kind, f)
		}
	}
	res.AppID = strconv.FormatUint(uint64(appID), 10)
	if _, err := c.Library.UpdateDevice(game.ID, c.DeviceID(), func(d *library.DeviceConfig) {
		if emuID != "" {
			ec := d.Emulators[emuID]
			ec.SteamAppID = res.AppID
			d.Emulators[emuID] = ec
			return
		}
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
	return c.RemoveFromSteamFor(gameID, "", userID)
}

// RemoveFromSteamFor deletes the shortcut(s) of one profile of a game.
func (c *Core) RemoveFromSteamFor(gameID, emuID, userID string) (int, error) {
	root := c.SteamRoot()
	if root == "" {
		return 0, errors.New("no Steam installation found on this device")
	}
	n, err := steam.RemoveShortcuts(root, userID, shortcutMatcher(gameID, emuID))
	if err != nil {
		return 0, err
	}
	_, _ = c.Library.UpdateDevice(gameID, c.DeviceID(), func(d *library.DeviceConfig) {
		if emuID == "" {
			d.SteamAppID = ""
		} else if ec, ok := d.Emulators[emuID]; ok {
			ec.SteamAppID = ""
			d.Emulators[emuID] = ec
		}
	})
	return n, nil
}
