package heroic

import (
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writeFile(t *testing.T, p, content string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(p), 0o755))
	require.NoError(t, os.WriteFile(p, []byte(content), 0o644))
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

	require.Len(t, Installed(home), 4)
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
		if assert.True(t, ok, "ForExe(%s)", c.exe) {
			assert.Equal(t, c.runner, g.Runner, c.exe)
			assert.Equal(t, c.app, g.AppName, c.exe)
			assert.Equal(t, c.title, g.Title, c.exe)
			assert.Equal(t, c.flatpak, g.Flatpak, c.exe)
		}
	}
	// A folder that only shares a name prefix is not the game's.
	_, ok := ForExe(home, j("Games", "Heroic", "Hades2", "Hades2.exe"))
	assert.False(t, ok, "ForExe matched a sibling folder")
}

func TestCommand(t *testing.T) {
	g := Game{Runner: GOG, AppName: "12 34", Flatpak: true}
	cmd := g.Command()
	want := []string{"flatpak", "run", FlatpakID, "--no-gui", "--no-sandbox"}
	require.Len(t, cmd, len(want)+1)
	assert.Equal(t, want, cmd[:len(want)])
	u, err := url.Parse(cmd[len(cmd)-1])
	require.NoError(t, err)
	assert.Equal(t, "heroic", u.Scheme)
	assert.Equal(t, "launch", u.Host)
	assert.Equal(t, "12 34", u.Query().Get("appName"))
	assert.Equal(t, GOG, u.Query().Get("runner"))
}
