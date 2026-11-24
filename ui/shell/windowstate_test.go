package shell

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWindowStatesRoundTrip(t *testing.T) {
	dir := t.TempDir()
	s := newWindowStates(dir)

	got, err := s.LoadWindowStates()
	require.NoError(t, err)
	assert.Empty(t, got, "no file, no states")

	in := map[string]mygo.WindowState{mainWindowKey: {X: 10, Y: 20, Width: 1000, Height: 700, Maximized: true}}
	require.NoError(t, s.SaveWindowStates(in))

	got, err = newWindowStates(dir).LoadWindowStates()
	require.NoError(t, err)
	assert.Equal(t, in, got)
}

func TestWindowStatesDropFullScreen(t *testing.T) {
	s := newWindowStates(t.TempDir())
	require.NoError(t, s.SaveWindowStates(map[string]mygo.WindowState{mainWindowKey: {Width: 900, Height: 600, FullScreen: true}}))

	got, err := s.LoadWindowStates()
	require.NoError(t, err)
	assert.False(t, got[mainWindowKey].FullScreen, "applyFullScreen alone decides full screen")
	assert.Equal(t, 900, got[mainWindowKey].Width)
}

func TestWindowStatesDamagedFile(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "window-state.json"), []byte("{not json"), 0o644))

	got, err := newWindowStates(dir).LoadWindowStates()
	require.NoError(t, err)
	assert.Empty(t, got)
}
