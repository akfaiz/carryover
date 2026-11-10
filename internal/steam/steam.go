// Package steam locates a Steam installation and manages non-Steam shortcuts.
package steam

import (
	"errors"
	"fmt"
	"hash/crc32"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"

	"github.com/shirou/gopsutil/v4/process"
)

// steamID64Base converts between SteamID64 and the 32-bit account id used in userdata.
const steamID64Base = 76561197960265728

// FindRoot returns the Steam installation directory, or "" if not found.
func FindRoot() string {
	if v := os.Getenv("CARRYOVER_STEAM_ROOT"); v != "" {
		return v
	}
	for _, c := range candidateRoots() {
		if c == "" {
			continue
		}
		if fi, err := os.Stat(filepath.Join(c, "steamapps")); err == nil && fi.IsDir() {
			if r, err := filepath.EvalSymlinks(c); err == nil {
				return r
			}
			return c
		}
	}
	return ""
}

func candidateRoots() []string {
	home, _ := os.UserHomeDir()
	switch runtime.GOOS {
	case "windows":
		return append([]string{registrySteamPath()},
			`C:\Program Files (x86)\Steam`, `C:\Program Files\Steam`)
	case "darwin":
		return []string{filepath.Join(home, "Library", "Application Support", "Steam")}
	default:
		return []string{
			filepath.Join(home, ".steam", "steam"),
			filepath.Join(home, ".local", "share", "Steam"),
			filepath.Join(home, ".steam", "root"),
			filepath.Join(home, ".var", "app", "com.valvesoftware.Steam", ".local", "share", "Steam"),
			filepath.Join(home, "snap", "steam", "common", ".local", "share", "Steam"),
		}
	}
}

// Libraries returns all Steam library folders (including root).
func Libraries(root string) []string {
	if root == "" {
		return nil
	}
	libs := []string{root}
	b, err := os.ReadFile(filepath.Join(root, "steamapps", "libraryfolders.vdf"))
	if err != nil {
		return libs
	}
	doc, err := ParseText(string(b))
	if err != nil {
		return libs
	}
	lf := doc.Child("libraryfolders")
	if lf == nil {
		return libs
	}
	seen := map[string]bool{filepath.Clean(root): true}
	for _, c := range lf.Children {
		p := c.Get("path")
		if p == "" && !c.IsMap {
			p = c.Value // very old format
		}
		if p == "" {
			continue
		}
		p = filepath.Clean(p)
		if !seen[p] {
			seen[p] = true
			libs = append(libs, p)
		}
	}
	return libs
}

// CompatDataPrefix finds the Proton prefix user dir for appID
// (…/steamapps/compatdata/<appid>/pfx/drive_c/users/steamuser). If it does not
// exist in any library, the path under root is returned.
func CompatDataPrefix(root string, appID string) string {
	rel := filepath.Join("steamapps", "compatdata", appID, "pfx", "drive_c", "users", "steamuser")
	for _, lib := range Libraries(root) {
		p := filepath.Join(lib, rel)
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return filepath.Join(root, rel)
}

// User is a Steam account that has logged in on this machine.
type User struct {
	ID   string `json:"id"` // 32-bit account id (userdata folder name)
	Name string `json:"name"`
}

// Users lists accounts with a userdata folder.
func Users(root string) ([]User, error) {
	if root == "" {
		return nil, errors.New("steam installation not found")
	}
	entries, err := os.ReadDir(filepath.Join(root, "userdata"))
	if err != nil {
		return nil, err
	}
	names := loginUserNames(root)
	var users []User
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if _, err := strconv.ParseUint(e.Name(), 10, 32); err != nil || e.Name() == "0" {
			continue
		}
		u := User{ID: e.Name(), Name: names[e.Name()]}
		if u.Name == "" {
			u.Name = e.Name()
		}
		users = append(users, u)
	}
	sort.Slice(users, func(i, j int) bool { return users[i].Name < users[j].Name })
	return users, nil
}

func loginUserNames(root string) map[string]string {
	out := map[string]string{}
	b, err := os.ReadFile(filepath.Join(root, "config", "loginusers.vdf"))
	if err != nil {
		return out
	}
	doc, err := ParseText(string(b))
	if err != nil {
		return out
	}
	for _, u := range doc.Child("users").Children {
		id64, err := strconv.ParseUint(u.Key, 10, 64)
		if err != nil || id64 < steamID64Base {
			continue
		}
		out[strconv.FormatUint(id64-steamID64Base, 10)] = u.Get("PersonaName")
	}
	return out
}

// IsRunning reports whether the Steam client is running.
func IsRunning() bool {
	procs, err := process.Processes()
	if err != nil {
		return false
	}
	for _, p := range procs {
		n, err := p.Name()
		if err != nil {
			continue
		}
		n = strings.ToLower(n)
		if n == "steam.exe" || n == "steam" || n == "steam_osx" || n == "steamwebhelper" {
			return true
		}
	}
	return false
}

// ShortcutAppID computes the app id Steam assigns to a non-Steam shortcut.
func ShortcutAppID(exe, name string) uint32 {
	return crc32.ChecksumIEEE([]byte(exe+name)) | 0x80000000
}

// Shortcut is a non-Steam game entry.
type Shortcut struct {
	AppID         uint32   `json:"appId"`
	AppName       string   `json:"appName"`
	Exe           string   `json:"exe"` // quoted
	StartDir      string   `json:"startDir"`
	Icon          string   `json:"icon"`
	LaunchOptions string   `json:"launchOptions"`
	Tags          []string `json:"tags"`
}

