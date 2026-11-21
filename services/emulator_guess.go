package services

import (
	"os"
	"path"
	"regexp"
	"runtime"
	"strings"

	"github.com/akfaiz/carryover/internal/launcher"
	"github.com/akfaiz/carryover/internal/saves/emumanifest"
	"github.com/akfaiz/carryover/internal/stores/flatpak"
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
	// emuDrive is a bare Windows drive such as "D:", what the folder of D:/run.exe is.
	emuDrive = regexp.MustCompile(`^[A-Za-z]:$`)
	// emuGeneric are executable names that don't name the emulator.
	emuGeneric = map[string]bool{
		"apprun": true, "launcher": true, "launch": true, "start": true, "run": true, "main": true,
		"emulator": true, "emu": true, "app": true, "qt": true, "gui": true, "bin": true,
		"flatpak": true,
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
	if id := flatpak.AppID(exe, nil); id != "" {
		return guessFlatpak(id)
	}
	if name, preset := emumanifest.Detect(exe); name != "" {
		return EmuGuess{Name: name, Preset: preset}
	}
	base := emuExt.ReplaceAllString(path.Base(exe), "")
	name := cleanEmuName(base)
	if name == "" || emuGeneric[strings.ToLower(name)] {
		name = cleanEmuName(path.Base(path.Dir(exe)))
	}
	if emuGeneric[strings.ToLower(name)] {
		return EmuGuess{}
	}
	return EmuGuess{Name: name}
}

// guessFlatpak names a Flatpak app by its id: a known emulator, otherwise the
// id's last part ("org.example.MyEmu" -> "MyEmu").
func guessFlatpak(id string) EmuGuess {
	if name, preset := emumanifest.Detect(id); name != "" && !flatpakNotEmulators[id] {
		return EmuGuess{Name: name, Preset: preset}
	}
	return EmuGuess{Name: cleanEmuName(id[strings.LastIndex(id, ".")+1:])}
}

// flatpakNotEmulators are apps whose id has an emulator's name in it.
var flatpakNotEmulators = map[string]bool{"org.kde.dolphin": true} // KDE's file manager

// flatpakID is the id of the Flatpak app an emulator command starts, "" if it
// isn't one.
func flatpakID(exe, args string) string {
	a, _ := launcher.SplitArgs(args)
	return flatpak.AppID(exe, a)
}

// FlatpakEmulator is an emulator installed with Flatpak on this device.
type FlatpakEmulator struct {
	AppID  string `json:"appId"`
	Name   string `json:"name"`
	Preset string `json:"preset"` // emulator type id
	// Exe is the launcher Flatpak exports for it, to use as the executable.
	Exe string `json:"exe"`
}

// Flatpaks lists the emulators of a known type installed with Flatpak (Linux).
func (s *EmulatorService) Flatpaks() []FlatpakEmulator {
	if runtime.GOOS != "linux" {
		return nil
	}
	home, _ := os.UserHomeDir()
	var out []FlatpakEmulator
	for _, a := range flatpak.Apps(home) {
		if g := guessFlatpak(a.ID); g.Preset != "" {
			out = append(out, FlatpakEmulator{AppID: a.ID, Name: g.Name, Preset: g.Preset, Exe: a.Launcher})
		}
	}
	return out
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
	if s == "." || s == "/" || s == `\` || emuDrive.MatchString(s) {
		return ""
	}
	return s
}
