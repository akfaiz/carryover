package services

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/akfaiz/carryover/internal/library"
)

// The Heroic library lists Heroic's games by name, with the executable its
// store gives, and links the ones the library already starts from their folder.
func TestHeroicLibrary(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("reads Heroic's Linux config folder")
	}
	svc, c, id := newLibEnv(t)
	home := t.TempDir()
	t.Setenv("HOME", home)
	cfg := filepath.Join(home, ".config", "heroic")
	write := func(p, body string) {
		require.NoError(t, os.MkdirAll(filepath.Dir(p), 0o755))
		require.NoError(t, os.WriteFile(p, []byte(body), 0o644))
	}
	hades := filepath.Join(home, "Games", "Heroic", "Hades")
	tunic := filepath.Join(home, "Games", "Heroic", "Tunic")
	write(filepath.Join(hades, "Hades.exe"), "")
	write(filepath.Join(hades, "goggame-1.info"), `{"playTasks":[{"isPrimary":true,"path":"Hades.exe"}]}`)
	write(filepath.Join(cfg, "gog_store", "installed.json"),
		`{"installed":[{"appName":"1","install_path":"`+hades+`","platform":"windows"}]}`)
	write(filepath.Join(cfg, "store_cache", "gog_library.json"), `{"games":[{"app_name":"1","title":"Hades"}]}`)
	write(filepath.Join(cfg, "nile_config", "nile", "installed.json"), `[{"id":"amzn1.tunic","path":"`+tunic+`"}]`)

	// The library starts Hades from another file in its folder.
	_, err := c.Library.UpdateDevice(id, c.DeviceID(), func(d *library.DeviceConfig) {
		d.Exe = filepath.Join(hades, "x64", "Launcher.exe")
	})
	require.NoError(t, err)

	got := svc.HeroicLibrary()
	require.Len(t, got, 2)
	names := []string{got[0].Name, got[1].Name}
	assert.Equal(t, []string{"amzn1.tunic", "Hades"}, names, "by name; no title means Heroic's id")
	h := got[1]
	assert.Equal(t, filepath.Join(hades, "Hades.exe"), h.Exe)
	assert.Equal(t, id, h.GameID, "the library already has this game")
	assert.Empty(t, got[0].GameID)
	assert.Empty(t, got[0].Exe, "no fuel.json, so no executable")
	assert.True(t, strings.HasPrefix(got[0].InstallPath, home))
}

func TestHeroicLibraryIsEmptyOnMac(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("Heroic games are listed on Linux and Windows")
	}
	svc, _, _ := newLibEnv(t)
	assert.Empty(t, svc.HeroicLibrary())
}

// Heroic and Lutris count as installed once their folders exist; on macOS
// neither can start games for Carryover.
func TestLaunchers(t *testing.T) {
	svc, _, _ := newLibEnv(t)
	home := t.TempDir()
	t.Setenv("HOME", home)
	if runtime.GOOS == "darwin" {
		require.NoError(t, os.MkdirAll(filepath.Join(home, ".config", "heroic"), 0o755))
		assert.Equal(t, Launchers{}, svc.Launchers())
		return
	}
	if runtime.GOOS != "linux" {
		t.Skip("reads the Linux folders")
	}
	require.NoError(t, os.MkdirAll(filepath.Join(home, ".config", "heroic"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(home, ".local", "share", "lutris"), 0o755))
	assert.Equal(t, Launchers{Heroic: true, Lutris: true}, svc.Launchers())
}
