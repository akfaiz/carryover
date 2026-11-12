package core

import (
	"context"
	"errors"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/akfaiz/carryover/internal/syncer"
)

// A session marker records that a play session on this device holds the game's
// cloud lock. Play writes it before taking the lock and removes it once the
// lock is released. A marker left behind means the wrapper was killed (Steam's
// "Exit Game" sends SIGKILL, or the machine lost power) before it could release
// the lock, so other devices would keep warning "being played here" until the
// lock goes stale. RecoverSessions releases such locks.

const sessionExt = ".session"

func (c *Core) sessionMarkerPath(gameID string) string {
	return filepath.Join(c.Dirs.StateDir(), gameID+sessionExt)
}

// markSession records that this device is about to hold gameID's cloud lock.
// Best-effort: without the marker a killed session just keeps its lock until
// it goes stale.
func (c *Core) markSession(gameID string) {
	if err := os.WriteFile(c.sessionMarkerPath(gameID), nil, 0o644); err != nil {
		log.Printf("session marker %s: %v", gameID, err)
	}
}

func (c *Core) clearSession(gameID string) {
	_ = os.Remove(c.sessionMarkerPath(gameID))
}

// RecoverSessions releases the cloud locks of play sessions that were killed
// before they could do it themselves, and returns how many games it cleaned
// up. Sessions still running (their local lock is held) are left alone. It
// does nothing without a cloud remote; a marker whose release fails stays for
// the next call.
func (c *Core) RecoverSessions(ctx context.Context) (int, error) {
	if !c.CloudConfigured() {
		return 0, nil
	}
	entries, err := os.ReadDir(c.Dirs.StateDir())
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return 0, nil
		}
		return 0, err
	}
	var eng *syncer.Engine
	recovered := 0
	for _, e := range entries {
		id, ok := strings.CutSuffix(e.Name(), sessionExt)
		if !ok || e.IsDir() {
			continue
		}
		// Holding the local lock proves no live Carryover process is playing
		// this game; a dead process's lock is taken over.
		lock, err := syncer.AcquireLocalLock(c.Dirs.StateDir(), id)
		if err != nil {
			continue
		}
		if eng == nil {
			if eng, err = c.Engine(); err != nil {
				lock.Release()
				return recovered, err
			}
		}
		err = eng.ReleaseLock(ctx, id)
		if err == nil {
			c.clearSession(id)
			recovered++
			log.Printf("released the cloud lock of an interrupted session of game %s", id)
		}
		lock.Release()
		if err != nil {
			return recovered, err
		}
	}
	return recovered, nil
}
