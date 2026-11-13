package core

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/akfaiz/carryover/internal/cloudsync/syncer"
	"github.com/akfaiz/carryover/internal/launcher"
	"github.com/akfaiz/carryover/internal/library"
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
	h.Status("sync", "Checking")
	assert.NotEmpty(t, lines, "status is logged")
	HeadlessUI{}.Status("sync", "no logger is fine")
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
	c.markSession(id)
	assert.FileExists(t, c.sessionMarkerPath(id))
	c.clearSession(id)
	assert.NoFileExists(t, c.sessionMarkerPath(id))

	// Make the marker's place a directory, so writing the file fails.
	require.NoError(t, os.MkdirAll(c.sessionMarkerPath(id), 0o755))
	assert.NotPanics(t, func() { c.markSession(id) })
}

// Forgetting a key that has nothing recorded is harmless, and repeated calls too.
func TestForgetLocalIsIdempotent(t *testing.T) {
	c, id, _, _ := newPlayable(t)
	c.forgetLocal(id)
	c.forgetLocal(id)
	c.forgetLocal("never-existed")
	assert.NoError(t, c.PurgeSaves(context.Background(), []string{"never-existed"}))
}
