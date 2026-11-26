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
)

// The Bottles library lists the programs of the bottles, sorted by name, and
// links the ones the library already has.
func TestBottlesLibrary(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Bottles is Linux only")
	}
	svc, c, id := newLibEnv(t)
	home := t.TempDir()
	t.Setenv("HOME", home)
	bottle := filepath.Join(home, ".local", "share", "bottles", "bottles", "Games")
	require.NoError(t, os.MkdirAll(filepath.Join(bottle, "drive_c"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(bottle, "bottle.yml"), []byte(`Name: Games
External_Programs:
  h: {id: h, name: Hades, path: 'C:\Hades\Hades.exe'}
  c: {id: c, name: Celeste, path: 'C:\Celeste\Celeste.exe'}
`), 0o644))
	hades := filepath.Join(bottle, "drive_c", "Hades", "Hades.exe")
	_, err := c.Library.UpdateDevice(id, c.DeviceID(), func(d *library.DeviceConfig) { d.Exe = hades })
	require.NoError(t, err)

	got := svc.BottlesLibrary()
	require.Len(t, got, 2)
	assert.Equal(t, "Celeste", got[0].Name)
	assert.Empty(t, got[0].GameID)
	assert.Equal(t, "Hades", got[1].Name)
	assert.Equal(t, "Games", got[1].Bottle)
	assert.Equal(t, id, got[1].GameID, "the library already has this game")

	name, ok := svc.BottlesGame(hades)
	assert.True(t, ok)
	assert.Equal(t, "Hades", name)
	name, ok = svc.BottlesGame(filepath.Join(bottle, "drive_c", "x.exe"))
	assert.True(t, ok)
	assert.Equal(t, "Games", name, "an executable that is no program goes by its bottle")
	_, ok = svc.BottlesGame(filepath.Join(home, "elsewhere", "a.exe"))
	assert.False(t, ok)
	assert.True(t, svc.Launchers().Bottles)

	// The game's bottle gets every location, named unlike the game, and the
	// game never runs in Steam's Proton prefix.
	require.NoError(t, os.MkdirAll(filepath.Join(bottle, "drive_c", "users", "deck"), 0o755))
	res, err := svc.SuggestSaves(SuggestInput{Name: "Hollow Knight", Exe: hades, LaunchMode: library.LaunchBottles})
	require.NoError(t, err)
	own := 0
	for _, s := range res.Suggestions {
		assert.NotEqual(t, ludusavi.KindProton, s.Kind)
		if s.Prefix == "Bottles: Games" {
			own++
		}
	}
	assert.Positive(t, own)
}

func TestBottlesLibraryIsEmptyOffLinux(t *testing.T) {
	if runtime.GOOS == "linux" {
		t.Skip("Linux has Bottles")
	}
	svc, _, _ := newLibEnv(t)
	assert.Empty(t, svc.BottlesLibrary())
	_, ok := svc.BottlesGame("/games/a.exe")
	assert.False(t, ok)
	assert.False(t, svc.Launchers().Bottles)
}
