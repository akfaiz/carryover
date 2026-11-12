package main

import (
	"io"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/rclone/rclone/fs/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/akfaiz/carryover/internal/appdir"
	"github.com/akfaiz/carryover/internal/core"
	"github.com/akfaiz/carryover/internal/library"
)

// cliEnv is an isolated data dir with a local "cloud" remote and one game, for
// running the command line modes against.
type cliEnv struct {
	id, saves, cloud string
}

func newCLIEnv(t *testing.T) cliEnv {
	t.Helper()
	home := t.TempDir()
	t.Setenv(appdir.EnvHome, home)
	c, err := core.New()
	require.NoError(t, err)
	config.FileSetValue("cloud", "type", "local")
	cloud := filepath.Join(home, "cloud")
	st := c.Settings.Get()
	st.Remote, st.RemoteRoot = "cloud", filepath.ToSlash(cloud)
	require.NoError(t, c.Settings.Update(st))
	saves := filepath.Join(home, "saves")
	require.NoError(t, os.MkdirAll(saves, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(saves, "a.sav"), []byte("save"), 0o644))
	g, err := c.Library.Save(library.Game{
		Name:    "Game A",
		Slots:   []library.SaveSlot{{Key: "saves"}},
		Devices: map[string]library.DeviceConfig{c.DeviceID(): {Paths: map[string]string{"saves": saves}, Exe: "x", LaunchMode: library.LaunchDirect}},
	}, c.DeviceID())
	require.NoError(t, err)
	return cliEnv{id: g.ID, saves: saves, cloud: cloud}
}

// capture runs fn and returns what it wrote to stdout. Commands also redirect
// the standard logger to a file in the data dir, so it is restored after.
func capture(t *testing.T, fn func() int) (string, int) {
	t.Helper()
	oldOut, oldErr := os.Stdout, os.Stderr
	oldLog := log.Writer()
	r, w, err := os.Pipe()
	require.NoError(t, err)
	os.Stdout, os.Stderr = w, w
	var code int
	done := make(chan string)
	go func() {
		b, _ := io.ReadAll(r)
		done <- string(b)
	}()
	func() {
		defer func() {
			os.Stdout, os.Stderr = oldOut, oldErr
			log.SetOutput(oldLog)
			closeLogFile() // Windows can't delete the data dir while the log is open
			w.Close()
		}()
		code = fn()
	}()
	return <-done, code
}

func TestCmdListShowsGames(t *testing.T) {
	e := newCLIEnv(t)
	out, code := capture(t, cmdList)
	require.Zero(t, code)
	require.Contains(t, out, e.id)
	require.Contains(t, out, "Game A")
}

func TestCmdListEmpty(t *testing.T) {
	t.Setenv(appdir.EnvHome, t.TempDir())
	_, code := capture(t, cmdList)
	require.Zero(t, code, "cmdList on an empty library")
}

func TestCmdSyncUsageErrors(t *testing.T) {
	for _, args := range [][]string{
		nil,                       // no game and no --all
		{"--push", "--pull", "x"}, // contradictory
		{"--bogus", "x"},          // unknown flag
	} {
		_, code := capture(t, func() int { return cmdSync(args) })
		assert.Equal(t, 2, code, "cmdSync(%q)", args)
	}
}

func TestCmdSyncPushesThenDryRunsClean(t *testing.T) {
	e := newCLIEnv(t)

	// A dry run changes nothing, not even the cloud.
	out, code := capture(t, func() int { return cmdSync([]string{"--dry-run", "Game A"}) })
	require.Zero(t, code, "dry run")
	require.Contains(t, out, "would upload")
	require.NoDirExists(t, filepath.Join(e.cloud, "games"), "a dry run wrote to the cloud")

	// The game can be named by its exact name or its id; flags may follow it.
	out, code = capture(t, func() int { return cmdSync([]string{e.id, "--push"}) })
	require.Zero(t, code, "sync")
	require.Contains(t, out, "push")
	require.FileExists(t, filepath.Join(e.cloud, "games", e.id, "current", "saves", "a.sav"), "save not uploaded")

	out, code = capture(t, func() int { return cmdSync([]string{"--all", "--dry-run"}) })
	require.Zero(t, code, "--all --dry-run after a sync")
	require.Contains(t, out, "do nothing")
}

func TestCmdSyncUnknownGame(t *testing.T) {
	newCLIEnv(t)
	_, code := capture(t, func() int { return cmdSync([]string{"No Such Game"}) })
	require.Equal(t, 1, code, "unknown game")
}

// Forcing a direction on every game needs --yes when no terminal can answer.
func TestCmdSyncAllForceNeedsConfirmation(t *testing.T) {
	e := newCLIEnv(t)
	if runtime.GOOS == "windows" {
		t.Skip("stdin is a character device here")
	}
	f, err := os.Open(os.DevNull)
	require.NoError(t, err)
	defer f.Close()
	oldIn := os.Stdin
	os.Stdin = f
	defer func() { os.Stdin = oldIn }()

	// /dev/null is a character device that yields EOF: counts as no answer.
	out, code := capture(t, func() int { return cmdSync([]string{"--all", "--push"}) })
	require.Equal(t, 1, code, "--all --push")
	require.Contains(t, out, "aborted")
	require.NoDirExists(t, filepath.Join(e.cloud, "games"), "saves uploaded without confirmation")

	// --yes skips the question.
	_, code = capture(t, func() int { return cmdSync([]string{"--all", "--push", "--yes"}) })
	require.Zero(t, code, "--all --push --yes")
}

func TestCmdRunUsageErrors(t *testing.T) {
	for _, args := range [][]string{nil, {"--bogus", "x"}, {"a", "b"}} {
		_, code := capture(t, func() int { return cmdRun(args) })
		assert.Equal(t, 2, code, "cmdRun(%q)", args)
	}
}

// A headless run launches the command given after "--" instead of the exe,
// returns its exit code, and syncs around it.
func TestCmdRunHeadless(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses sh")
	}
	e := newCLIEnv(t)
	_, code := capture(t, func() int {
		return cmdRun([]string{"--headless", "--no-sync", e.id, "--", "sh", "-c", "exit 7"})
	})
	require.Equal(t, 7, code, "exit code of the game")
	require.NoDirExists(t, filepath.Join(e.cloud, "games"), "--no-sync still touched the cloud")

	_, code = capture(t, func() int {
		return cmdRun([]string{"--headless", e.id, "--", "sh", "-c", "exit 0"})
	})
	require.Zero(t, code, "synced headless run")
	require.FileExists(t, filepath.Join(e.cloud, "games", e.id, "current", "saves", "a.sav"), "saves not uploaded by the run")
}

func TestCmdRunHeadlessUnknownGame(t *testing.T) {
	newCLIEnv(t)
	_, code := capture(t, func() int { return cmdRun([]string{"--headless", "nope"}) })
	require.Equal(t, 1, code, "unknown game")
}

func TestRecoverSessionsHelperWithoutMarkers(t *testing.T) {
	newCLIEnv(t)
	c, err := core.New()
	require.NoError(t, err)
	recoverSessions(c) // must not panic or fail without leftovers
}
