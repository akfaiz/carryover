// Package bottles reads the bottles of Bottles (Linux) and the programs added
// to them, so a game can be started through Bottles, in its bottle and with
// its runner, instead of directly or in a prefix Steam would make for it, and
// so save locations in a bottle can be suggested.
//
// Only Bottles's own config files are read: data.yml (where the bottles are),
// each bottle's bottle.yml and, for a bottle kept elsewhere, placeholder.yml.
package bottles

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	"gopkg.in/yaml.v3"
)

// FlatpakID is Bottles's Flatpak application id.
const FlatpakID = "com.usebottles.bottles"

// maxBottles bounds how many bottles are read per installation.
const maxBottles = 200

// Bottle is one bottle: a Wine prefix Bottles manages.
type Bottle struct {
	// Name is what Bottles calls it, which "bottles-cli run -b" takes.
	Name string
	// Path is the bottle's folder, the one holding drive_c.
	Path string
	// Programs are the programs added to the bottle in Bottles.
	Programs []Program
	// Configured is false for a folder without a readable bottle.yml:
	// Bottles doesn't list it, so nothing can be started in it.
	Configured bool
	// Flatpak is true when the bottle belongs to the Flatpak build.
	Flatpak bool
}

// Program is a program added to a bottle.
type Program struct {
	ID   string
	Name string
	// Exe is the program's file on this device, "" when its path can't be
	// found from here.
	Exe string
}

// Game is a program of a bottle, or any other executable of one, that Bottles
// can start.
type Game struct {
	Bottle string `json:"bottle"`
	// Program is the name Bottles has for it, "" for an executable that wasn't
	// added to the bottle (or whose name another program there has too): it is
	// then started by its path.
	Program   string `json:"program"`
	ProgramID string `json:"programId"`
	Name      string `json:"name"`
	Exe       string `json:"exe"`
	// Prefix is the bottle's folder.
	Prefix  string `json:"prefix"`
	Flatpak bool   `json:"flatpak"`
}

// install is one installation's data folder (it holds data.yml and, by
// default, the bottles).
type install struct {
	data    string
	flatpak bool
}

// installs returns the data folders of the native Bottles and of its Flatpak.
func installs(home string) []install {
	return []install{
		{data: filepath.Join(home, ".local", "share", "bottles")},
		{data: filepath.Join(home, ".var", "app", FlatpakID, "data", "bottles"), flatpak: true},
	}
}

// bottlesDir is the folder an installation keeps its bottles in: the one the
// user chose in Bottles' settings, if it is there, else the default.
func bottlesDir(home string, in install) string {
	var data struct {
		Custom string `yaml:"custom_bottles_path"`
	}
	readYAML(filepath.Join(in.data, "data.yml"), &data)
	if p := hostPath(data.Custom, home, in.flatpak); p != "" && !isPortal(p) && isDir(p) {
		return p
	}
	return filepath.Join(in.data, "bottles")
}

// isPortal reports a path of the document portal, which Bottles ignores as a
// place for bottles.
func isPortal(p string) bool {
	p = filepath.ToSlash(p)
	return strings.HasPrefix(p, "/run/user/") && strings.Contains(p, "/doc/")
}

// hostPath makes a path from a config file one of this device's: inside the
// Flatpak's sandbox, /var/data is its data folder. "" if it isn't absolute.
func hostPath(p, home string, flatpak bool) string {
	p = strings.TrimSpace(p)
	if flatpak {
		if rest, ok := strings.CutPrefix(p, "/var/data/"); ok {
			return filepath.Join(home, ".var", "app", FlatpakID, "data", filepath.FromSlash(rest))
		}
	}
	if !filepath.IsAbs(p) && !strings.HasPrefix(p, "/") {
		return ""
	}
	return filepath.Clean(filepath.FromSlash(p))
}

type bottleYML struct {
	Name       string `yaml:"Name"`
	Path       string `yaml:"Path"`
	CustomPath bool   `yaml:"Custom_Path"`
	Programs   map[string]struct {
		ID      string `yaml:"id"`
		Name    string `yaml:"name"`
		Path    string `yaml:"path"`
		Removed bool   `yaml:"removed"`
	} `yaml:"External_Programs"`
}

