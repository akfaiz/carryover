package core

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/akfaiz/carryover/internal/library"
	"github.com/akfaiz/carryover/internal/steam"
	"github.com/akfaiz/carryover/internal/syncer"
)

func TestQuote(t *testing.T) {
	require.Equal(t, `"C:\Program Files\carryover.exe"`, quote(`C:\Program Files\carryover.exe`))
}

func TestSteamWrapLaunchOptions(t *testing.T) {
	got, err := SteamWrapLaunchOptions("abc-123")
	require.NoError(t, err)
	require.True(t, strings.HasPrefix(got, `"`), "launch options = %s", got)
	require.True(t, strings.HasSuffix(got, ` run abc-123 -- %command%`), "launch options = %s", got)
}

func TestShortcutMatcher(t *testing.T) {
	match := shortcutMatcher("g1")
	tests := []struct {
		opts string
		want bool
	}{
		{`"/opt/carryover" run g1 -- %command%`, true},
		{`run g1`, true},
		{`"C:\carryover.exe" run g1`, true},
		{`"/opt/carryover" run g10 -- %command%`, true}, // prefix match: ids are UUIDs, so this never collides
		{`"/opt/carryover" run other`, false},
		{``, false},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.want, match(steam.Shortcut{LaunchOptions: tt.opts}), "match(%q)", tt.opts)
	}
}

func TestLabelOr(t *testing.T) {
	require.Equal(t, "Main saves", labelOr(library.SaveSlot{Key: "saves", Label: "Main saves"}))
	require.Equal(t, "saves", labelOr(library.SaveSlot{Key: "saves"}), "labelOr without label")
}

// HeadlessUI never blocks: conflicts are skipped, play continues despite a
// lock or a sync failure, and every message is logged.
func TestHeadlessUI(t *testing.T) {
	var logged []string
	ui := HeadlessUI{Logf: func(f string, a ...any) { logged = append(logged, f) }}
	g := library.Game{Name: "Game A"}
	ctx := context.Background()

	ui.Status("sync", "Checking")
	require.Equal(t, syncer.ResolveAuto, ui.ResolveConflict(ctx, g, nil, true))
	require.True(t, ui.ConfirmLock(ctx, g, &syncer.Lock{DeviceName: "Deck", Since: time.Now()}), "ConfirmLock refused")
	require.True(t, ui.ConfirmSyncFailure(ctx, g, context.DeadlineExceeded, false), "ConfirmSyncFailure refused")
	ui.GameRunning(true)
	ui.SyncCancelable(func() {})
	require.Len(t, logged, 4)

	// Without a logger nothing panics.
	HeadlessUI{}.Status("x", "y")
}

// steamEnv points Carryover at an empty fake Steam installation.
func steamEnv(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	t.Setenv("CARRYOVER_STEAM_ROOT", root)
	return root
}

func TestAddToSteamModes(t *testing.T) {
	c, id, saves, _ := newPlayable(t)
	root := steamEnv(t)
	const user = "12345"

	// A direct shortcut: Steam runs Carryover itself.
	res, err := c.AddToSteam(id, user, library.LaunchDirect)
	require.NoError(t, err, "AddToSteam direct")
	require.NotEmpty(t, res.AppID)
	list, err := steam.ListShortcuts(root, user)
	require.NoError(t, err)
	require.Len(t, list, 1)
	require.Equal(t, "run "+id, list[0].LaunchOptions)
	require.Equal(t, "Game A", list[0].AppName)
	g := mustFind(t, c, id)
	dev, _ := g.Device(c.DeviceID())
	require.Equal(t, res.AppID, dev.SteamAppID)
	require.Equal(t, library.LaunchDirect, dev.LaunchMode)

	// A wrapped shortcut replaces it: Steam runs the game exe.
	_, err = c.Library.UpdateDevice(id, c.DeviceID(), func(d *library.DeviceConfig) { d.Args = "-windowed" })
	require.NoError(t, err)
	_, err = c.AddToSteam(id, user, library.LaunchSteamWrap)
	require.NoError(t, err)
	list, _ = steam.ListShortcuts(root, user)
	require.Len(t, list, 1)
	require.True(t, strings.HasSuffix(list[0].LaunchOptions, " run "+id+" -- %command% -windowed"))
	require.Contains(t, list[0].StartDir, filepath.Base(saves))

	// Removing clears the shortcut and the recorded app id.
	n, err := c.RemoveFromSteam(id, user)
	require.NoError(t, err)
	require.Equal(t, 1, n)
	list, _ = steam.ListShortcuts(root, user)
	require.Empty(t, list, "shortcut left behind")
	g = mustFind(t, c, id)
	dev, _ = g.Device(c.DeviceID())
	require.Empty(t, dev.SteamAppID, "SteamAppID after removal")
}

func TestAddToSteamErrors(t *testing.T) {
	c, id, _, _ := newPlayable(t)
	t.Setenv("CARRYOVER_STEAM_ROOT", "")
	// No Steam found (the real machine may have one, so force "none").
	c.steamOnce.Do(func() { c.steamRoot = "" })
	_, err := c.AddToSteam(id, "1", library.LaunchDirect)
	assert.Error(t, err, "AddToSteam without Steam")
	_, err = c.RemoveFromSteam(id, "1")
	assert.Error(t, err, "RemoveFromSteam without Steam")

	c2, id2, _, _ := newPlayable(t)
	steamEnv(t)
	_, err = c2.AddToSteam("no-such-game", "1", library.LaunchDirect)
	assert.Error(t, err, "unknown game")
	// A game with no executable on this device can't be wrapped or run via Heroic.
	_, err = c2.Library.UpdateDevice(id2, c2.DeviceID(), func(d *library.DeviceConfig) { d.Exe = "" })
	require.NoError(t, err)
	for _, mode := range []library.LaunchMode{library.LaunchSteamWrap, library.LaunchHeroic} {
		_, err := c2.AddToSteam(id2, "1", mode)
		assert.Error(t, err, "mode %q without an executable", mode)
	}
}
