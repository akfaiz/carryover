package main

import (
	"errors"
	"io"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rclone/rclone/fs/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/akfaiz/carryover/internal/app/appdir"
	"github.com/akfaiz/carryover/internal/app/notify"
	"github.com/akfaiz/carryover/internal/core"
	"github.com/akfaiz/carryover/internal/library"
)

// The test binary can't be a session watcher: tests that play record the
// request instead (see watcherCalls).
func init() {
	startDetached = func(string, []string) error { return nil }
	notifySend = func(notify.App, string, string) error { return nil }
}

// watcherCalls records the session watchers requested until the test ends.
func watcherCalls(t *testing.T) func() [][]string {
	t.Helper()
	var mu sync.Mutex
	var calls [][]string
	prev := startDetached
	startDetached = func(_ string, args []string) error {
		mu.Lock()
		defer mu.Unlock()
		calls = append(calls, slices.Clone(args))
		return nil
	}
	t.Cleanup(func() { startDetached = prev })
	return func() [][]string {
		mu.Lock()
		defer mu.Unlock()
		return slices.Clone(calls)
	}
}

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
	watchers := watcherCalls(t)
	_, code := capture(t, func() int {
		return cmdRun([]string{"--headless", "--no-sync", e.id, "--", "sh", "-c", "exit 7"})
	})
	require.Equal(t, 7, code, "exit code of the game")
	require.NoDirExists(t, filepath.Join(e.cloud, "games"), "--no-sync still touched the cloud")
	require.Empty(t, watchers(), "session watcher started without syncing")

	_, code = capture(t, func() int {
		return cmdRun([]string{"--headless", e.id, "--", "sh", "-c", "exit 0"})
	})
	require.Zero(t, code, "synced headless run")
	require.FileExists(t, filepath.Join(e.cloud, "games", e.id, "current", "saves", "a.sav"), "saves not uploaded by the run")
	require.Eventually(t, func() bool { return len(watchers()) == 1 }, 5*time.Second, 10*time.Millisecond, "session watcher not started")
	home := os.Getenv(appdir.EnvHome)
	require.Equal(t, []string{finishSessionCmd, "--home", home, e.id}, watchers()[0])
}

