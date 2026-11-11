// Package heroic reads the games installed by Heroic Games Launcher (GOG,
// Epic through Legendary, Amazon through Nile, and games added by hand), so a
// game can be started through Heroic, in Heroic's own Wine prefix, instead of
// a prefix Steam would make for it.
package heroic

import (
	"encoding/json"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// FlatpakID is Heroic's Flatpak application id.
const FlatpakID = "com.heroicgameslauncher.hgl"

// Runners.
const (
	GOG       = "gog"
	Legendary = "legendary"
	Nile      = "nile"
	Sideload  = "sideload"
)

// Game is a game Heroic has installed.
type Game struct {
	Runner  string
	AppName string // Heroic's id for the game, e.g. a GOG product id
	Title   string
	// InstallPath is the game folder, or for a sideloaded game the folder of
	// its executable.
	InstallPath string
	// Flatpak is true when the game belongs to the Flatpak build of Heroic.
	Flatpak bool
}

// ConfigDir is a Heroic config folder and whether it is the Flatpak build's.
type ConfigDir struct {
	Path    string
	Flatpak bool
}

// ConfigDirs returns where Heroic keeps its config: on Linux the native
// build's and the Flatpak build's, on Windows %APPDATA%\heroic.
func ConfigDirs(home string) []ConfigDir { return configDirs(home) }

// LinuxConfigDirs returns Heroic's config folders on Linux, whatever this
// device runs: the native build's and the Flatpak build's.
func LinuxConfigDirs(home string) []ConfigDir {
	return []ConfigDir{
		{Path: filepath.Join(home, ".config", "heroic")},
		{Path: filepath.Join(home, ".var", "app", FlatpakID, "config", "heroic"), Flatpak: true},
	}
}

// Installed lists the games installed by Heroic for the user with this home
// folder.
func Installed(home string) []Game {
	if home == "" {
		return nil
	}
	var out []Game
	for _, d := range ConfigDirs(home) {
		out = append(out, installedIn(d, home)...)
	}
	return out
}

// ForExe returns the installed game whose folder holds exe (the deepest one if
// folders nest).
func ForExe(home, exe string) (Game, bool) {
	exe = filepath.Clean(exe)
	var best Game
	for _, g := range Installed(home) {
		dir := filepath.Clean(g.InstallPath)
		if inDir(exe, dir) && len(dir) > len(best.InstallPath) {
			best = g
			best.InstallPath = dir
		}
	}
	return best, best.AppName != ""
}

// URL is the heroic:// link that makes Heroic launch the game, the same one
// Heroic's own "Add to Steam" uses.
func (g Game) URL() string {
	q := url.Values{}
	q.Set("appName", g.AppName)
	q.Set("runner", g.Runner)
	return "heroic://launch?" + q.Encode()
}

// Command returns the command that launches the game through Heroic, without
// its window (--no-gui), as Heroic's own Steam shortcuts do.
func (g Game) Command() []string {
	if g.Flatpak {
		return []string{"flatpak", "run", FlatpakID, "--no-gui", "--no-sandbox", g.URL()}
	}
	for _, name := range executables() {
		if p, err := exec.LookPath(name); err == nil {
			return append([]string{p}, launchArgs(g.URL())...)
		}
	}
	// A build not found (e.g. an AppImage): ask the desktop to open the link
	// with whatever handles heroic://.
	return openURL(g.URL())
}

func installedIn(d ConfigDir, home string) []Game {
	titles := Titles(d.Path)
	var out []Game
	add := func(runner, app, path string) {
		if path = absPath(path, home); app == "" || path == "" {
			return
		}
		out = append(out, Game{Runner: runner, AppName: app, Title: titles[app], InstallPath: path, Flatpak: d.Flatpak})
	}

	// GOG: {"installed": [{"appName": "...", "install_path": "..."}]}
	var gog struct {
		Installed []struct {
			AppName     string `json:"appName"`
			InstallPath string `json:"install_path"`
		} `json:"installed"`
	}
	if readJSON(filepath.Join(d.Path, "gog_store", "installed.json"), &gog) {
		for _, g := range gog.Installed {
			add(GOG, g.AppName, g.InstallPath)
		}
	}

	// Epic (Legendary): {"<app_name>": {"app_name": "...", "install_path": "...", "title": "..."}}
	var epic map[string]struct {
		AppName     string `json:"app_name"`
		InstallPath string `json:"install_path"`
		Title       string `json:"title"`
	}
	if readJSON(filepath.Join(d.Path, "legendaryConfig", "legendary", "installed.json"), &epic) {
		for key, g := range epic {
			app := g.AppName
			if app == "" {
				app = key
			}
			if titles[app] == "" && g.Title != "" {
				titles[app] = g.Title
			}
			add(Legendary, app, g.InstallPath)
		}
	}

	// Amazon (Nile): [{"id": "...", "path": "..."}]
	var amazon []struct {
		ID   string `json:"id"`
		Path string `json:"path"`
	}
	if readJSON(filepath.Join(d.Path, "nile_config", "nile", "installed.json"), &amazon) {
		for _, g := range amazon {
			add(Nile, g.ID, g.Path)
		}
	}

	// Added by hand: {"games": [{"app_name": "...", "install": {"executable": "..."}}]}
	var side struct {
		Games []struct {
			AppName string `json:"app_name"`
			Install struct {
				Executable string `json:"executable"`
			} `json:"install"`
		} `json:"games"`
	}
	if readJSON(filepath.Join(d.Path, "sideload_apps", "library.json"), &side) {
		for _, g := range side.Games {
			if exe := absPath(g.Install.Executable, home); exe != "" {
				add(Sideload, g.AppName, filepath.Dir(exe))
			}
		}
	}
	return out
}

// Titles maps Heroic's opaque app names to game titles, from its library
// caches (GOG, Epic, Amazon, and games added by hand) in config folder dir.
func Titles(dir string) map[string]string {
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

func readJSON(p string, v any) bool {
	b, err := os.ReadFile(p)
	return err == nil && json.Unmarshal(b, v) == nil
}

// absPath resolves a leading ~ against home; anything still not absolute is
// dropped ("" means none).
func absPath(p, home string) string {
	p = strings.TrimSpace(p)
	switch {
	case p == "":
		return ""
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
