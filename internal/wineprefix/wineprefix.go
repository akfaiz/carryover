// Package wineprefix finds the Wine prefixes that Linux game launchers other
// than Steam create (Heroic, Lutris, Bottles, plain Wine), so save locations
// inside them can be suggested. Steam's own Proton prefixes are handled by
// the {proton:...} path tokens instead.
//
// Only the launchers' known folders and their own config files are read,
// never a blind walk of the home folder, and the result is bounded.
package wineprefix

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// Launchers a prefix can come from.
const (
	Heroic  = "heroic"
	Lutris  = "lutris"
	Bottles = "bottles"
	Wine    = "wine" // a prefix no launcher config claims (e.g. ~/.wine)
)

// Prefix is a Wine prefix found on this device.
type Prefix struct {
	Launcher string
	// Name is the game title (Heroic, Lutris) or the bottle / folder name.
	Name string
	// Games are other titles known to run in the prefix (programs added to a
	// bottle).
	Games []string
	// Path is the prefix folder, the one holding drive_c.
	Path string
	// Users are the user folders, drive_c/users/<name>, except Public.
	Users []string
}

// Roots tells Find where to look. Tests point it at a made-up machine.
type Roots struct {
	Home string
	// Drives are the roots of other mounted drives (SD card, second disk),
	// where Heroic is often told to install.
	Drives []string
}

// maxPrefixes bounds the result: a pathological library (hundreds of
// bottles) must not make suggestions slow.
const maxPrefixes = 200

// mountBases are where SteamOS and Linux desktops mount other drives.
var mountBases = []string{"/run/media", "/media", "/mnt", "/var/mnt"}

// DefaultRoots returns this device's home folder and mounted drives.
func DefaultRoots() Roots {
	home, _ := os.UserHomeDir()
	return Roots{Home: home, Drives: drivesUnder(mountBases)}
}

// drivesUnder lists the folders under each base that may be a drive.
// /run/media and /media hold drives directly (/run/media/mmcblk0p1, older
// SteamOS) or per user (/run/media/deck/<label>), so both levels count there.
func drivesUnder(bases []string) []string {
	var out []string
	for _, base := range bases {
		twoLevels := filepath.Base(base) == "media"
		for _, a := range subdirs(base) {
			first := filepath.Join(base, a)
			out = append(out, first)
			if twoLevels {
				for _, b := range subdirs(first) {
					out = append(out, filepath.Join(first, b))
				}
			}
		}
	}
	return out
}

// finder collects prefixes, keyed by cleaned path. The first source to name a
// prefix wins, so config files (with real titles) are read before folder
// listings (with folder names).
type finder struct {
	out  []Prefix
	seen map[string]int
}

func (f *finder) add(launcher, name, path string, games ...string) {
	if path == "" || len(f.out) >= maxPrefixes {
		return
	}
	path = filepath.Clean(path)
	if i, ok := f.seen[path]; ok {
		f.out[i].Games = append(f.out[i].Games, games...)
		return
	}
	root, ok := prefixRoot(path)
	if !ok {
		return
	}
	if i, ok := f.seen[root]; ok {
		f.seen[path] = i
		f.out[i].Games = append(f.out[i].Games, games...)
		return
	}
	users := userDirs(root)
	if len(users) == 0 {
		return
	}
	if name == "" {
		name = filepath.Base(path)
	}
	f.seen[path], f.seen[root] = len(f.out), len(f.out)
	f.out = append(f.out, Prefix{Launcher: launcher, Name: name, Games: games, Path: root, Users: users})
}

// addChildren adds every prefix directly inside parent.
func (f *finder) addChildren(launcher, parent string) {
	for _, sub := range subdirs(parent) {
		f.add(launcher, "", filepath.Join(parent, sub))
	}
}

