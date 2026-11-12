package main

import (
	"io"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/rclone/rclone/fs/config"

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
	if err != nil {
		t.Fatal(err)
	}
	config.FileSetValue("cloud", "type", "local")
	cloud := filepath.Join(home, "cloud")
	st := c.Settings.Get()
	st.Remote, st.RemoteRoot = "cloud", filepath.ToSlash(cloud)
	if err := c.Settings.Update(st); err != nil {
		t.Fatal(err)
	}
	saves := filepath.Join(home, "saves")
	if err := os.MkdirAll(saves, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(saves, "a.sav"), []byte("save"), 0o644); err != nil {
		t.Fatal(err)
	}
	g, err := c.Library.Save(library.Game{
		Name:    "Game A",
		Slots:   []library.SaveSlot{{Key: "saves"}},
		Devices: map[string]library.DeviceConfig{c.DeviceID(): {Paths: map[string]string{"saves": saves}, Exe: "x", LaunchMode: library.LaunchDirect}},
	}, c.DeviceID())
	if err != nil {
		t.Fatal(err)
	}
	return cliEnv{id: g.ID, saves: saves, cloud: cloud}
}

// capture runs fn and returns what it wrote to stdout. Commands also redirect
// the standard logger to a file in the data dir, so it is restored after.
func capture(t *testing.T, fn func() int) (string, int) {
	t.Helper()
	oldOut, oldErr := os.Stdout, os.Stderr
	oldLog := log.Writer()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
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
			w.Close()
		}()
		code = fn()
	}()
	return <-done, code
}

func TestCmdListShowsGames(t *testing.T) {
	e := newCLIEnv(t)
	out, code := capture(t, cmdList)
	if code != 0 || !strings.Contains(out, e.id) || !strings.Contains(out, "Game A") {
		t.Fatalf("cmdList = %d, %q", code, out)
	}
}

func TestCmdListEmpty(t *testing.T) {
	t.Setenv(appdir.EnvHome, t.TempDir())
	if _, code := capture(t, cmdList); code != 0 {
		t.Fatalf("cmdList on an empty library = %d", code)
	}
}

func TestCmdSyncUsageErrors(t *testing.T) {
	for _, args := range [][]string{
		nil,                       // no game and no --all
		{"--push", "--pull", "x"}, // contradictory
		{"--bogus", "x"},          // unknown flag
	} {
		if _, code := capture(t, func() int { return cmdSync(args) }); code != 2 {
			t.Errorf("cmdSync(%q) = %d, want 2", args, code)
		}
	}
}

func TestCmdSyncPushesThenDryRunsClean(t *testing.T) {
	e := newCLIEnv(t)

	// A dry run changes nothing, not even the cloud.
	out, code := capture(t, func() int { return cmdSync([]string{"--dry-run", "Game A"}) })
	if code != 0 || !strings.Contains(out, "would upload") {
		t.Fatalf("dry run = %d, %q", code, out)
	}
	if _, err := os.Stat(filepath.Join(e.cloud, "games")); !os.IsNotExist(err) {
		t.Fatalf("a dry run wrote to the cloud (err=%v)", err)
	}

	// The game can be named by its exact name or its id; flags may follow it.
	out, code = capture(t, func() int { return cmdSync([]string{e.id, "--push"}) })
	if code != 0 || !strings.Contains(out, "push") {
		t.Fatalf("sync = %d, %q", code, out)
	}
	if _, err := os.Stat(filepath.Join(e.cloud, "games", e.id, "current", "saves", "a.sav")); err != nil {
		t.Fatalf("save not uploaded: %v", err)
	}

	out, code = capture(t, func() int { return cmdSync([]string{"--all", "--dry-run"}) })
	if code != 0 || !strings.Contains(out, "do nothing") {
		t.Fatalf("--all --dry-run after a sync = %d, %q", code, out)
	}
}

func TestCmdSyncUnknownGame(t *testing.T) {
	newCLIEnv(t)
	if _, code := capture(t, func() int { return cmdSync([]string{"No Such Game"}) }); code != 1 {
		t.Fatalf("unknown game = %d, want 1", code)
	}
}

// Forcing a direction on every game needs --yes when no terminal can answer.
func TestCmdSyncAllForceNeedsConfirmation(t *testing.T) {
	e := newCLIEnv(t)
	if runtime.GOOS == "windows" {
		t.Skip("stdin is a character device here")
	}
	f, err := os.Open(os.DevNull)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	oldIn := os.Stdin
	os.Stdin = f
	defer func() { os.Stdin = oldIn }()

	// /dev/null is a character device that yields EOF: counts as no answer.
	out, code := capture(t, func() int { return cmdSync([]string{"--all", "--push"}) })
	if code != 1 || !strings.Contains(out, "aborted") {
		t.Fatalf("--all --push = %d, %q", code, out)
	}
	if _, err := os.Stat(filepath.Join(e.cloud, "games")); !os.IsNotExist(err) {
		t.Fatal("saves uploaded without confirmation")
	}

	// --yes skips the question.
	if _, code := capture(t, func() int { return cmdSync([]string{"--all", "--push", "--yes"}) }); code != 0 {
		t.Fatalf("--all --push --yes = %d", code)
	}
}

func TestCmdRunUsageErrors(t *testing.T) {
	for _, args := range [][]string{nil, {"--bogus", "x"}, {"a", "b"}} {
		if _, code := capture(t, func() int { return cmdRun(args) }); code != 2 {
			t.Errorf("cmdRun(%q) = %d, want 2", args, code)
		}
	}
}

// A headless run launches the command given after "--" instead of the exe,
// returns its exit code, and syncs around it.
func TestCmdRunHeadless(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses sh")
	}
	e := newCLIEnv(t)
	if _, code := capture(t, func() int {
		return cmdRun([]string{"--headless", "--no-sync", e.id, "--", "sh", "-c", "exit 7"})
	}); code != 7 {
		t.Fatalf("exit code = %d, want the game's 7", code)
	}
	if _, err := os.Stat(filepath.Join(e.cloud, "games")); !os.IsNotExist(err) {
		t.Fatal("--no-sync still touched the cloud")
	}

	if _, code := capture(t, func() int {
		return cmdRun([]string{"--headless", e.id, "--", "sh", "-c", "exit 0"})
	}); code != 0 {
		t.Fatalf("synced headless run = %d", code)
	}
	if _, err := os.Stat(filepath.Join(e.cloud, "games", e.id, "current", "saves", "a.sav")); err != nil {
		t.Fatalf("saves not uploaded by the run: %v", err)
	}
}

func TestCmdRunHeadlessUnknownGame(t *testing.T) {
	newCLIEnv(t)
	if _, code := capture(t, func() int { return cmdRun([]string{"--headless", "nope"}) }); code != 1 {
		t.Fatalf("unknown game = %d, want 1", code)
	}
}

func TestRecoverSessionsHelperWithoutMarkers(t *testing.T) {
	newCLIEnv(t)
	c, err := core.New()
	if err != nil {
		t.Fatal(err)
	}
	recoverSessions(c) // must not panic or fail without leftovers
}