func shortcutsPath(root, userID string) string {
	return filepath.Join(root, "userdata", userID, "config", "shortcuts.vdf")
}

func loadShortcuts(root, userID string) (*BinNode, *BinNode, error) {
	b, err := os.ReadFile(shortcutsPath(root, userID))
	if errors.Is(err, os.ErrNotExist) {
		doc := &BinNode{Type: TypeMap}
		list := &BinNode{Type: TypeMap, Key: "shortcuts"}
		doc.Children = append(doc.Children, list)
		return doc, list, nil
	}
	if err != nil {
		return nil, nil, err
	}
	doc, err := ParseBinary(b)
	if err != nil {
		return nil, nil, fmt.Errorf("parse shortcuts.vdf: %w", err)
	}
	list := doc.Child("shortcuts")
	if list == nil {
		list = &BinNode{Type: TypeMap, Key: "shortcuts"}
		doc.Children = append(doc.Children, list)
	}
	return doc, list, nil
}

func saveShortcuts(root, userID string, doc *BinNode) error {
	p := shortcutsPath(root, userID)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	if _, err := os.Stat(p); err == nil {
		if err := copyFile(p, p+".carryover.bak"); err != nil {
			return fmt.Errorf("backup shortcuts.vdf: %w", err)
		}
	}
	data, err := MarshalBinary(doc)
	if err != nil {
		return err
	}
	tmp := p + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, p)
}

// ListShortcuts returns all non-Steam shortcuts for a user.
func ListShortcuts(root, userID string) ([]Shortcut, error) {
	_, list, err := loadShortcuts(root, userID)
	if err != nil {
		return nil, err
	}
	var out []Shortcut
	for _, n := range list.Children {
		s := Shortcut{
			AppID:         n.GetInt("appid"),
			AppName:       n.GetString("AppName"),
			Exe:           n.GetString("Exe"),
			StartDir:      n.GetString("StartDir"),
			Icon:          n.GetString("icon"),
			LaunchOptions: n.GetString("LaunchOptions"),
		}
		if s.AppName == "" {
			s.AppName = n.GetString("appname")
		}
		if t := n.Child("tags"); t != nil {
			for _, tc := range t.Children {
				s.Tags = append(s.Tags, tc.Str)
			}
		}
		out = append(out, s)
	}
	return out, nil
}

// UpsertShortcut adds a shortcut, replacing any existing entry for which match returns true.
// It returns the app id written.
func UpsertShortcut(root, userID string, s Shortcut, match func(Shortcut) bool) (uint32, error) {
	doc, list, err := loadShortcuts(root, userID)
	if err != nil {
		return 0, err
	}
	if s.AppID == 0 {
		s.AppID = ShortcutAppID(s.Exe, s.AppName)
	}
	var target *BinNode
	existing, _ := ListShortcuts(root, userID)
	for i, e := range existing {
		if match != nil && match(e) && i < len(list.Children) {
			target = list.Children[i]
			break
		}
	}
	if target == nil {
		target = &BinNode{Type: TypeMap, Key: strconv.Itoa(len(list.Children))}
		list.Children = append(list.Children, target)
		target.SetInt("IsHidden", 0)
		target.SetInt("AllowDesktopConfig", 1)
		target.SetInt("AllowOverlay", 1)
		target.SetInt("OpenVR", 0)
		target.SetInt("Devkit", 0)
		target.SetString("DevkitGameID", "")
		target.SetInt("DevkitOverrideAppID", 0)
		target.SetInt("LastPlayTime", 0)
		target.SetString("FlatpakAppID", "")
		target.SetString("ShortcutPath", "")
	}
	target.SetInt("appid", s.AppID)
	target.SetString("AppName", s.AppName)
	target.SetString("Exe", s.Exe)
	target.SetString("StartDir", s.StartDir)
	target.SetString("icon", s.Icon)
	target.SetString("LaunchOptions", s.LaunchOptions)
	tags := target.Child("tags")
	if tags == nil {
		tags = &BinNode{Type: TypeMap, Key: "tags"}
		target.Children = append(target.Children, tags)
	}
	tags.Children = nil
	for i, t := range s.Tags {
		tags.Children = append(tags.Children, &BinNode{Type: TypeString, Key: strconv.Itoa(i), Str: t})
	}
	return s.AppID, saveShortcuts(root, userID, doc)
}

// RemoveShortcuts deletes shortcuts matching match, returning how many were removed.
func RemoveShortcuts(root, userID string, match func(Shortcut) bool) (int, error) {
	doc, list, err := loadShortcuts(root, userID)
	if err != nil {
		return 0, err
	}
	existing, err := ListShortcuts(root, userID)
	if err != nil {
		return 0, err
	}
	var kept []*BinNode
	removed := 0
	for i, n := range list.Children {
		if i < len(existing) && match(existing[i]) {
			removed++
			continue
		}
		kept = append(kept, n)
	}
	if removed == 0 {
		return 0, nil
	}
	for i, n := range kept {
		n.Key = strconv.Itoa(i)
	}
	list.Children = kept
	return removed, saveShortcuts(root, userID, doc)
}

// SetGridImage copies an image as the portrait capsule art (<appid>p.ext) for a shortcut.
func SetGridImage(root, userID string, appID uint32, imagePath string) error {
	ext := strings.ToLower(filepath.Ext(imagePath))
	if ext == "" {
		ext = ".png"
	}
	dir := filepath.Join(root, "userdata", userID, "config", "grid")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	return copyFile(imagePath, filepath.Join(dir, fmt.Sprintf("%dp%s", appID, ext)))
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}
