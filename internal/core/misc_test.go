package core

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/akfaiz/carryover/internal/app/msg"
	"github.com/akfaiz/carryover/internal/cloudsync/syncer"
	"github.com/akfaiz/carryover/internal/launcher"
	"github.com/akfaiz/carryover/internal/library"
	"github.com/akfaiz/carryover/internal/stores/lutris"
)

func TestGameOf(t *testing.T) {
	assert.Equal(t, "g", GameOf("g"))
	assert.Equal(t, "g", GameOf("g.emu"))
	assert.Equal(t, "", GameOf(""))
}

// Progress events reach the handler that was set, and nothing breaks without one.
func TestEventHandler(t *testing.T) {
	c, _, _, _ := newPlayable(t)
	c.emit(syncer.Event{Message: "ignored"}) // no handler yet

	var got []syncer.Event
	c.SetEventHandler(func(ev syncer.Event) { got = append(got, ev) })
	c.emit(syncer.Event{GameID: "g", Message: "Uploading"})
	require.Len(t, got, 1)
	assert.Equal(t, "Uploading", got[0].Message)

	c.SetEventHandler(nil)
	c.emit(syncer.Event{Message: "after"})
	assert.Len(t, got, 1, "a removed handler hears nothing")
}

// The headless UI answers every question so a script never blocks.
func TestHeadlessUIAnswers(t *testing.T) {
	var lines []string
	h := HeadlessUI{Logf: func(f string, a ...any) { lines = append(lines, f) }}
	h.GameRunning(true)
	h.GameRunning(false)
	h.SyncCancelable(func() {})
	h.SyncCancelable(nil)
	h.Status("sync", msg.Raw("Checking"))
	assert.NotEmpty(t, lines, "status is logged")
	HeadlessUI{}.Status("sync", msg.Raw("no logger is fine"))
}

// A game that Heroic didn't install can't be started through it.
func TestHeroicSpecRefusesAnExeHeroicDoesNotKnow(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("USERPROFILE", t.TempDir())
	_, err := heroicSpec(launcher.Spec{}, filepath.Join(t.TempDir(), "game.exe"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not in a game folder Heroic")
}

func TestArtFileOnlyReturnsStoredImages(t *testing.T) {
	c, _, _, _ := newPlayable(t)
	require.NoError(t, os.MkdirAll(c.Dirs.CoversDir(), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(c.Dirs.CoversDir(), "hero.png"), []byte("x"), 0o644))

	g := library.Game{Art: map[string]string{"hero": "hero.png", "logo": "missing.png"}}
	assert.Equal(t, filepath.Join(c.Dirs.CoversDir(), "hero.png"), c.artFile(g, "hero"))
	assert.Empty(t, c.artFile(g, "logo"), "named, but not on this device")
	assert.Empty(t, c.artFile(g, "icon"), "no such kind")
	assert.Empty(t, c.artFile(library.Game{}, "hero"))
}

// A session marker is only a best-effort note: failing to write it is logged,
// not fatal.
func TestMarkSessionIsBestEffort(t *testing.T) {
	c, id, _, _ := newPlayable(t)
	c.markSession(id, id, "")
	assert.FileExists(t, c.sessionMarkerPath(id))
	c.clearSession(id)
	assert.NoFileExists(t, c.sessionMarkerPath(id))

	// Make the marker's place a directory, so writing the file fails.
	require.NoError(t, os.MkdirAll(c.sessionMarkerPath(id), 0o755))
	assert.NotPanics(t, func() { c.markSession(id, id, "") })
}

// Forgetting a key that has nothing recorded is harmless, and repeated calls too.
func TestForgetLocalIsIdempotent(t *testing.T) {
	c, id, _, _ := newPlayable(t)
	c.forgetLocal(id)
	c.forgetLocal(id)
	c.forgetLocal("never-existed")
	assert.NoError(t, c.PurgeSaves(context.Background(), []string{"never-existed"}))
}

// Lutris is Linux only; elsewhere a Lutris game can't be started.
func TestLutrisSpecNeedsLinux(t *testing.T) {
	if runtime.GOOS == "linux" {
		t.Skip("Linux has Lutris")
	}
	_, err := lutrisSpec(launcher.Spec{}, "/games/a.exe")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "only possible on Linux")
}

// A game that Lutris didn't install can't be started through it.
func TestLutrisSpecRefusesAnExeLutrisDoesNotKnow(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Lutris is Linux only")
	}
	t.Setenv("HOME", t.TempDir())
	_, err := lutrisSpec(launcher.Spec{}, filepath.Join(t.TempDir(), "game.exe"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not in a game Lutris")
}

// A Lutris game starts through lutris:rungame/<slug> and is waited for by its
// executable's name, unless the device says otherwise.
func TestLutrisSpecStartsTheGameThroughLutris(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Lutris is Linux only")
	}
	home := t.TempDir()
	t.Setenv("HOME", home)
	exe := filepath.Join(home, "Games", "hades", "Hades.exe")
	cfg := filepath.Join(home, ".var", "app", lutris.FlatpakID, "data", "lutris", "games", "hades-1710000000.yml")
	require.NoError(t, os.MkdirAll(filepath.Dir(cfg), 0o755))
	require.NoError(t, os.WriteFile(cfg, []byte("name: Hades\nslug: hades\ngame_slug: hades\ngame:\n  exe: '"+exe+"'\n"), 0o644))

	spec, err := lutrisSpec(launcher.Spec{}, exe)
	require.NoError(t, err)
	assert.Equal(t, "flatpak", spec.Exe)
	assert.Equal(t, []string{"run", lutris.FlatpakID, "lutris:rungame/hades"}, spec.Args)
	assert.Equal(t, "Hades.exe", spec.WaitProcess)
	assert.True(t, spec.Detach)
	assert.Equal(t, launcherAppear, spec.WaitAppear)
	assert.Equal(t, home, spec.WorkDir)

	spec, err = lutrisSpec(launcher.Spec{WaitProcess: "Real.exe"}, exe)
	require.NoError(t, err)
	assert.Equal(t, "Real.exe", spec.WaitProcess)
}
