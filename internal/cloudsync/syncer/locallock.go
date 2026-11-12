package syncer

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/shirou/gopsutil/v4/process"
)

// ErrLocked is returned when another Carryover process is already syncing/running the game.
var ErrLocked = errors.New("game is already running or syncing in another Carryover process")

// LocalLock is an exclusive per-game lock between Carryover processes on one machine.
type LocalLock struct {
	path string
}

// AcquireLocalLock creates dir/<gameID>.lock. A lock left by a dead process is taken over.
func AcquireLocalLock(dir, gameID string) (*LocalLock, error) {
	p := filepath.Join(dir, gameID+".lock")
	for range 2 {
		f, err := os.OpenFile(p, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
		if err == nil {
			_, _ = fmt.Fprint(f, os.Getpid())
			f.Close()
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
		_ = os.Remove(p)
	}
	return nil, ErrLocked
}

// Release removes the lock.
func (l *LocalLock) Release() {
	if l != nil {
		_ = os.Remove(l.path)
	}
}
