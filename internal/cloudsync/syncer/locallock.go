package syncer

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/shirou/gopsutil/v4/process"
)

// ErrLocked is returned when another Carryover process is already syncing/running the game.
var ErrLocked = errors.New("game is already running or syncing in another Carryover process")

// held lists the lock files this process holds. The lock file only carries a
// PID, which can't tell two holders inside one process apart, so without this
// a second sync of a game that is being played would "take over" its lock and,
// on release, delete the one the play session still relies on.
var (
	heldMu sync.Mutex
	held   = map[string]bool{}
)

// freshLockFile is how long a lock file without a readable PID is assumed to
// be one another process is still writing.
const freshLockFile = 5 * time.Second

// LocalLock is an exclusive per-game lock between Carryover processes on one machine.
type LocalLock struct {
	path string
	once sync.Once
}

// AcquireLocalLock creates dir/<gameID>.lock. A lock left by a dead process is taken over.
func AcquireLocalLock(dir, gameID string) (*LocalLock, error) {
	p := filepath.Join(dir, gameID+".lock")
	heldMu.Lock()
	defer heldMu.Unlock()
	if held[p] {
		return nil, ErrLocked
	}
	for range 2 {
		f, err := os.OpenFile(p, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
		if err == nil {
			_, _ = fmt.Fprint(f, os.Getpid())
			f.Close()
			held[p] = true
			return &LocalLock{path: p}, nil
		}
		if !errors.Is(err, os.ErrExist) {
			return nil, err
		}
		b, _ := os.ReadFile(p)
		pid, _ := strconv.Atoi(strings.TrimSpace(string(b)))
		if pid > 0 && pid != os.Getpid() {
			if alive, _ := process.PidExists(int32(pid)); alive {
				return nil, ErrLocked
			}
		}
		if pid <= 0 {
			// Created but not written yet by a process that is acquiring it right
			// now: removing it would let both of them in.
			if fi, err := os.Stat(p); err == nil && time.Since(fi.ModTime()) < freshLockFile {
				return nil, ErrLocked
			}
		}
		_ = os.Remove(p)
	}
	return nil, ErrLocked
}

// Release removes the lock. Releasing twice does nothing.
func (l *LocalLock) Release() {
	if l == nil {
		return
	}
	l.once.Do(func() {
		heldMu.Lock()
		defer heldMu.Unlock()
		_ = os.Remove(l.path)
		delete(held, l.path)
	})
}
