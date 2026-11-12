//go:build windows

package heroic

import (
	"os"
	"path/filepath"
	"strings"
)

func configDirs(home string) []ConfigDir {
	appData := os.Getenv("APPDATA")
	if appData == "" {
		appData = filepath.Join(home, "AppData", "Roaming")
	}
	return []ConfigDir{{Path: filepath.Join(appData, "heroic")}}
}

// executables are where Heroic's installer puts it: for all users, or per
// user.
func executables() []string {
	var out []string
	for _, env := range []string{"ProgramFiles", "ProgramW6432"} {
		if d := os.Getenv(env); d != "" {
			out = append(out, filepath.Join(d, "Heroic", "Heroic.exe"))
		}
	}
	if d := os.Getenv("LOCALAPPDATA"); d != "" {
		out = append(out, filepath.Join(d, "Programs", "heroic", "Heroic.exe"))
	}
	return out
}

func launchArgs(url string) []string { return []string{"--no-gui", url} }

// openURL opens the link with the registered heroic:// handler.
func openURL(url string) []string {
	return []string{"rundll32", "url.dll,FileProtocolHandler", url}
}

// inDir reports whether p is inside dir; Windows paths ignore case.
func inDir(p, dir string) bool {
	return strings.HasPrefix(strings.ToLower(p), strings.ToLower(dir)+string(filepath.Separator))
}
