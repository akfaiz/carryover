package core

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/akfaiz/carryover/internal/app/msg"
	"github.com/akfaiz/carryover/internal/cloudsync/syncer"
)

// A session marker records that a play session on this device holds the game's
// cloud lock. Play writes it before taking the lock and removes it once the
// lock is released. A marker left behind means the wrapper was killed (Steam's
// "Exit Game" sends SIGKILL, or the machine lost power) before it could release
// the lock, so other devices would keep warning "being played here" until the
// lock goes stale. RecoverSessions releases such locks; FinishSession (run by
// the session watcher, outside the game's process tree) also uploads the saves.

const sessionExt = ".session"

// sessionInfo is the content of a session marker: which profile is played, so
// a watcher can upload its saves. Markers written by older versions are empty.
type sessionInfo struct {
	Game     string `json:"game"`
	Emulator string `json:"emulator,omitempty"`
	// Ended: the session ended on its own but couldn't finish (offline after
	// playing; the user was told). Only the cloud lock is left to release,
	// and the saves to upload, which the next sync does anyway.
	Ended bool `json:"ended,omitempty"`
	// Detached: the session was started with StartSession, so no process
	// holds the game's local lock while it is played. Until it ends
	// (EndSession) it is open, not killed: RecoverSessions leaves it alone and
	// manual syncs are refused (ErrPlaying).
	Detached bool `json:"detached,omitempty"`
}

func (c *Core) sessionMarkerPath(cloudKey string) string {
	return filepath.Join(c.Dirs.StateDir(), cloudKey+sessionExt)
}

// markSession records that this device is about to hold cloudKey's cloud lock
// while gameID is played with emuID. Best-effort: without the marker a killed
// session just keeps its lock until it goes stale.
func (c *Core) markSession(cloudKey, gameID, emuID string) {
	c.writeSession(cloudKey, sessionInfo{Game: gameID, Emulator: emuID})
}

func (c *Core) writeSession(cloudKey string, info sessionInfo) {
	b, _ := json.Marshal(info)
	if err := os.WriteFile(c.sessionMarkerPath(cloudKey), b, 0o644); err != nil {
		log.Printf("session marker %s: %v", cloudKey, err)
	}
}

func (c *Core) clearSession(cloudKey string) {
	_ = os.Remove(c.sessionMarkerPath(cloudKey))
}

// readSession reads a session marker. ok is false when there is none; an old
// (empty) or unreadable marker gives a zero sessionInfo.
func (c *Core) readSession(cloudKey string) (info sessionInfo, ok bool) {
	b, err := os.ReadFile(c.sessionMarkerPath(cloudKey))
	if err != nil {
		return info, false
	}
	_ = json.Unmarshal(b, &info)
	return info, true
}

// OpenSession is a session started with StartSession and not ended yet.
type OpenSession struct {
	CloudKey string `json:"cloudKey"`
	Key      string `json:"key"` // the profile's sync key
	Game     string `json:"game"`
	Emulator string `json:"emulator,omitempty"`
}

