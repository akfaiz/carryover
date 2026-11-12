//go:build !windows

package heroic

import (
	"path/filepath"
	"strings"
)

func configDirs(home string) []ConfigDir { return LinuxConfigDirs(home) }

// executables are the native Heroic builds, by name on PATH or where the
// .deb/.rpm/pacman packages put them.
func executables() []string { return []string{"heroic", "/opt/Heroic/heroic"} }

func launchArgs(url string) []string { return []string{"--no-gui", "--no-sandbox", url} }

func openURL(url string) []string { return []string{"xdg-open", url} }

// inDir reports whether p is inside dir.
func inDir(p, dir string) bool { return strings.HasPrefix(p, dir+string(filepath.Separator)) }
