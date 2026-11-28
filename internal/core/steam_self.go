package core

import (
	"path/filepath"
	"strconv"
	"strings"

	"github.com/akfaiz/carryover/internal/app/msg"
	"github.com/akfaiz/carryover/internal/stores/steam"
)

// selfShortcutName is the name of Carryover's own shortcut in Steam.
const selfShortcutName = "Carryover"

// SelfArtKinds are the kinds of library art Carryover's own shortcut gets, in
// the order they are written.
var SelfArtKinds = []string{"icon", "cover", "wide", "hero", "logo"}

// isSelfShortcut matches Carryover's own shortcut: one that starts Carryover
// (this executable, or another copy of it, as after the AppImage was moved or
// renamed) without running a game.
func isSelfShortcut(self string) func(steam.Shortcut) bool {
	return func(s steam.Shortcut) bool {
		if _, _, ok := parseRunOptions(s.LaunchOptions); ok {
			return false
		}
		if s.Exe == quote(self) {
			return true
		}
		exe := strings.Trim(s.Exe, `"`)
		// The base name is taken by hand: a shortcut written on Windows keeps
		// its backslashes, which filepath.Base splits only on Windows.
		if i := strings.LastIndexAny(exe, `/\`); i >= 0 {
			exe = exe[i+1:]
		}
		return strings.HasPrefix(strings.ToLower(exe), "carryover")
	}
}

// AddSelfToSteam adds Carryover itself to Steam as a non-Steam game (or updates
// its shortcut), so it can be opened from Steam Deck Game Mode and used with a
// controller. art holds its library images, PNG data by kind (SelfArtKinds);
// a missing kind is left as it is.
func (c *Core) AddSelfToSteam(userID string, art map[string][]byte) (AddToSteamResult, error) {
	var res AddToSteamResult
	root := c.SteamRoot()
	if root == "" {
		return res, msg.SteamNotFound.Err()
	}
	if userID == "" {
		return res, msg.SteamPickAccount.Err()
	}
	self, err := SelfExecutable()
	if err != nil {
		return res, err
	}
	sc := steam.Shortcut{
		AppName:  selfShortcutName,
		Exe:      quote(self),
		StartDir: quote(filepath.Dir(self)),
		Tags:     []string{"Carryover"},
	}
	match := isSelfShortcut(self)
	// The icon is referenced from the shortcut itself, so it is written first,
	// under the app id the shortcut keeps.
	if data := art["icon"]; len(data) > 0 {
		id := steam.ShortcutAppID(sc.Exe, sc.AppName)
		if list, err := steam.ListShortcuts(root, userID); err == nil {
			for _, s := range list {
				if match(s) {
					id = s.AppID
					break
				}
			}
		}
		if dst, err := steam.SetGridArtData(root, userID, id, "icon", ".png", data); err == nil {
			sc.Icon = dst
		}
	}
	appID, err := steam.UpsertShortcut(root, userID, sc, match)
	if err != nil {
		return res, err
	}
	for _, kind := range SelfArtKinds {
		if data := art[kind]; kind != "icon" && len(data) > 0 {
			if _, err := steam.SetGridArtData(root, userID, appID, kind, ".png", data); err != nil {
				return res, err
			}
		}
	}
	res.AppID = strconv.FormatUint(uint64(appID), 10)
	res.SteamRunning = steam.IsRunning()
	return res, nil
}

// SelfSteamAppID returns the app id of Carryover's own shortcut for a Steam
// user, or "" if it has none.
func (c *Core) SelfSteamAppID(userID string) (string, error) {
	root := c.SteamRoot()
	if root == "" || userID == "" {
		return "", nil
	}
	self, err := SelfExecutable()
	if err != nil {
		return "", err
	}
	list, err := steam.ListShortcuts(root, userID)
	if err != nil {
		return "", err
	}
	match := isSelfShortcut(self)
	for _, s := range list {
		if match(s) {
			return strconv.FormatUint(uint64(s.AppID), 10), nil
		}
	}
	return "", nil
}

// RemoveSelfFromSteam deletes Carryover's own shortcut for a Steam user.
func (c *Core) RemoveSelfFromSteam(userID string) (int, error) {
	root := c.SteamRoot()
	if root == "" {
		return 0, msg.SteamNotFound.Err()
	}
	self, err := SelfExecutable()
	if err != nil {
		return 0, err
	}
	return steam.RemoveShortcuts(root, userID, isSelfShortcut(self))
}
