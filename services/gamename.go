package services

import (
	"path/filepath"
	"regexp"
	"strings"

	"github.com/akfaiz/carryover/internal/ludusavi"
)

// GuessName suggests a game title for an executable picked in the editor:
// the exe name if the Ludusavi manifest knows it, otherwise the game's
// folder name (skipping folders like Binaries/Win64), using the manifest's
// spelling when the folder is a known game.
func (s *LibraryService) GuessName(exe string) string {
	return guessGameName(exe, func(name string) string {
		if g, err := ludusavi.Find("", name); err == nil && g != nil {
			return g.Name
		}
		return ""
	})
}

var (
	exeExt = regexp.MustCompile(`(?i)\.(exe|bat|cmd|lnk|sh|x86_64|x86|AppImage|app)$`)
	// Unreal's "<Game>-Win64-Shipping" and similar build suffixes.
	exeBuildSuffix = regexp.MustCompile(`(?i)[-_.]?(win(32|64)|x(64|86)|shipping|dx(11|12)|vulkan|steam|epic|gog|release|retail|final)$`)
)

// genericExe are executable names that say nothing about the game.
var genericExe = map[string]bool{
	"game": true, "launcher": true, "launch": true, "start": true, "play": true, "run": true,
	"main": true, "app": true, "client": true, "setup": true, "unityplayer": true, "runtime": true,
	"nw": true, "electron": true, "dosbox": true, "scummvm": true, "java": true, "javaw": true,
}

// genericDirs are folder names that aren't the game's name (build output,
// platform and store folders).
var genericDirs = map[string]bool{
	"bin": true, "bin32": true, "bin64": true, "binaries": true, "binary": true, "win32": true, "win64": true,
	"windows": true, "windowsnoeditor": true, "x64": true, "x86": true, "x86_64": true, "amd64": true,
	"release": true, "shipping": true, "retail": true, "game": true, "games": true, "app": true,
	"launcher": true, "common": true, "steamapps": true, "program files": true, "program files (x86)": true,
	"gog games": true, "epic games": true, "data": true, "linux": true, "macos": true, "content": true,
	"drive_c": true, "pfx": true, "dosbox": true,
}

// guessGameName implements GuessName; known returns the manifest title for
// a name, or "" if the manifest doesn't know it.
func guessGameName(exe string, known func(string) string) string {
	exe = strings.TrimSpace(exe)
	if exe == "" {
		return ""
	}
	exe = filepath.Clean(strings.ReplaceAll(exe, "\\", string(filepath.Separator)))
	stem := exeExt.ReplaceAllString(filepath.Base(exe), "")
	for {
		trimmed := exeBuildSuffix.ReplaceAllString(stem, "")
		if trimmed == stem || trimmed == "" {
			break
		}
		stem = trimmed
	}
	if !genericExe[strings.ToLower(stem)] {
		if title := known(stem); title != "" {
			return title
		}
	}

	folder := gameFolder(filepath.Dir(exe))
	if folder != "" {
		if title := known(folder); title != "" {
			return title
		}
		return folder
	}
	if genericExe[strings.ToLower(stem)] {
		return ""
	}
	return stem
}

// gameFolder walks up from dir to the first folder that isn't generic.
func gameFolder(dir string) string {
	for range 6 {
		name := filepath.Base(dir)
		parent := filepath.Dir(dir)
		if parent == dir || name == "" || name == "." || strings.HasSuffix(name, ":") || name == string(filepath.Separator) {
			return "" // reached the drive/filesystem root
		}
		if !genericDirs[strings.ToLower(name)] {
			return name
		}
		dir = parent
	}
	return ""
}
