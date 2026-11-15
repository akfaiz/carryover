package core

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/akfaiz/carryover/internal/library"
	"github.com/akfaiz/carryover/internal/stores/steam"
)

func TestIsSelfShortcut(t *testing.T) {
	match := isSelfShortcut("/opt/test/app.bin")
	tests := []struct {
		exe, opts string
		want      bool
	}{
		{`"/opt/test/app.bin"`, ``, true},
		{`"/home/deck/Applications/Carryover-x86_64.AppImage"`, ``, true}, // moved or renamed
		{`"C:\Tools\carryover.exe"`, ``, true},
		{`"/opt/test/app.bin"`, `run g1`, false},                   // a game's shortcut
		{`"C:\Tools\carryover.exe"`, `run g1 --emulator e`, false}, // an emulator's
		{`"/usr/games/celeste"`, ``, false},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.want, match(steam.Shortcut{Exe: tt.exe, LaunchOptions: tt.opts}), "%s %s", tt.exe, tt.opts)
	}
}

// Carryover's own shortcut gets its artwork, is updated in place, and is
// removed without touching the shortcuts of games.
func TestAddSelfToSteam(t *testing.T) {
	c, id, _, _ := newPlayable(t)
	root := steamEnv(t)
	const user = "12345"
	_, err := c.AddToSteam(id, user, library.LaunchDirect)
	require.NoError(t, err)

	got, err := c.SelfSteamAppID(user)
	require.NoError(t, err)
	assert.Empty(t, got, "not in Steam yet")

	art := map[string][]byte{}
	for _, k := range SelfArtKinds {
		art[k] = []byte("png:" + k)
	}
	res, err := c.AddSelfToSteam(user, art)
	require.NoError(t, err)
	require.NotEmpty(t, res.AppID)

	list, err := steam.ListShortcuts(root, user)
	require.NoError(t, err)
	require.Len(t, list, 2)
	self := list[1]
	assert.Equal(t, "Carryover", self.AppName)
	assert.Empty(t, self.LaunchOptions)
	grid := steam.GridDir(root, user)
	assert.Equal(t, filepath.Join(grid, res.AppID+"_icon.png"), self.Icon)
	for file, want := range map[string]string{
		"p.png": "cover", ".png": "wide", "_hero.png": "hero", "_logo.png": "logo", "_icon.png": "icon",
	} {
		b, err := os.ReadFile(filepath.Join(grid, res.AppID+file))
		require.NoError(t, err, file)
		assert.Equal(t, "png:"+want, string(b), file)
	}

	again, err := c.AddSelfToSteam(user, nil)
	require.NoError(t, err)
	assert.Equal(t, res.AppID, again.AppID, "updating keeps the app id")
	list, _ = steam.ListShortcuts(root, user)
	assert.Len(t, list, 2, "updated, not added twice")
	got, err = c.SelfSteamAppID(user)
	require.NoError(t, err)
	assert.Equal(t, res.AppID, got)

	n, err := c.RemoveSelfFromSteam(user)
	require.NoError(t, err)
	assert.Equal(t, 1, n)
	list, _ = steam.ListShortcuts(root, user)
	require.Len(t, list, 1)
	assert.Equal(t, "run "+id, list[0].LaunchOptions, "the game's shortcut stays")
}

func TestAddSelfToSteamErrors(t *testing.T) {
	c, _, _, _ := newPlayable(t)
	steamEnv(t)
	_, err := c.AddSelfToSteam("", nil)
	assert.Error(t, err, "no Steam account")

	c2, _, _, _ := newPlayable(t)
	t.Setenv("CARRYOVER_STEAM_ROOT", "")
	c2.steamOnce.Do(func() { c2.steamRoot = "" })
	_, err = c2.AddSelfToSteam("1", nil)
	assert.Error(t, err, "no Steam")
	_, err = c2.RemoveSelfFromSteam("1")
	assert.Error(t, err, "no Steam")
}