// OpenSessions lists the sessions started with StartSession that EndSession
// hasn't finished, for an app whose process was killed while a game ran.
func (c *Core) OpenSessions() []OpenSession {
	entries, err := os.ReadDir(c.Dirs.StateDir())
	if err != nil {
		return nil
	}
	var out []OpenSession
	for _, e := range entries {
		id, ok := strings.CutSuffix(e.Name(), sessionExt)
		if !ok || e.IsDir() {
			continue
		}
		if info, ok := c.readSession(id); ok && info.Detached && !info.Ended {
			out = append(out, OpenSession{CloudKey: id, Key: SyncKey(info.Game, info.Emulator), Game: info.Game, Emulator: info.Emulator})
		}
	}
	return out
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
		if _, ok := c.playing.Load(id); ok {
			continue // this process is playing it: AcquireLocalLock can't tell
		}
		if c.SessionOpen(id) {
			continue // started with StartSession and not ended: still being played
		}
		// Holding the local lock proves no live Carryover process is playing
		// this game; a dead process's lock is taken over.
		lock, err := syncer.AcquireLocalLock(c.Dirs.StateDir(), id)
		if err != nil {
			continue
		}
		if eng == nil {
			// Checked once, and only when there is something to release: an
			// offline release would hold the game's local lock for minutes,
			// and a Play started meanwhile couldn't take it.
			if err = c.Online(ctx); err == nil {
				eng, err = c.Engine()
			}
			if err != nil {
				lock.Release()
				if offline(ctx, err) {
					log.Printf("recover sessions: offline, left for the next start or sync: %v", err)
					return recovered, nil
				}
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

// FinishSession completes the play session of cloudKey if its wrapper was
// killed (Steam's "Exit game"): it uploads the saves when only this device
// changed them, flags a conflict when both sides did, and releases the cloud
// lock, as Play would have after the game. It is run by the session watcher,
// a process started outside the game's process tree.
//
// It returns syncer.ErrLocked while the session is still running (its local
// lock is held), and finished=false with no error when there is nothing to do
// (the session ended normally and removed its marker). settle is waited after
// taking over the session, so a game that is still shutting down can finish
// writing its saves. notice, if not nil, is what the user should be told: an
// upload, a conflict or a failure (also when releasing the lock then fails).
// When the cloud can't be reached it returns at once with an error wrapping
// ErrOffline and keeps the marker for RecoverSessions or the next session.
func (c *Core) FinishSession(ctx context.Context, cloudKey string, settle time.Duration) (finished bool, notice *Notice, err error) {
	return c.finishSession(ctx, cloudKey, settle, false)
}

// finishSession is FinishSession; onlyEnded finishes only a session that has
// ended (its marker says Ended) and leaves one that started again alone: the
// marker is checked once the game's local lock is held, which a new session
// holds while it starts.
func (c *Core) finishSession(ctx context.Context, cloudKey string, settle time.Duration, onlyEnded bool) (finished bool, notice *Notice, err error) {
	if _, ok := c.readSession(cloudKey); !ok || !c.CloudConfigured() {
		return false, nil, nil
	}
	if _, ok := c.playing.Load(cloudKey); ok {
		return false, nil, syncer.ErrLocked
	}
	lock, err := syncer.AcquireLocalLock(c.Dirs.StateDir(), cloudKey)
	if err != nil {
		return false, nil, err
	}
	defer lock.Release()
	// The wrapper removes the marker before it releases the local lock: look
	// again now that the lock is ours.
	if _, ok := c.readSession(cloudKey); !ok {
		return false, nil, nil
	}
	if settle > 0 {
		select {
		case <-ctx.Done():
			return false, nil, ctx.Err()
		case <-time.After(settle):
		}
	}
	info, ok := c.readSession(cloudKey)
	if !ok || (onlyEnded && !info.Ended) {
		return false, nil, nil // recovered by someone else meanwhile, or played again
	}
	// Offline, the upload and the release below would retry for minutes
	// while holding the game's local lock, and playing again would fail.
	if err := c.Online(ctx); err != nil {
		if !offline(ctx, err) {
			return false, nil, err
		}
		log.Printf("session %s: offline, left for the next start or sync: %v", cloudKey, err)
		if info.Ended || info.Game == "" {
			return false, nil, err // the session told the user itself
		}
		return false, c.offlineNotice(info), err
	}
	eng, err := c.Engine()
	if err != nil {
		return false, nil, err
	}
	switch {
	case info.Ended:
		log.Printf("session %s ended offline: finishing it", cloudKey)
	case info.Detached:
		log.Printf("session %s ended: finishing it", cloudKey)
	default:
		log.Printf("session %s ended without finishing (killed): finishing it", cloudKey)
	}
	if info.Game != "" {
		notice = c.uploadAfterKill(ctx, eng, info)
	}
	if err := eng.ReleaseLock(ctx, cloudKey); err != nil {
		return false, notice, err
	}
	c.clearSession(cloudKey)
	return true, notice, nil
}

// offlineNotice tells the user that a killed session's saves weren't uploaded,
// as the cloud couldn't be reached (nil if the game no longer exists).
func (c *Core) offlineNotice(info sessionInfo) *Notice {
	g, err := c.Library.Get(info.Game)
	if err != nil {
		return nil
	}
	n := &Notice{Time: time.Now(), Key: SyncKey(info.Game, info.Emulator), Title: g.Name, Kind: NoticeOffline}
	if info.Emulator != "" {
		n.Title += " (" + c.ProfileName(info.Emulator) + ")"
	}
	return n
}

// uploadAfterKill does the upload a killed session missed, and returns what to
// tell the user (nil: nothing happened worth telling). Only an upload is done:
// if the cloud changed meanwhile, that is a conflict for the user to resolve
// (flagged as Play does without an answer), and never a download into a game
// folder nobody is looking at.
func (c *Core) uploadAfterKill(ctx context.Context, eng *syncer.Engine, info sessionInfo) *Notice {
	key := SyncKey(info.Game, info.Emulator)
	g, err := c.Library.Get(info.Game)
	if err != nil {
		log.Printf("finish session %s: %v", key, err)
		return nil // deleted meanwhile: nothing to upload
	}
	n := &Notice{Time: time.Now(), Key: key, Title: g.Name}
	if info.Emulator != "" {
		n.Title += " (" + c.ProfileName(info.Emulator) + ")"
	}
	job, err := c.JobFor(g, info.Emulator)
	if err == nil {
		var plan *syncer.Plan
		if plan, err = eng.Plan(ctx, job); err == nil {
			switch plan.Action {
			case syncer.ActionPush, syncer.ActionConflict:
				var action syncer.Action
				action, err = eng.Apply(ctx, plan, syncer.ResolveAuto)
				if errors.Is(err, syncer.ErrConflict) {
					// Flagged for the user (state.Conflict), not a sync error.
					log.Printf("finish session %s: conflict, both sides changed; left for the user", key)
					n.Kind = NoticeConflict
					return n
				}
				if err == nil {
					log.Printf("finish session %s: %s", key, action)
					n.Kind = NoticeUploaded
					return n
				}
			default:
				log.Printf("finish session %s: nothing to upload (%s)", key, plan.Action)
				return nil
			}
		}
	}
	log.Printf("finish session %s: %v", key, err)
	c.States.SetError(key, err)
	failure := msg.Of(err)
	n.Kind, n.Err, n.ErrMsg = NoticeFailed, failure.Text, failure.Ptr()
	return n
}
