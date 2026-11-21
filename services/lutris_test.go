package services

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/akfaiz/carryover/internal/library"
	"github.com/akfaiz/carryover/internal/saves/ludusavi"
	"github.com/akfaiz/carryover/internal/stores/wineprefix"
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

// A game started through Lutris gets every location of the prefix Lutris runs
// it in, even when Lutris names the game differently and the executable is
// outside the prefix.
func TestSuggestSavesInTheLutrisPrefix(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Lutris is Linux only")
	}
	svc, _, _ := newLibEnv(t)
	home := t.TempDir()
	t.Setenv("HOME", home)
	prefix := filepath.Join(home, "Wine", "hk")
	user := filepath.Join(prefix, "drive_c", "users", "deck")
	require.NoError(t, os.MkdirAll(user, 0o755))
	exe := filepath.Join(home, "Games", "hk", "hollow_knight.exe")
	cfg := filepath.Join(home, ".local", "share", "lutris", "games", "hk-1.yml")
	require.NoError(t, os.MkdirAll(filepath.Dir(cfg), 0o755))
	require.NoError(t, os.WriteFile(cfg, []byte("name: HK (GOG)\ngame:\n  exe: '"+exe+"'\n  prefix: '"+prefix+"'\n"), 0o644))
	svc.prefixes = func() []wineprefix.Prefix {
		return []wineprefix.Prefix{{Launcher: wineprefix.Lutris, Name: "HK (GOG)", Path: prefix, Users: []string{user}}}
	}

	res, err := svc.SuggestSaves(SuggestInput{Name: "Hollow Knight", Exe: exe, LaunchMode: library.LaunchLutris})
	require.NoError(t, err)
	var own []SaveSuggestion
	for _, s := range res.Suggestions {
		if s.Kind == ludusavi.KindPrefix && s.Prefix == "Lutris: HK (GOG)" {
			own = append(own, s)
		}
	}
	require.NotEmpty(t, own, "the game's Lutris prefix, though empty and named differently")
	assert.Zero(t, own[0].Files)
	for _, s := range res.Suggestions {
		assert.NotEqual(t, ludusavi.KindProton, s.Kind, "a Lutris game never runs in Steam's Proton prefix")
	}
}