// Find returns the prefixes on this machine.
func Find(r Roots) []Prefix {
	if r.Home == "" {
		return nil
	}
	f := &finder{seen: map[string]int{}}
	home := r.Home
	var heroicParents []string
	for _, dir := range heroicConfigDirs(home) {
		heroicParents = append(heroicParents, f.heroicConfig(dir, home)...)
	}
	for _, dir := range lutrisGameDirs(home) {
		f.lutrisGames(dir, home)
	}
	for _, dir := range bottlesDirs(home) {
		f.bottles(dir)
	}

	// Folders the launchers create prefixes in by default, here and on the
	// other drives.
	heroicParents = append(heroicParents,
		filepath.Join(home, "Games", "Heroic", "Prefixes"),
		filepath.Join(home, "Games", "Heroic", "Prefixes", "default"),
		filepath.Join(home, ".var", "app", heroicFlatpak, "config", "heroic", "Prefixes"),
		filepath.Join(home, ".var", "app", heroicFlatpak, "config", "heroic", "Prefixes", "default"),
	)
	for _, d := range r.Drives {
		heroicParents = append(heroicParents,
			filepath.Join(d, "Heroic", "Prefixes"),
			filepath.Join(d, "Heroic", "Prefixes", "default"),
			filepath.Join(d, "Games", "Heroic", "Prefixes"),
			filepath.Join(d, "Games", "Heroic", "Prefixes", "default"),
		)
	}
	for _, p := range heroicParents {
		f.addChildren(Heroic, p)
	}
	for _, p := range []string{
		filepath.Join(home, ".local", "share", "lutris", "prefixes"),
		filepath.Join(home, ".var", "app", lutrisFlatpak, "data", "lutris", "prefixes"),
	} {
		f.addChildren(Lutris, p)
	}
	// Lutris installs into ~/Games/<slug> by default; anything there that a
	// Lutris config didn't already claim is just a Wine prefix.
	f.addChildren(Wine, filepath.Join(home, "Games"))
	f.addChildren(Wine, filepath.Join(home, ".local", "share", "wineprefixes"))
	f.add(Wine, "Wine", filepath.Join(home, ".wine"))
	return f.out
}

// prefixRoot returns the folder holding drive_c: the prefix itself, or its
// pfx subfolder (a Proton prefix, as Heroic makes for its Proton runner).
func prefixRoot(p string) (string, bool) {
	for _, root := range []string{p, filepath.Join(p, "pfx")} {
		if isDir(filepath.Join(root, "drive_c")) {
			return root, true
		}
	}
	return "", false
}

// userDirs returns the per-user folders of a prefix. Steam's Proton always
// uses "steamuser", but Wine names it after the account.
func userDirs(prefix string) []string {
	usersDir := filepath.Join(prefix, "drive_c", "users")
	var out []string
	for _, u := range subdirs(usersDir) {
		switch strings.ToLower(u) {
		case "public", "default", "default user", "all users":
			continue
		}
		out = append(out, filepath.Join(usersDir, u))
	}
	return out
}

// --- Heroic ---

const heroicFlatpak = "com.heroicgameslauncher.hgl"

func heroicConfigDirs(home string) []string {
	return []string{
		filepath.Join(home, ".config", "heroic"),
		filepath.Join(home, ".var", "app", heroicFlatpak, "config", "heroic"),
	}
}

// maxConfigFiles bounds how many per-game config files are read per folder.
const maxConfigFiles = 500

// heroicConfig adds each game's prefix from GamesConfig/<appName>.json,
// titled from Heroic's library caches, and returns the folders Heroic makes
// new prefixes in (config.json).
func (f *finder) heroicConfig(dir, home string) (parents []string) {
	var cfg struct {
		DefaultSettings struct {
			DefaultWinePrefix string `json:"defaultWinePrefix"`
			WinePrefix        string `json:"winePrefix"`
		} `json:"defaultSettings"`
	}
	if readJSON(filepath.Join(dir, "config.json"), &cfg) {
		for _, p := range []string{cfg.DefaultSettings.DefaultWinePrefix, cfg.DefaultSettings.WinePrefix} {
			if p = absPath(p, home); p != "" {
				parents = append(parents, p)
			}
		}
	}
	titles := heroicTitles(dir)
	entries, _ := os.ReadDir(filepath.Join(dir, "GamesConfig"))
	for i, e := range entries {
		if i >= maxConfigFiles {
			break
		}
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		// {"<appName>": {"winePrefix": "...", ...}, "version": "v0"}
		var byGame map[string]json.RawMessage
		if !readJSON(filepath.Join(dir, "GamesConfig", e.Name()), &byGame) {
			continue
		}
		for app, raw := range byGame {
			var g struct {
				WinePrefix string `json:"winePrefix"`
			}
			if json.Unmarshal(raw, &g) != nil {
				continue
			}
			if p := absPath(g.WinePrefix, home); p != "" {
				f.add(Heroic, titles[app], p)
			}
		}
	}
	return parents
}

