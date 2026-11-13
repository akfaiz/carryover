// Package applog sets up Carryover's log file, <data>/logs/carryover.log.
//
// It is what users attach to bug reports, so it must explain a crash too:
// besides the standard logger, the Go runtime writes the stack of a fatal
// panic there (debug.SetCrashOutput). The GUI build has no stderr on Windows,
// and Steam swallows it on Linux, so without that a crash leaves no trace.
//
// Several processes (the launcher, `carryover run` windows, the CLI) append to
// the same file; every line carries the writer's PID.
package applog

import (
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strings"
	"sync"
)

const (
	// FileName is the current log; OldFileName holds the one before it.
	FileName    = "carryover.log"
	OldFileName = "carryover.1.log"
)

// Sizes at which the log is rotated, and truncated when rotating fails (on
// Windows another Carryover process may hold it open). Variables for tests.
var (
	rotateSize   int64 = 5 << 20
	truncateSize int64 = 20 << 20
)

// Log is the open log file. Close restores the standard logger.
type Log struct {
	f       *os.File
	prevOut io.Writer
	prevFlg int
	prevPre string
}

var (
	mu      sync.Mutex
	current *Log
)

// Open starts logging to logsDir/carryover.log (rotated first when large) in
// addition to stderr, and sends fatal crash output there. header is written
// to the file only, after the environment line; it says which build runs.
func Open(logsDir, header string) (*Log, error) {
	path := filepath.Join(logsDir, FileName)
	rotate(path, filepath.Join(logsDir, OldFileName))
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, err
	}
	l := &Log{f: f, prevOut: log.Writer(), prevFlg: log.Flags(), prevPre: log.Prefix()}
	mu.Lock()
	if current != nil {
		current.closeLocked()
	}
	current = l
	mu.Unlock()

	log.SetFlags(log.LstdFlags | log.Lmicroseconds | log.Lmsgprefix)
	log.SetPrefix(fmt.Sprintf("[%d] ", os.Getpid()))
	log.SetOutput(io.MultiWriter(os.Stderr, f))
	// Fatal panics and runtime errors (e.g. a concurrent map write) on any
	// goroutine are written here, right after the last log lines.
	_ = debug.SetCrashOutput(f, debug.CrashOptions{})

	fl := log.New(f, log.Prefix(), log.Flags())
	fl.Printf("----- %s", header)
	fl.Printf("%s", Environment())
	return l, nil
}

// Close stops crash output to the file, closes it and restores the standard
// logger. The process normally keeps the log open until it exits; tests close
// it so the data dir can be deleted (Windows refuses to delete an open file).
func (l *Log) Close() {
	mu.Lock()
	defer mu.Unlock()
	l.closeLocked()
	if current == l {
		current = nil
	}
}

func (l *Log) closeLocked() {
	if l.f == nil {
		return
	}
	// SetCrashOutput keeps its own handle of the file: release it first.
	_ = debug.SetCrashOutput(nil, debug.CrashOptions{})
	log.SetOutput(l.prevOut)
	log.SetFlags(l.prevFlg)
	log.SetPrefix(l.prevPre)
	_ = l.f.Close()
	l.f = nil
}

// Exit logs the exit code to the log file (only) and exits. A log that ends
// with neither an exit line nor a crash means the process was killed.
func Exit(code int) {
	mu.Lock()
	if current != nil && current.f != nil {
		log.New(current.f, log.Prefix(), log.Flags()).Printf("exit %d", code)
	}
	mu.Unlock()
	os.Exit(code)
}

// rotate moves a large log to oldPath. When that fails (Windows: another
// process has it open) logging continues in the same file, unless it has
// grown far too large, then it is emptied.
func rotate(path, oldPath string) {
	fi, err := os.Stat(path)
	if err != nil || fi.Size() <= rotateSize {
		return
	}
	_ = os.Remove(oldPath)
	if err := os.Rename(path, oldPath); err == nil || fi.Size() <= truncateSize {
		return
	}
	_ = os.Truncate(path, 0)
}

// Environment describes the platform for the log header: OS, Go version and,
// on Linux, hints of how the process was started (Steam, Game Mode). It holds
// no paths or user names.
func Environment() string {
	parts := []string{runtime.GOOS + "/" + runtime.GOARCH, runtime.Version()}
	for _, k := range []string{"SteamDeck", "SteamGameId", "XDG_CURRENT_DESKTOP", "XDG_SESSION_TYPE"} {
		if v := os.Getenv(k); v != "" {
			parts = append(parts, k+"="+v)
		}
	}
	if os.Getenv("GAMESCOPE_WAYLAND_DISPLAY") != "" {
		parts = append(parts, "gamescope")
	}
	if os.Getenv("APPIMAGE") != "" {
		parts = append(parts, "appimage")
	}
	return strings.Join(parts, ", ")
}

// WriteStartupError appends a startup failure to the log in logsDir, for when
// the log could not be opened the normal way (the data dir is broken).
func WriteStartupError(logsDir string, err error) error {
	if logsDir == "" {
		return errors.New("no log directory")
	}
	if mkErr := os.MkdirAll(logsDir, 0o755); mkErr != nil {
		return mkErr
	}
	f, ferr := os.OpenFile(filepath.Join(logsDir, FileName), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if ferr != nil {
		return ferr
	}
	defer f.Close()
	l := log.New(f, fmt.Sprintf("[%d] ", os.Getpid()), log.LstdFlags|log.Lmicroseconds|log.Lmsgprefix)
	l.Printf("%s", Environment())
	l.Printf("startup failed: %v", err)
	return nil
}
