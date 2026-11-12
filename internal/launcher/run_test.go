package launcher

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"
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
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("GO_LAUNCHER_EXIT", strconv.Itoa(exit))
	return Spec{Exe: self, Env: map[string]string{"GO_LAUNCHER_HELPER": out, "GAME_FLAG": "on"}}
}

func TestRunNeedsExecutable(t *testing.T) {
	if _, err := Run(context.Background(), Spec{}, nil); err == nil {
		t.Fatal("Run without an executable: want an error")
	}
}

func TestRunStartFailure(t *testing.T) {
	_, err := Run(context.Background(), Spec{Exe: filepath.Join(t.TempDir(), "missing")}, nil)
	if err == nil {
		t.Fatal("Run of a missing executable: want an error")
	}
}

func TestRunPassesEnvironmentAndWorkDir(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "out.txt")
	spec := helperSpec(t, out, 0)
	spec.WorkDir = dir

	var pid int
	res, err := Run(context.Background(), spec, func(p int) { pid = p })
	if err != nil || res.ExitCode != 0 {
		t.Fatalf("Run = %+v, %v", res, err)
	}
	if pid <= 0 {
		t.Fatalf("onStarted pid = %d", pid)
	}
	if res.Duration <= 0 {
		t.Fatalf("Duration = %v", res.Duration)
	}
	b, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	wantDir, _ := filepath.EvalSymlinks(dir)
	got := string(b)
	if want := wantDir + "\non"; got != want && got != dir+"\non" {
		t.Fatalf("game saw %q, want working dir %q and GAME_FLAG=on", got, wantDir)
	}
}

// Without a work dir an absolute executable runs from its own folder.
func TestRunDefaultsWorkDirToExeFolder(t *testing.T) {
	out := filepath.Join(t.TempDir(), "out.txt")
	spec := helperSpec(t, out, 0)
	if _, err := Run(context.Background(), spec, nil); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(out)
	wantDir, _ := filepath.EvalSymlinks(filepath.Dir(spec.Exe))
	if got := string(b); got != wantDir+"\non" && got != filepath.Dir(spec.Exe)+"\non" {
		t.Fatalf("game ran in %q, want %q", got, wantDir)
	}
}

// A non-zero exit is the game's result, not an error.
func TestRunReturnsExitCode(t *testing.T) {
	spec := helperSpec(t, filepath.Join(t.TempDir(), "out.txt"), 7)
	res, err := Run(context.Background(), spec, nil)
	if err != nil || res.ExitCode != 7 {
		t.Fatalf("Run = %+v, %v; want exit code 7 and no error", res, err)
	}
}

func TestProcessRunning(t *testing.T) {
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	if !processRunning(filepath.Base(self)) {
		t.Fatalf("processRunning(%q) = false for the test process itself", filepath.Base(self))
	}
	if processRunning("carryover-no-such-process-xyz") {
		t.Fatal("processRunning found a process that doesn't exist")
	}
}

// If the process never appears the wait ends once WaitAppear has passed.
func TestWaitForProcessGivesUpWhenItNeverStarts(t *testing.T) {
	start := time.Now()
	waitForProcess(context.Background(), "carryover-no-such-process-xyz", 10*time.Millisecond)
	if d := time.Since(start); d > 5*time.Second {
		t.Fatalf("waited %v for a process that never started", d)
	}
}

// Cancelling the context ends the wait even though the process still runs.
func TestWaitForProcessStopsOnCancel(t *testing.T) {
	self, _ := os.Executable()
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	start := time.Now()
	waitForProcess(ctx, filepath.Base(self), time.Minute)
	if d := time.Since(start); d > 5*time.Second {
		t.Fatalf("waited %v despite the cancelled context", d)
	}
}

// With WaitProcess the run outlasts Exe until that process is gone; a
// detached run doesn't wait for Exe at all.
func TestRunWaitProcess(t *testing.T) {
	spec := helperSpec(t, filepath.Join(t.TempDir(), "out.txt"), 0)
	spec.WaitProcess = "carryover-no-such-process-xyz"
	spec.WaitAppear = 10 * time.Millisecond
	if res, err := Run(context.Background(), spec, nil); err != nil || res.ExitCode != 0 {
		t.Fatalf("Run with WaitProcess = %+v, %v", res, err)
	}

	spec.Detach = true
	res, err := Run(context.Background(), spec, nil)
	if err != nil || res.ExitCode != 0 || res.Duration <= 0 {
		t.Fatalf("detached Run = %+v, %v", res, err)
	}
}