// heroicTitles maps Heroic's opaque app names to game titles, from its
// library caches (GOG, Epic, Amazon, and games added by hand).
func heroicTitles(dir string) map[string]string {
	type game struct {
		AppName string `json:"app_name"`
		Title   string `json:"title"`
	}
	titles := map[string]string{}
	for _, name := range []string{
		"store_cache/gog_library.json", "gog_store/library.json",
		"store_cache/legendary_library.json", "store_cache/nile_library.json",
		"sideload_apps/library.json",
	} {
		var lib struct {
			Games   []game `json:"games"`
			Library []game `json:"library"`
		}
		if !readJSON(filepath.Join(dir, filepath.FromSlash(name)), &lib) {
			continue
		}
		for _, g := range append(lib.Games, lib.Library...) {
			if g.AppName != "" && g.Title != "" {
				titles[g.AppName] = g.Title
			}
		}
	}
	return titles
}

// --- Lutris ---

const lutrisFlatpak = "net.lutris.Lutris"

func lutrisGameDirs(home string) []string {
	return []string{
		filepath.Join(home, ".local", "share", "lutris", "games"),
		filepath.Join(home, ".config", "lutris", "games"),
		filepath.Join(home, ".var", "app", lutrisFlatpak, "data", "lutris", "games"),
		filepath.Join(home, ".var", "app", lutrisFlatpak, "config", "lutris", "games"),
	}
}

// lutrisFileSuffix is the "-<timestamp>" Lutris appends to a game file name.
var lutrisFileSuffix = regexp.MustCompile(`-\d+$`)

// lutrisGames adds the prefix of each game in a Lutris games folder
// (<slug>-<timestamp>.yml).
func (f *finder) lutrisGames(dir, home string) {
	entries, _ := os.ReadDir(dir)
	for i, e := range entries {
		if i >= maxConfigFiles {
			break
		}
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".yml") {
			continue
		}
		var spec struct {
			Name     string `yaml:"name"`
			GameSlug string `yaml:"game_slug"`
			Game     struct {
				Prefix string `yaml:"prefix"`
			} `yaml:"game"`
		}
		b, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil || yaml.Unmarshal(b, &spec) != nil {
			continue
		}
		p := absPath(spec.Game.Prefix, home)
		if p == "" {
			continue
		}
		name := spec.Name
		if name == "" {
			name = spec.GameSlug
		}
		if name == "" {
			name = lutrisFileSuffix.ReplaceAllString(strings.TrimSuffix(e.Name(), ".yml"), "")
		}
		f.add(Lutris, name, p)
	}
}

// --- Bottles ---

func bottlesDirs(home string) []string {
	return []string{
		filepath.Join(home, ".local", "share", "bottles", "bottles"),
		filepath.Join(home, ".var", "app", "com.usebottles.bottles", "data", "bottles", "bottles"),
	}
}

// bottles adds each bottle, with the programs added to it as its games.
func (f *finder) bottles(dir string) {
	for _, sub := range subdirs(dir) {
		p := filepath.Join(dir, sub)
		var spec struct {
			Name     string `yaml:"Name"`
			Programs map[string]struct {
				Name string `yaml:"name"`
			} `yaml:"External_Programs"`
		}
		if b, err := os.ReadFile(filepath.Join(p, "bottle.yml")); err == nil {
			_ = yaml.Unmarshal(b, &spec)
		}
		var games []string
		for _, prog := range spec.Programs {
			if prog.Name != "" {
				games = append(games, prog.Name)
			}
		}
		f.add(Bottles, spec.Name, p, games...)
	}
}

// --- helpers ---

func readJSON(p string, v any) bool {
	b, err := os.ReadFile(p)
	return err == nil && json.Unmarshal(b, v) == nil
}

// absPath resolves a leading ~ against home; anything still not absolute is
// dropped ("" means none).
func absPath(p, home string) string {
	p = strings.TrimSpace(p)
	switch {
	case p == "~":
		p = home
	case strings.HasPrefix(p, "~/"):
		p = filepath.Join(home, p[2:])
	}
	if !filepath.IsAbs(p) && !strings.HasPrefix(p, "/") {
		return ""
	}
	return filepath.Clean(filepath.FromSlash(p))
}

func isDir(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && fi.IsDir()
}

func subdirs(dir string) []string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range entries {
		if e.IsDir() || (e.Type()&os.ModeSymlink != 0 && isDir(filepath.Join(dir, e.Name()))) {
			out = append(out, e.Name())
		}
	}
	return out
}
