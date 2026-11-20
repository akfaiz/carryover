package services

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/akfaiz/carryover/internal/library"
)

// The Lutris library lists the games Lutris can start (those with an
// executable), sorted by name, and links the ones the library already has.
func TestLutrisLibrary(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Lutris is Linux only")
	}
	svc, c, id := newLibEnv(t)
	home := t.TempDir()
	t.Setenv("HOME", home)
	games := filepath.Join(home, ".local", "share", "lutris", "games")
	require.NoError(t, os.MkdirAll(games, 0o755))
	hades := filepath.Join(home, "Games", "hades", "Hades.exe")
	celeste := filepath.Join(home, "Games", "celeste", "Celeste.exe")
	write := func(name, body string) {
		require.NoError(t, os.WriteFile(filepath.Join(games, name), []byte(body), 0o644))
	}
	write("hades-1.yml", "name: Hades\ngame:\n  exe: '"+hades+"'\n")
	write("celeste-2.yml", "name: Celeste\ngame:\n  exe: '"+celeste+"'\n")
	write("snes-3.yml", "name: Zelda\ngame:\n  main_file: /roms/z.sfc\n")

	_, err := c.Library.UpdateDevice(id, c.DeviceID(), func(d *library.DeviceConfig) { d.Exe = hades })
	require.NoError(t, err)

	got := svc.LutrisLibrary()
	require.Len(t, got, 2, "a game with no executable can't be started through Lutris")
	assert.Equal(t, "Celeste", got[0].Name)
	assert.Empty(t, got[0].GameID)
	assert.Equal(t, "Hades", got[1].Name)
	assert.Equal(t, id, got[1].GameID, "the library already has this game")

	title, ok := svc.LutrisGame(filepath.Join(home, "Games", "celeste", "Launcher.exe"))
	assert.True(t, ok)
	assert.Equal(t, "Celeste", title)
	_, ok = svc.LutrisGame(filepath.Join(home, "elsewhere", "a.exe"))
	assert.False(t, ok)
}

func TestLutrisLibraryIsEmptyOffLinux(t *testing.T) {
	if runtime.GOOS == "linux" {
		t.Skip("Linux has Lutris")
	}
	svc, _, _ := newLibEnv(t)
	assert.Empty(t, svc.LutrisLibrary())
	_, ok := svc.LutrisGame("/games/a.exe")
	assert.False(t, ok)
}
