// Package lutris reads the games installed in Lutris (Linux), so a game can
// be started through Lutris, with its own runner, Wine build and prefix,
// instead of directly or through a prefix Steam would make for it.
//
// Only Lutris's per-game config files (games/<slug>-<timestamp>.yml) are
// read, never its pga.db database, so no SQLite driver is needed.
package lutris

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"

	"gopkg.in/yaml.v3"
)

// FlatpakID is Lutris's Flatpak application id.
const FlatpakID = "net.lutris.Lutris"

// maxConfigFiles bounds how many game configs are read per folder.
const maxConfigFiles = 500

// Game is a game Lutris has installed.
type Game struct {
	// ConfigID is the config file's name without ".yml": <slug>-<timestamp>.
	ConfigID string
	// Slug is the game's slug in Lutris's library, which `lutris:rungame/`
	// takes.
	Slug string
	Name string
	// Exe is the game's executable (game.exe), "" when the game has none
	// (runners that start a ROM or a script, with main_file).
	Exe     string
	WorkDir string
	// Prefix is the game's Wine prefix, "" when it has none.
	Prefix string
	// Flatpak is true when the game belongs to the Flatpak build of Lutris.
	Flatpak bool
}

// GameDir is a folder of Lutris game configs and whether it is the Flatpak
// build's.
type GameDir struct {
	Path    string
	Flatpak bool
}

// GameDirs returns where Lutris keeps its game configs: the XDG data folder
// (current versions), the XDG config folder (older ones) and the same two
// inside the Flatpak build's sandbox.
func GameDirs(home string) []GameDir {
	flat := filepath.Join(home, ".var", "app", FlatpakID)
	return []GameDir{
		{Path: filepath.Join(home, ".local", "share", "lutris", "games")},
		{Path: filepath.Join(home, ".config", "lutris", "games")},
		{Path: filepath.Join(flat, "data", "lutris", "games"), Flatpak: true},
		{Path: filepath.Join(flat, "config", "lutris", "games"), Flatpak: true},
	}
}

// fileSuffix is the "-<timestamp>" Lutris appends to a game config's name.
var fileSuffix = regexp.MustCompile(`-\d+$`)

// Installed lists the games Lutris has configured for the user with this home
// folder. A game with no executable is still listed (its prefix is what
// matters for save suggestions); callers that need to start it check Exe.
func Installed(home string) []Game {
	if home == "" {
		return nil
	}
	var out []Game
	seen := map[string]bool{}
	for _, d := range GameDirs(home) {
		entries, _ := os.ReadDir(d.Path)
		for i, e := range entries {
			if i >= maxConfigFiles {
				break
			}
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".yml") {
				continue
			}
			id := strings.TrimSuffix(e.Name(), ".yml")
			if seen[id] {
				continue
			}
			if g, ok := readGame(filepath.Join(d.Path, e.Name()), id, home); ok {
				g.Flatpak = d.Flatpak
				seen[id] = true
				out = append(out, g)
			}
		}
	}
	return out
}

// readGame parses one game config.
func readGame(path, id, home string) (Game, bool) {
	var spec struct {
		Name     string `yaml:"name"`
		Slug     string `yaml:"slug"`
		GameSlug string `yaml:"game_slug"`
		Game     struct {
			Exe     string `yaml:"exe"`
			Prefix  string `yaml:"prefix"`
			WorkDir string `yaml:"working_dir"`
		} `yaml:"game"`
	}
	b, err := os.ReadFile(path)
	if err != nil || yaml.Unmarshal(b, &spec) != nil {
		return Game{}, false
	}
	fileSlug := fileSuffix.ReplaceAllString(id, "")
	if fileSlug == "" {
		return Game{}, false
	}
	// Lutris renames the file to <library slug>-<timestamp> when a game's
	// settings are saved, so the file name is the current slug. A file never
	// renamed still carries the installer's slug in "slug"; the library's
	// slug is then "game_slug".
	slug := fileSlug
	if spec.GameSlug != "" && spec.Slug == fileSlug {
		slug = spec.GameSlug
	}
	name := strings.TrimSpace(spec.Name)
	if name == "" {
		name = titleOf(slug)
	}
	return Game{
		ConfigID: id,
		Slug:     slug,
		Name:     name,
		Exe:      absPath(spec.Game.Exe, home),
		WorkDir:  absPath(spec.Game.WorkDir, home),
		Prefix:   absPath(spec.Game.Prefix, home),
	}, true
}

// ByConfigID returns the installed game with this config id.
func ByConfigID(home, id string) (Game, bool) {
	for _, g := range Installed(home) {
		if g.ConfigID == id {
			return g, true
		}
	}
	return Game{}, false
}

// ForExe returns the installed game that exe belongs to: the one whose game.exe
// it is or, failing that, the one with the deepest folder (the executable's
// folder, the working directory or the Wine prefix) that holds it.
func ForExe(home, exe string) (Game, bool) {
	exe = filepath.Clean(exe)
	var best Game
	depth := -1
	for _, g := range Installed(home) {
		if g.Exe != "" && g.Exe == exe {
			return g, true
		}
		for _, dir := range []string{dirOf(g.Exe), g.WorkDir, g.Prefix} {
			if dir == "" {
				continue
			}
			if dir = filepath.Clean(dir); inDir(exe, dir) && len(dir) > depth {
				best, depth = g, len(dir)
			}
		}
	}
	return best, depth >= 0
}

// URL is the link that makes Lutris start the game. rungame takes the library
// slug and, unlike a bare slug, never opens an installer for an unknown one.
func (g Game) URL() string { return "lutris:rungame/" + g.Slug }

// lookPath finds a program on PATH; tests replace it.
var lookPath = exec.LookPath

// nativePaths are where packages put the native Lutris, besides PATH.
var nativePaths = []string{"/usr/bin/lutris", "/usr/local/bin/lutris"}

// Command returns the command that starts the game through Lutris.
func (g Game) Command() []string {
	if g.Flatpak {
		return []string{"flatpak", "run", FlatpakID, g.URL()}
	}
	if p, err := lookPath("lutris"); err == nil {
		return []string{p, g.URL()}
	}
	for _, p := range nativePaths {
		if _, err := os.Stat(p); err == nil {
			return []string{p, g.URL()}
		}
	}
	// Not found (e.g. an AppImage): ask the desktop to open the link with
	// whatever handles lutris:.
	return []string{"xdg-open", g.URL()}
}

// titleOf makes a title from a slug: "the-witcher-3" -> "The Witcher 3".
func titleOf(slug string) string {
	words := strings.Split(strings.NewReplacer("-", " ", "_", " ").Replace(slug), " ")
	for i, w := range words {
		if w != "" {
			r := []rune(w)
			r[0] = unicode.ToUpper(r[0])
			words[i] = string(r)
		}
	}
	return strings.TrimSpace(strings.Join(words, " "))
}

func dirOf(p string) string {
	if p == "" {
		return ""
	}
	return filepath.Dir(p)
}

// inDir reports whether p is inside dir.
func inDir(p, dir string) bool { return strings.HasPrefix(p, dir+string(filepath.Separator)) }

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
