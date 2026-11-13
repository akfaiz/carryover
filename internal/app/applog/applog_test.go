package applog

import (
	"bytes"
	"errors"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func readLog(t *testing.T, dir, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(dir, name))
	require.NoError(t, err)
	return string(b)
}

func TestOpenWritesHeaderAndLogLines(t *testing.T) {
	dir := t.TempDir()
	l, err := Open(dir, "Carryover test")
	require.NoError(t, err)
	log.Printf("hello %d", 42)
	l.Close()

	got := readLog(t, dir, FileName)
	assert.Contains(t, got, "----- Carryover test")
	assert.Contains(t, got, Environment())
	assert.Contains(t, got, "hello 42")
	assert.Contains(t, got, "[", "lines carry the PID")
}

func TestCloseRestoresLoggerAndReleasesFile(t *testing.T) {
	var buf bytes.Buffer
	log.SetOutput(&buf)
	log.SetFlags(0)
	log.SetPrefix("")
	t.Cleanup(func() { log.SetOutput(os.Stderr); log.SetFlags(log.LstdFlags) })

	dir := t.TempDir()
	l, err := Open(dir, "h")
	require.NoError(t, err)
	l.Close()
	log.Print("after")
	assert.Equal(t, "after\n", buf.String())
	// Windows refuses this while the file (or the crash output's copy) is open.
	require.NoError(t, os.RemoveAll(dir))
}

func TestOpenRotatesLargeLog(t *testing.T) {
	old := rotateSize
	rotateSize = 10
	t.Cleanup(func() { rotateSize = old })

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, FileName), []byte("previous run, long enough\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, OldFileName), []byte("ancient\n"), 0o644))
	l, err := Open(dir, "new run")
	require.NoError(t, err)
	l.Close()

	assert.Equal(t, "previous run, long enough\n", readLog(t, dir, OldFileName))
	got := readLog(t, dir, FileName)
	assert.NotContains(t, got, "previous run")
	assert.True(t, strings.Contains(got, "----- new run"))
}

func TestOpenKeepsSmallLog(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, FileName), []byte("earlier\n"), 0o644))
	l, err := Open(dir, "again")
	require.NoError(t, err)
	l.Close()
	assert.True(t, strings.HasPrefix(readLog(t, dir, FileName), "earlier\n"))
	assert.NoFileExists(t, filepath.Join(dir, OldFileName))
}

func TestWriteStartupError(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "logs")
	require.NoError(t, WriteStartupError(dir, errors.New("parse settings.json: bad")))
	assert.Contains(t, readLog(t, dir, FileName), "startup failed: parse settings.json: bad")
}

// A panic on any goroutine ends up in the log with its stack.
func TestCrashIsWrittenToLog(t *testing.T) {
	if dir := os.Getenv("APPLOG_CRASH_DIR"); dir != "" {
		if _, err := Open(dir, "crash test"); err != nil {
			os.Exit(3)
		}
		done := make(chan struct{})
		go func() {
			defer close(done)
			panic("boom from a goroutine")
		}()
		<-done
		return
	}
	dir := t.TempDir()
	cmd := exec.Command(os.Args[0], "-test.run=^TestCrashIsWrittenToLog$")
	cmd.Env = append(os.Environ(), "APPLOG_CRASH_DIR="+dir)
	err := cmd.Run()
	var exit *exec.ExitError
	require.ErrorAs(t, err, &exit, "the child must crash")

	got := readLog(t, dir, FileName)
	assert.Contains(t, got, "----- crash test")
	assert.Contains(t, got, "panic: boom from a goroutine")
	assert.Contains(t, got, "goroutine ")
}
