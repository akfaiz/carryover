package heroic

import (
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeFile(t *testing.T, p, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// js writes a path into a JSON string.
func js(p string) string { return strings.ReplaceAll(p, `\`, `\\`) }

func TestInstalledAndForExe(t *testing.T) {
	home := t.TempDir()
	j := func(parts ...string) string { return filepath.Join(append([]string{home}, parts...)...) }
	t.Setenv("APPDATA", j("AppData", "Roaming")) // Windows
	dirs := ConfigDirs(home)
	native, flat := dirs[0].Path, dirs[len(dirs)-1].Path // the same folder on Windows
	flatpak := dirs[len(dirs)-1].Flatpak

	hades := j("Games", "Heroic", "Hades")
	writeFile(t, filepath.Join(flat, "gog_store", "installed.json"),
		`{"installed":[{"appName":"1234","install_path":"`+js(hades)+`","platform":"windows"}]}`)
	writeFile(t, filepath.Join(flat, "store_cache", "gog_library.json"),
		`{"games":[{"app_name":"1234","title":"Hades"}]}`)

	celeste := j("Games", "Heroic", "Celeste")
	writeFile(t, filepath.Join(native, "legendaryConfig", "legendary", "installed.json"),
		`{"Salt":{"app_name":"Salt","install_path":"`+js(celeste)+`","title":"Celeste"}}`)

	tunic := j("Games", "Tunic")
	writeFile(t, filepath.Join(native, "nile_config", "nile", "installed.json"),
		`[{"id":"amzn1.tunic","path":"`+js(tunic)+`"}]`)

	side := j("Games", "Side", "Game.exe")
	writeFile(t, filepath.Join(native, "sideload_apps", "library.json"),
		`{"games":[{"app_name":"abc","title":"Side","install":{"executable":"`+js(side)+`"}}]}`)

	if got := Installed(home); len(got) != 4 {
		t.Fatalf("Installed: got %d games %+v, want 4", len(got), got)
	}
	cases := []struct {
		exe, runner, app, title string
		flatpak                 bool
	}{
		{filepath.Join(hades, "x64", "Hades.exe"), GOG, "1234", "Hades", flatpak},
		{filepath.Join(celeste, "Celeste.exe"), Legendary, "Salt", "Celeste", false},
		{filepath.Join(tunic, "Tunic.exe"), Nile, "amzn1.tunic", "", false},
		{side, Sideload, "abc", "Side", false},
	}
	for _, c := range cases {
		g, ok := ForExe(home, c.exe)
		if !ok || g.Runner != c.runner || g.AppName != c.app || g.Title != c.title || g.Flatpak != c.flatpak {
			t.Errorf("ForExe(%s) = %+v, %v", c.exe, g, ok)
		}
	}
	// A folder that only shares a name prefix is not the game's.
	if g, ok := ForExe(home, j("Games", "Heroic", "Hades2", "Hades2.exe")); ok {
		t.Errorf("ForExe matched a sibling folder: %+v", g)
	}
}

func TestCommand(t *testing.T) {
	g := Game{Runner: GOG, AppName: "12 34", Flatpak: true}
	cmd := g.Command()
	want := []string{"flatpak", "run", FlatpakID, "--no-gui", "--no-sandbox"}
	if len(cmd) != len(want)+1 || strings.Join(cmd[:len(want)], " ") != strings.Join(want, " ") {
		t.Fatalf("Command() = %q", cmd)
	}
	u, err := url.Parse(cmd[len(cmd)-1])
	if err != nil || u.Scheme != "heroic" || u.Host != "launch" ||
		u.Query().Get("appName") != "12 34" || u.Query().Get("runner") != GOG {
		t.Errorf("launch URL = %q", cmd[len(cmd)-1])
	}
}
