package launcher

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The test binary doubles as the "game": run with GO_LAUNCHER_HELPER set, it
// writes its working directory and an env var to a file, then exits with the
// requested code.
func TestMain(m *testing.M) {
	if out := os.Getenv("GO_LAUNCHER_HELPER"); out != "" {
		wd, _ := os.Getwd()
		_ = os.WriteFile(out, []byte(wd+"\n"+os.Getenv("GAME_FLAG")), 0o644)
		code, _ := strconv.Atoi(os.Getenv("GO_LAUNCHER_EXIT"))
		os.Exit(code)
	}
	os.Exit(m.Run())
}

func helperSpec(t *testing.T, out string, exit int) Spec {
	t.Helper()
	self, err := os.Executable()
	require.NoError(t, err)
	t.Setenv("GO_LAUNCHER_EXIT", strconv.Itoa(exit))
	return Spec{Exe: self, Env: map[string]string{"GO_LAUNCHER_HELPER": out, "GAME_FLAG": "on"}}
}

func TestRunNeedsExecutable(t *testing.T) {
	_, err := Run(context.Background(), Spec{}, nil)
	require.Error(t, err, "Run without an executable")
}

func TestRunStartFailure(t *testing.T) {
	_, err := Run(context.Background(), Spec{Exe: filepath.Join(t.TempDir(), "missing")}, nil)
	require.Error(t, err, "Run of a missing executable")
}

func TestRunPassesEnvironmentAndWorkDir(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "out.txt")
	spec := helperSpec(t, out, 0)
	spec.WorkDir = dir

	var pid int
	res, err := Run(context.Background(), spec, func(p int) { pid = p })
	require.NoError(t, err)
	assert.Zero(t, res.ExitCode)
	assert.Positive(t, pid, "onStarted pid")
	assert.Positive(t, res.Duration)
	b, err := os.ReadFile(out)
	require.NoError(t, err)
	wantDir, _ := filepath.EvalSymlinks(dir)
	got := string(b)
	assert.Contains(t, []string{wantDir + "\non", dir + "\non"}, got, "game saw its working dir and GAME_FLAG=on")
}

// Without a work dir an absolute executable runs from its own folder.
func TestRunDefaultsWorkDirToExeFolder(t *testing.T) {
	out := filepath.Join(t.TempDir(), "out.txt")
	spec := helperSpec(t, out, 0)
	_, err := Run(context.Background(), spec, nil)
	require.NoError(t, err)
	b, _ := os.ReadFile(out)
	wantDir, _ := filepath.EvalSymlinks(filepath.Dir(spec.Exe))
	assert.Contains(t, []string{wantDir + "\non", filepath.Dir(spec.Exe) + "\non"}, string(b), "game ran in the exe's folder")
}

// A non-zero exit is the game's result, not an error.
func TestRunReturnsExitCode(t *testing.T) {
	spec := helperSpec(t, filepath.Join(t.TempDir(), "out.txt"), 7)
	res, err := Run(context.Background(), spec, nil)
	require.NoError(t, err)
	require.Equal(t, 7, res.ExitCode)
}

func TestProcessRunning(t *testing.T) {
	self, err := os.Executable()
	require.NoError(t, err)
	require.True(t, processRunning(filepath.Base(self)))
	require.False(t, processRunning("carryover-no-such-process-xyz"), "found a process that doesn't exist")
}

// If the process never appears the wait ends once WaitAppear has passed.
func TestWaitForProcessGivesUpWhenItNeverStarts(t *testing.T) {
	start := time.Now()
	waitForProcess(context.Background(), "carryover-no-such-process-xyz", 10*time.Millisecond)
	require.Less(t, time.Since(start), 5*time.Second, "waited for a process that never started")
}

// Cancelling the context ends the wait even though the process still runs.
func TestWaitForProcessStopsOnCancel(t *testing.T) {
	self, _ := os.Executable()
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	start := time.Now()
	waitForProcess(ctx, filepath.Base(self), time.Minute)
	require.Less(t, time.Since(start), 5*time.Second, "waited despite the cancelled context")
}

// With WaitProcess the run outlasts Exe until that process is gone; a
// detached run doesn't wait for Exe at all.
func TestRunWaitProcess(t *testing.T) {
	spec := helperSpec(t, filepath.Join(t.TempDir(), "out.txt"), 0)
	spec.WaitProcess = "carryover-no-such-process-xyz"
	spec.WaitAppear = 10 * time.Millisecond
	res, err := Run(context.Background(), spec, nil)
	require.NoError(t, err, "Run with WaitProcess")
	require.Zero(t, res.ExitCode)

	spec.Detach = true
	res, err = Run(context.Background(), spec, nil)
	require.NoError(t, err, "detached Run")
	require.Zero(t, res.ExitCode)
	require.Positive(t, res.Duration)
}