// The session watcher of a session that ended normally has nothing to do.
func TestCmdFinishSessionAfterNormalEnd(t *testing.T) {
	e := newCLIEnv(t)
	_, code := capture(t, func() int { return cmdFinishSession([]string{"--home", os.Getenv(appdir.EnvHome), e.id}) })
	require.Zero(t, code)
	require.NoDirExists(t, filepath.Join(e.cloud, "games"), "watcher touched the cloud")
	_, code = capture(t, func() int { return cmdFinishSession(nil) })
	require.Equal(t, 2, code, "usage error")
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

// A game with an emulator syncs each profile separately; --emulator picks one.
func TestCmdSyncEmulatorProfiles(t *testing.T) {
	e := newCLIEnv(t)
	c, err := core.New()
	require.NoError(t, err)
	eden, err := c.Library.SaveEmulator(library.Emulator{Name: "Eden"}, c.DeviceID())
	require.NoError(t, err)
	edenSaves := filepath.Join(t.TempDir(), "eden")
	require.NoError(t, os.MkdirAll(edenSaves, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(edenSaves, "e.sav"), []byte("eden"), 0o644))
	g, err := c.Library.Get(e.id)
	require.NoError(t, err)
	g.Emulators = []library.GameEmulator{{EmulatorID: eden.ID, Slots: []library.SaveSlot{{Key: "saves"}}}}
	d := g.Devices[c.DeviceID()]
	d.Emulators = map[string]library.EmulatorConfig{eden.ID: {Paths: map[string]string{"saves": edenSaves}}}
	g.Devices[c.DeviceID()] = d
	_, err = c.Library.Save(g, c.DeviceID())
	require.NoError(t, err)
	closeLogFile()

	out, _ := capture(t, cmdList)
	require.Contains(t, out, "Game A (Eden)")

	out, code := capture(t, func() int { return cmdSync([]string{"Game A", "--emulator", "eden", "--push"}) })
	require.Zero(t, code, out)
	require.FileExists(t, filepath.Join(e.cloud, "games", e.id+"."+eden.ID, "current", "saves", "e.sav"))
	require.NoDirExists(t, filepath.Join(e.cloud, "games", e.id), "native profile synced although --emulator picked Eden")

	// Without the flag every profile is synced.
	out, code = capture(t, func() int { return cmdSync([]string{"Game A", "--push"}) })
	require.Zero(t, code, out)
	require.FileExists(t, filepath.Join(e.cloud, "games", e.id, "current", "saves", "a.sav"))

	_, code = capture(t, func() int { return cmdSync([]string{"Game A", "--emulator=nope"}) })
	require.Equal(t, 1, code, "unknown emulator")
}

func TestReorderFlagsKeepsValues(t *testing.T) {
	assert.Equal(t, []string{"--emulator", "x y", "--push", "game"},
		reorderFlags([]string{"game", "--emulator", "x y", "--push"}, "emulator"))
	assert.Equal(t, []string{"--emulator=x", "game"}, reorderFlags([]string{"game", "--emulator=x"}, "emulator"))
}

// The watcher's notice is shown as a notification; it is kept for Carryover
// to show when no notification could be shown, or none is seen (Game Mode).
func TestTellUser(t *testing.T) {
	newCLIEnv(t)
	c, err := core.New()
	require.NoError(t, err)
	n := core.Notice{Time: time.Now(), Key: "g", Title: "Zelda", Kind: core.NoticeUploaded}
	var got []string
	prev := notifySend
	t.Cleanup(func() { notifySend = prev })
	sendErr := error(nil)
	notifySend = func(app notify.App, title, body string) error {
		got = append(got, app.Name+"|"+title+"|"+body)
		return sendErr
	}

	t.Setenv("XDG_CURRENT_DESKTOP", "KDE")
	tellUser(c, n)
	require.Len(t, got, 1)
	// The text is in the system language here (no language set): check its parts.
	require.True(t, strings.HasPrefix(got[0], "Carryover|Carryover|Zelda: "), got[0])
	require.Empty(t, c.TakeNotices(), "kept although the notification was shown")

	sendErr = errors.New("no notification service")
	tellUser(c, n)
	require.Len(t, c.TakeNotices(), 1, "not kept when the notification failed")

	sendErr = nil
	t.Setenv("XDG_CURRENT_DESKTOP", "gamescope")
	tellUser(c, n)
	require.Len(t, c.TakeNotices(), 1, "not kept in Game Mode")
}

// The session watcher of a killed session: it uploads the saves, releases the
// session and notifies the user.
func TestCmdFinishSessionKilled(t *testing.T) {
	e := newCLIEnv(t)
	home := os.Getenv(appdir.EnvHome)
	_, code := capture(t, func() int { return cmdSync([]string{e.id}) })
	require.Zero(t, code, "first sync")
	require.NoError(t, os.WriteFile(filepath.Join(e.saves, "a.sav"), []byte("played"), 0o644))
	// What a wrapper killed by Steam leaves: its session marker (the local
	// lock of the dead process doesn't matter).
	marker := filepath.Join(home, "state", e.id+".session")
	require.NoError(t, os.WriteFile(marker, []byte(`{"game":"`+e.id+`"}`), 0o644))

	prevSettle := watchSettle
	watchSettle = 0
	t.Cleanup(func() { watchSettle = prevSettle })
	var sent []string
	prevSend := notifySend
	notifySend = func(_ notify.App, _, body string) error {
		sent = append(sent, body)
		return nil
	}
	t.Cleanup(func() { notifySend = prevSend })
	t.Setenv("XDG_CURRENT_DESKTOP", "KDE")

	_, code = capture(t, func() int { return cmdFinishSession([]string{"--home", home, e.id}) })
	require.Zero(t, code)
	got, err := os.ReadFile(filepath.Join(e.cloud, "games", e.id, "current", "saves", "a.sav"))
	require.NoError(t, err)
	require.Equal(t, "played", string(got), "saves not uploaded")
	require.NoFileExists(t, marker, "session not finished")
	require.Len(t, sent, 1, "user not notified")
	require.True(t, strings.HasPrefix(sent[0], "Game A: "), sent[0])
}
