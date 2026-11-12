package services

import (
	"path/filepath"
	"regexp"
	"strings"

	"github.com/akfaiz/carryover/internal/saves/emumanifest"
)

// EmuGuess is what Carryover could tell about an emulator from its executable.
type EmuGuess struct {
	Name   string `json:"name"`
	Preset string `json:"preset"` // emulator type id, "" if not recognised
}

var (
	emuExt = regexp.MustCompile(`(?i)\.(exe|bat|cmd|sh|appimage|x86_64|app|run)$`)
	// emuNoise are version, architecture and build words in release file names,
	// such as "Eden-Linux-x86_64-v0.0.3" or "yuzu-windows-msvc-early-access".
	emuNoise = regexp.MustCompile(`(?i)(^|[\s_.-]+)(v?\d[\d.]*|x86[_-]?64|x64|amd64|aarch64|arm64|win(dows|32|64)?|linux|mac(os)?|msvc|gcc|clang|qt\d*|appimage|portable|release|stable|nightly|early[\s_-]*access|ea|steamdeck|deck)(\b|$)`)
	// emuGeneric are executable names that don't name the emulator.
	emuGeneric = map[string]bool{
		"apprun": true, "launcher": true, "launch": true, "start": true, "run": true, "main": true,
		"emulator": true, "emu": true, "app": true, "qt": true, "gui": true, "bin": true,
	}
)

// GuessEmulator suggests a name and type for an emulator executable or
// AppImage picked on the Emulators page: a known emulator's proper name (from
// the file or the folders above it), otherwise the file's name without
// extension, version and platform words, or the folder's name if the file is
// generic.
func (s *EmulatorService) GuessEmulator(exe string) EmuGuess {
	exe = strings.TrimSpace(strings.ReplaceAll(exe, `\`, "/"))
	if exe == "" {
		return EmuGuess{}
	}
	if name, preset := emumanifest.Detect(exe); name != "" {
		return EmuGuess{Name: name, Preset: preset}
	}
	base := emuExt.ReplaceAllString(filepath.Base(filepath.FromSlash(exe)), "")
	name := cleanEmuName(base)
	if name == "" || emuGeneric[strings.ToLower(name)] {
		name = cleanEmuName(filepath.Base(filepath.Dir(filepath.FromSlash(exe))))
	}
	if emuGeneric[strings.ToLower(name)] {
		return EmuGuess{}
	}
	return EmuGuess{Name: name}
}

// cleanEmuName turns a file or folder name into a display name.
func cleanEmuName(s string) string {
	for {
		trimmed := emuNoise.ReplaceAllString(s, "")
		if trimmed == s || strings.TrimSpace(trimmed) == "" {
			break
		}
		s = trimmed
	}
	s = strings.NewReplacer("_", " ", "-", " ").Replace(s)
	s = strings.Join(strings.Fields(s), " ")
	if s == "." || s == "/" || s == `\` {
		return ""
	}
	return s
}
