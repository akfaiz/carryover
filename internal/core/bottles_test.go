package core

import (
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/akfaiz/carryover/internal/launcher"
	"github.com/akfaiz/carryover/internal/stores/bottles"
)

// Bottles is Linux only; elsewhere a Bottles game can't be started.
func TestBottlesSpecNeedsLinux(t *testing.T) {
	if runtime.GOOS == "linux" {
		t.Skip("Linux has Bottles")
	}
	_, err := bottlesSpec(launcher.Spec{}, "/games/a.exe")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "only possible on Linux")
}

// A program of a bottle is started by its name, any other executable of the
// bottle by its path, and the game is waited for by its executable's name.
func TestBottlesSpecStartsTheGameThroughBottles(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Bottles is Linux only")
	}
	home := t.TempDir()
	t.Setenv("HOME", home)
	bottle := filepath.Join(home, ".var", "app", bottles.FlatpakID, "data", "bottles", "bottles", "Games")
	require.NoError(t, os.MkdirAll(filepath.Join(bottle, "drive_c"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(bottle, "bottle.yml"),
		[]byte("Name: Games\nPath: Games\nExternal_Programs:\n  a:\n    id: a\n    name: Hades\n    path: 'C:\\Hades\\Hades.exe'\n"), 0o644))
	exe := filepath.Join(bottle, "drive_c", "Hades", "Hades.exe")
	flatpakRun := []string{"run", "--command=bottles-cli", bottles.FlatpakID, "run", "-b", "Games"}

	spec, err := bottlesSpec(launcher.Spec{}, exe)
	require.NoError(t, err)
	assert.Equal(t, "flatpak", spec.Exe)
	assert.Equal(t, append(slices.Clip(flatpakRun), "-p", "Hades"), spec.Args)
	assert.Equal(t, "Hades.exe", spec.WaitProcess)
	assert.True(t, spec.Detach)
	assert.Equal(t, launcherAppear, spec.WaitAppear)
	assert.Equal(t, home, spec.WorkDir)

	other := filepath.Join(bottle, "drive_c", "Other", "Other.exe")
	spec, err = bottlesSpec(launcher.Spec{WaitProcess: "Real.exe"}, other)
	require.NoError(t, err)
	assert.Equal(t, append(slices.Clip(flatpakRun), "-e", other), spec.Args)
	assert.Equal(t, "Real.exe", spec.WaitProcess)

	_, err = bottlesSpec(launcher.Spec{}, filepath.Join(t.TempDir(), "game.exe"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not in a bottle")
}
