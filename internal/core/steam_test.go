package core

import (
	"context"
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