// List returns the bottles of the user with this home folder.
func List(home string) []Bottle {
	if home == "" {
		return nil
	}
	var out []Bottle
	seen := map[string]bool{}
	for _, in := range installs(home) {
		dir := bottlesDir(home, in)
		entries, _ := os.ReadDir(dir)
		n := 0
		for _, e := range entries {
			if n >= maxBottles {
				break
			}
			folder := filepath.Join(dir, e.Name())
			if !isDir(folder) { // follows links
				continue
			}
			b, ok := read(home, folder, in.flatpak)
			if !ok || seen[b.Path] {
				continue
			}
			seen[b.Path] = true
			n++
			out = append(out, b)
		}
	}
	return out
}

// read reads the bottle in folder, or the one folder points to.
func read(home, folder string, flatpak bool) (Bottle, bool) {
	path := folder
	// A bottle kept somewhere else leaves a placeholder.yml naming the place.
	var placeholder struct {
		Path string `yaml:"Path"`
	}
	if readYAML(filepath.Join(folder, "placeholder.yml"), &placeholder) {
		if path = hostPath(placeholder.Path, home, flatpak); path == "" {
			return Bottle{}, false
		}
	}
	var spec bottleYML
	configured := readYAML(filepath.Join(path, "bottle.yml"), &spec)
	if configured && spec.CustomPath && path == folder {
		if p := hostPath(spec.Path, home, flatpak); p != "" && p != folder {
			path, spec = p, bottleYML{}
			configured = readYAML(filepath.Join(path, "bottle.yml"), &spec)
		}
	}
	if !isDir(path) {
		return Bottle{}, false // e.g. on a drive that isn't mounted
	}
	b := Bottle{Name: spec.Name, Path: path, Configured: configured, Flatpak: flatpak}
	if b.Name == "" {
		b.Name = filepath.Base(folder)
	}
	for key, p := range spec.Programs {
		if p.Removed || strings.TrimSpace(p.Name) == "" {
			continue
		}
		id := p.ID
		if id == "" {
			id = key
		}
		b.Programs = append(b.Programs, Program{ID: id, Name: p.Name, Exe: programPath(home, path, p.Path, flatpak)})
	}
	slices.SortFunc(b.Programs, func(a, c Program) int {
		if n := strings.Compare(strings.ToLower(a.Name), strings.ToLower(c.Name)); n != 0 {
			return n
		}
		return strings.Compare(a.ID, c.ID)
	})
	return b, true
}

