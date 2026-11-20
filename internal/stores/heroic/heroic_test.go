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

// Each store says which executable starts the game: GOG in goggame-<id>.info
// (Windows builds only), Epic in Legendary's installed.json, Amazon in
// fuel.json, and a game added by hand is its executable. Only files that
// exist count.
func TestInstalledExe(t *testing.T) {
	home := t.TempDir()
	j := func(parts ...string) string { return filepath.Join(append([]string{home}, parts...)...) }
	t.Setenv("APPDATA", j("AppData", "Roaming"))
	cfg := ConfigDirs(home)[0].Path

	hades, native, missing := j("G", "Hades"), j("G", "Native"), j("G", "Missing")
	writeFile(t, filepath.Join(hades, "x64", "Hades.exe"), "")
	writeFile(t, filepath.Join(hades, "goggame-1.info"),
		"\xef\xbb\xbf"+`{"playTasks":[{"category":"tool","path":"Setup.exe"},{"isPrimary":true,"category":"game","path":"x64\\Hades.exe"}]}`)
	writeFile(t, filepath.Join(native, "start.sh"), "")
	writeFile(t, filepath.Join(native, "goggame-2.info"), `{"playTasks":[{"isPrimary":true,"path":"start.sh"}]}`)
	writeFile(t, filepath.Join(missing, "goggame-3.info"), `{"playTasks":[{"isPrimary":true,"path":"Gone.exe"}]}`)
	writeFile(t, filepath.Join(cfg, "gog_store", "installed.json"), `{"installed":[`+
		`{"appName":"1","install_path":"`+js(hades)+`","platform":"windows"},`+
		`{"appName":"2","install_path":"`+js(native)+`","platform":"linux"},`+
		`{"appName":"3","install_path":"`+js(missing)+`","platform":"windows"}]}`)

	celeste := j("G", "Celeste")
	writeFile(t, filepath.Join(celeste, "Bin", "Celeste.exe"), "")
	writeFile(t, filepath.Join(cfg, "legendaryConfig", "legendary", "installed.json"),
		`{"Salt":{"app_name":"Salt","install_path":"`+js(celeste)+`","executable":"Bin/Celeste.exe"}}`)

	tunic := j("G", "Tunic")
	writeFile(t, filepath.Join(tunic, "Tunic.exe"), "")
	writeFile(t, filepath.Join(tunic, "fuel.json"), `{"Main":{"Command":"Tunic.exe"}}`)
	writeFile(t, filepath.Join(cfg, "nile_config", "nile", "installed.json"), `[{"id":"amzn1.tunic","path":"`+js(tunic)+`"}]`)

	side := j("G", "Side", "Side.exe")
	writeFile(t, side, "")
	writeFile(t, filepath.Join(cfg, "sideload_apps", "library.json"),
		`{"games":[{"app_name":"abc","install":{"executable":"`+js(side)+`"}}]}`)

	exes := map[string]string{}
	for _, g := range Installed(home) {
		exes[g.AppName] = g.Exe
	}
	assert.Equal(t, map[string]string{
		"1":           filepath.Join(hades, "x64", "Hades.exe"),
		"2":           "",
		"3":           "",
		"Salt":        filepath.Join(celeste, "Bin", "Celeste.exe"),
		"amzn1.tunic": filepath.Join(tunic, "Tunic.exe"),
		"abc":         side,
	}, exes)
}
