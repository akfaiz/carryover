// Package flatpak finds the apps installed with Flatpak (Linux) and recognises
// the commands that start one.
package flatpak

import (
	"os"
	"path"
	"path/filepath"
	"strings"
)

// SystemDir is the installation Flatpak uses for every user; tests replace it.
var SystemDir = "/var/lib/flatpak"

// App is an installed Flatpak app.
type App struct {
	ID string // e.g. "org.DolphinEmu.dolphin-emu"
	// Launcher is the script Flatpak exports to start the app: it runs
	// "flatpak run <id>" with the arguments it is given.
	Launcher string
}

// installations are the user's own Flatpak installation and the system's.
func installations(home string) []string {
	var out []string
	if home != "" {
		out = append(out, filepath.Join(home, ".local", "share", "flatpak"))
	}
	return append(out, SystemDir)
}

// Apps lists the installed apps that export a launcher, the user's
// installation first; an app installed in both is listed once (the user's,
// which "flatpak run" prefers).
func Apps(home string) []App {
	var out []App
	seen := map[string]bool{}
	for _, inst := range installations(home) {
		entries, err := os.ReadDir(filepath.Join(inst, "app"))
		if err != nil {
			continue
		}
		for _, e := range entries {
			id := e.Name()
			if !e.IsDir() || seen[id] || !ValidID(id) {
				continue
			}
			launcher := filepath.Join(inst, "exports", "bin", id)
			if fi, err := os.Stat(launcher); err != nil || fi.IsDir() {
				continue
			}
			seen[id] = true
			out = append(out, App{ID: id, Launcher: launcher})
		}
	}
	return out
}

// AppID returns the id of the Flatpak app a command starts, or "": exe is a
// launcher Flatpak exports (".../flatpak/exports/bin/<id>"), or flatpak itself
// with "run [options] <id>" in args.
func AppID(exe string, args []string) string {
	exe = strings.TrimSpace(strings.ReplaceAll(exe, `\`, "/"))
	dir, base := path.Split(exe)
	if strings.HasSuffix(dir, "/exports/bin/") && strings.Contains(dir, "flatpak") && ValidID(base) {
		return base
	}
	if base != "flatpak" {
		return ""
	}
	run := false
	for _, a := range args {
		switch {
		case !run:
			run = a == "run"
		case ValidID(a):
			return a // options before it ("--branch=stable", "--arch x86_64") are skipped
		}
	}
	return ""
}

// ValidID reports whether s has the form of a Flatpak app id: at least three
// dot-separated parts of letters, digits, "_" and "-".
func ValidID(s string) bool {
	parts := strings.Split(s, ".")
	if len(parts) < 3 {
		return false
	}
	for _, p := range parts {
		if p == "" {
			return false
		}
		if strings.ContainsFunc(p, func(r rune) bool {
			return (r < 'a' || r > 'z') && (r < 'A' || r > 'Z') && (r < '0' || r > '9') && r != '_' && r != '-'
		}) {
			return false
		}
	}
	return true
}