// programPath finds a program's file on this device: Bottles stores either a
// Linux path or a Windows one inside the bottle, such as C:\Games\a.exe.
func programPath(home, bottle, p string, flatpak bool) string {
	p = strings.TrimSpace(p)
	if len(p) < 3 || p[1] != ':' || p[2] != '\\' {
		return hostPath(p, home, flatpak)
	}
	drive := strings.ToLower(p[:1])
	if drive[0] < 'a' || drive[0] > 'z' {
		return ""
	}
	rest := filepath.FromSlash(strings.ReplaceAll(p[3:], `\`, "/"))
	if drive == "c" {
		return filepath.Join(bottle, "drive_c", rest)
	}
	// Other drive letters are links in the prefix (z: is usually "/").
	root, err := filepath.EvalSymlinks(filepath.Join(bottle, "dosdevices", drive+":"))
	if err != nil {
		return ""
	}
	return filepath.Join(root, rest)
}

// game describes exe, started in b; p is the program added for it, or nil.
func game(b Bottle, p *Program, exe string) Game {
	g := Game{Bottle: b.Name, Exe: exe, Prefix: b.Path, Flatpak: b.Flatpak, Name: b.Name}
	if p == nil {
		return g
	}
	g.Name, g.ProgramID = p.Name, p.ID
	// Bottles starts the first program of a name: only a name that is its own
	// says which one is meant.
	n := 0
	for _, o := range b.Programs {
		if o.Name == p.Name {
			n++
		}
	}
	if n == 1 {
		g.Program = p.Name
	}
	return g
}

// Games lists the Windows programs added to the bottles, which Bottles can
// start and Carryover can wait for.
func Games(home string) []Game {
	var out []Game
	for _, b := range List(home) {
		for _, p := range b.Programs {
			if b.Configured && strings.EqualFold(filepath.Ext(p.Exe), ".exe") {
				out = append(out, game(b, &p, p.Exe))
			}
		}
	}
	return out
}

// ForExe returns how Bottles starts exe: as the program added for it or, when
// there is none, by its path in the bottle that holds it (in its folder, or
// next to one of its programs; the deepest match wins).
func ForExe(home, exe string) (Game, bool) {
	exe = filepath.Clean(exe)
	var best Game
	depth := -1
	for _, b := range List(home) {
		if !b.Configured {
			continue
		}
		dirs := []string{b.Path}
		for _, p := range b.Programs {
			if p.Exe == exe {
				return game(b, &p, exe), true
			}
			// A program on another drive: its folder is the game's. Not one of
			// the bottle's own, where it would only repeat the bottle's folder.
			if p.Exe != "" && !inDir(p.Exe, b.Path) {
				dirs = append(dirs, filepath.Dir(p.Exe))
			}
		}
		for _, dir := range dirs {
			if inDir(exe, dir) && len(dir) > depth {
				best, depth = game(b, nil, exe), len(dir)
			}
		}
	}
	return best, depth >= 0
}

// lookPath finds a program on PATH; tests replace it.
var lookPath = exec.LookPath

// nativePaths are where packages put the native bottles-cli, besides PATH.
var nativePaths = []string{"/usr/bin/bottles-cli", "/usr/local/bin/bottles-cli"}

// Command returns the command that starts the game through Bottles.
func (g Game) Command() []string {
	run := []string{"run", "-b", g.Bottle}
	if g.Program != "" {
		run = append(run, "-p", g.Program)
	} else {
		run = append(run, "-e", g.Exe)
	}
	if g.Flatpak {
		return append([]string{"flatpak", "run", "--command=bottles-cli", FlatpakID}, run...)
	}
	cli := "bottles-cli" // not found: starting it says so
	if p, err := lookPath(cli); err == nil {
		cli = p
	} else {
		for _, p := range nativePaths {
			if _, err := os.Stat(p); err == nil {
				cli = p
				break
			}
		}
	}
	return append([]string{cli}, run...)
}

// systemFlatpaks is where Flatpak installs apps for every user; tests
// replace it.
var systemFlatpaks = "/var/lib/flatpak/app"

// Present reports whether Bottles is installed for the user with this home
// folder: its data folder exists (Bottles makes it on first start), the native
// bottles-cli is found, or its Flatpak is installed.
func Present(home string) bool {
	if home != "" && slices.ContainsFunc([]string{
		filepath.Join(home, ".local", "share", "bottles"),
		filepath.Join(home, ".var", "app", FlatpakID),
		filepath.Join(home, ".local", "share", "flatpak", "app", FlatpakID),
	}, isDir) {
		return true
	}
	if isDir(filepath.Join(systemFlatpaks, FlatpakID)) {
		return true
	}
	if _, err := lookPath("bottles-cli"); err == nil {
		return true
	}
	return slices.ContainsFunc(nativePaths, func(p string) bool {
		_, err := os.Stat(p)
		return err == nil
	})
}

// readYAML reads path into v; false when there is no such file or it isn't YAML.
func readYAML(path string, v any) bool {
	b, err := os.ReadFile(path)
	return err == nil && yaml.Unmarshal(b, v) == nil
}

func isDir(p string) bool {
	st, err := os.Stat(p)
	return err == nil && st.IsDir()
}

// inDir reports whether p is inside dir.
func inDir(p, dir string) bool { return strings.HasPrefix(p, dir+string(filepath.Separator)) }
