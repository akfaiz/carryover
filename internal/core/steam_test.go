package core

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/akfaiz/carryover/internal/library"
	"github.com/akfaiz/carryover/internal/steam"
	"github.com/akfaiz/carryover/internal/syncer"
)

func TestQuote(t *testing.T) {
	if got := quote(`C:\Program Files\carryover.exe`); got != `"C:\Program Files\carryover.exe"` {
		t.Fatalf("quote = %s", got)
	}
}

func TestSteamWrapLaunchOptions(t *testing.T) {
	got, err := SteamWrapLaunchOptions("abc-123")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(got, `"`) || !strings.HasSuffix(got, ` run abc-123 -- %command%`) {
		t.Fatalf("launch options = %s", got)
	}
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
		if got := match(steam.Shortcut{LaunchOptions: tt.opts}); got != tt.want {
			t.Errorf("match(%q) = %v, want %v", tt.opts, got, tt.want)
		}
	}
}

func TestLabelOr(t *testing.T) {
	if got := labelOr(library.SaveSlot{Key: "saves", Label: "Main saves"}); got != "Main saves" {
		t.Fatalf("labelOr = %q", got)
	}
	if got := labelOr(library.SaveSlot{Key: "saves"}); got != "saves" {
		t.Fatalf("labelOr without label = %q", got)
	}
}

// HeadlessUI never blocks: conflicts are skipped, play continues despite a
// lock or a sync failure, and every message is logged.
func TestHeadlessUI(t *testing.T) {
	var logged []string
	ui := HeadlessUI{Logf: func(f string, a ...any) { logged = append(logged, f) }}
	g := library.Game{Name: "Game A"}
	ctx := context.Background()

	ui.Status("sync", "Checking")
	if res := ui.ResolveConflict(ctx, g, nil, true); res != syncer.ResolveAuto {
		t.Fatalf("ResolveConflict = %v, want ResolveAuto", res)
	}
	if !ui.ConfirmLock(ctx, g, &syncer.Lock{DeviceName: "Deck", Since: time.Now()}) {
		t.Fatal("ConfirmLock refused")
	}
	if !ui.ConfirmSyncFailure(ctx, g, context.DeadlineExceeded, false) {
		t.Fatal("ConfirmSyncFailure refused")
	}
	ui.GameRunning(true)
	ui.SyncCancelable(func() {})
	if len(logged) != 4 {
		t.Fatalf("logged %d lines, want 4: %q", len(logged), logged)
	}

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
	if err != nil || res.AppID == "" {
		t.Fatalf("AddToSteam direct = %+v, %v", res, err)
	}
	list, err := steam.ListShortcuts(root, user)
	if err != nil || len(list) != 1 {
		t.Fatalf("shortcuts = %+v, %v", list, err)
	}
	if list[0].LaunchOptions != "run "+id || list[0].AppName != "Game A" {
		t.Fatalf("direct shortcut = %+v", list[0])
	}
	g := mustFind(t, c, id)
	if dev, _ := g.Device(c.DeviceID()); dev.SteamAppID != res.AppID || dev.LaunchMode != library.LaunchDirect {
		t.Fatalf("device after AddToSteam = %+v", dev)
	}

	// A wrapped shortcut replaces it: Steam runs the game exe.
	if _, err := c.Library.UpdateDevice(id, c.DeviceID(), func(d *library.DeviceConfig) { d.Args = "-windowed" }); err != nil {
		t.Fatal(err)
	}
	if _, err := c.AddToSteam(id, user, library.LaunchSteamWrap); err != nil {
		t.Fatal(err)
	}
	list, _ = steam.ListShortcuts(root, user)
	if len(list) != 1 {
		t.Fatalf("a second AddToSteam must update, got %+v", list)
	}
	if !strings.HasSuffix(list[0].LaunchOptions, " run "+id+" -- %command% -windowed") {
		t.Fatalf("wrapped launch options = %q", list[0].LaunchOptions)
	}
	if !strings.Contains(list[0].StartDir, filepath.Base(saves)) {
		t.Fatalf("start dir = %q, want the work dir", list[0].StartDir)
	}

	// Removing clears the shortcut and the recorded app id.
	n, err := c.RemoveFromSteam(id, user)
	if err != nil || n != 1 {
		t.Fatalf("RemoveFromSteam = %d, %v", n, err)
	}
	if list, _ = steam.ListShortcuts(root, user); len(list) != 0 {
		t.Fatalf("shortcut left behind: %+v", list)
	}
	g = mustFind(t, c, id)
	if dev, _ := g.Device(c.DeviceID()); dev.SteamAppID != "" {
		t.Fatalf("SteamAppID = %q after removal", dev.SteamAppID)
	}
}

func TestAddToSteamErrors(t *testing.T) {
	c, id, _, _ := newPlayable(t)
	t.Setenv("CARRYOVER_STEAM_ROOT", "")
	// No Steam found (the real machine may have one, so force "none").
	c.steamOnce.Do(func() { c.steamRoot = "" })
	if _, err := c.AddToSteam(id, "1", library.LaunchDirect); err == nil {
		t.Error("AddToSteam without Steam: want an error")
	}
	if _, err := c.RemoveFromSteam(id, "1"); err == nil {
		t.Error("RemoveFromSteam without Steam: want an error")
	}

	c2, id2, _, _ := newPlayable(t)
	steamEnv(t)
	if _, err := c2.AddToSteam("no-such-game", "1", library.LaunchDirect); err == nil {
		t.Error("unknown game: want an error")
	}
	// A game with no executable on this device can't be wrapped or run via Heroic.
	if _, err := c2.Library.UpdateDevice(id2, c2.DeviceID(), func(d *library.DeviceConfig) { d.Exe = "" }); err != nil {
		t.Fatal(err)
	}
	for _, mode := range []library.LaunchMode{library.LaunchSteamWrap, library.LaunchHeroic} {
		if _, err := c2.AddToSteam(id2, "1", mode); err == nil {
			t.Errorf("mode %q without an executable: want an error", mode)
		}
	}
}
