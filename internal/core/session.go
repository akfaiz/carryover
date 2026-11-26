package core

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/akfaiz/carryover/internal/cloudsync/syncer"
	"github.com/akfaiz/carryover/internal/launcher"
	"github.com/akfaiz/carryover/internal/library"
	"github.com/akfaiz/carryover/internal/stores/bottles"
	"github.com/akfaiz/carryover/internal/stores/heroic"
	"github.com/akfaiz/carryover/internal/stores/lutris"
)

// SessionUI is how a play session interacts with the user.
type SessionUI interface {
	// Status reports progress text.
	Status(phase, message string)
	// ResolveConflict asks the user which side to keep. Returning ResolveAuto
	// skips syncing this time (nothing is overwritten).
	ResolveConflict(ctx context.Context, game library.Game, plan *syncer.Plan, beforePlay bool) syncer.Resolution
	// ConfirmLock asks whether to play although another device holds the lock.
	ConfirmLock(ctx context.Context, game library.Game, lock *syncer.Lock) bool
	// ConfirmSyncFailure asks whether to continue after a sync error.
	ConfirmSyncFailure(ctx context.Context, game library.Game, err error, beforePlay bool) bool
	// GameRunning is called while the game process runs (running=false once it exits).
	GameRunning(running bool)
	// SyncCancelable is called with a function that cancels the sync phase
	// in progress (before or after playing), and with nil when it ends. The
	// game itself is never stopped by it.
	SyncCancelable(cancel func())
}

// RunOptions customises a play session.
type RunOptions struct {
	// Command, if non-empty, is launched instead of the configured executable
	// (used for Steam's `-- %command%` wrapping).
	Command []string
	// NoSync skips cloud sync entirely.
	NoSync bool
	// Emulator picks the profile: an emulator id or name, NativeProfile, or ""
	// for the default (see Core.ResolveProfile).
	Emulator string
}

// SessionResult summarises a play session.
type SessionResult struct {
	ExitCode   int
	PreAction  syncer.Action
	PostAction syncer.Action
	// PostError is ErrCancelled when the user cancelled the upload, and wraps
	// ErrOffline when the cloud couldn't be reached (saves kept locally).
	PostError error
	// PreOffline: the cloud couldn't be reached before the game started, so
	// it was started without syncing.
	PreOffline bool
}

// ErrCancelled is returned when the user aborts before the game starts, and
// is the SessionResult.PostError when the user cancels the upload afterwards.
var ErrCancelled = errors.New("cancelled")

// cancelablePhase gives the UI a way to cancel one sync phase. done must be
// called when the phase ends; it reports whether the user cancelled it.
func cancelablePhase(parent context.Context, ui SessionUI) (ctx context.Context, done func() (userCancelled bool)) {
	ctx, cancel := context.WithCancel(parent)
	ui.SyncCancelable(cancel)
	return ctx, func() bool {
		ui.SyncCancelable(nil)
		userCancelled := ctx.Err() != nil && parent.Err() == nil
		cancel()
		return userCancelled
	}
}

// Play runs the full pull → play → push cycle for a game.
func (c *Core) Play(ctx context.Context, gameID string, opt RunOptions, ui SessionUI) (SessionResult, error) {
	var res SessionResult
	game, err := c.Library.Find(gameID)
	if err != nil {
		return res, err
	}
	dev, _ := game.Device(c.DeviceID())

	emuID, err := c.ResolveProfile(game, opt.Emulator)
	if err != nil {
		return res, err
	}
	key := SyncKey(game.ID, emuID) // this profile's own sync state
	spec, err := c.launchSpec(game, dev, opt, emuID)
	if err != nil {
		log.Printf("play %s: %v", key, err)
		return res, err
	}

	s, err := c.beginPlay(ctx, game, emuID, opt, launchLabel(dev, opt, emuID), false, ui)
	if err != nil {
		return s.res, err
	}
	defer s.close()
	game = s.game

	c.rememberProfile(game, emuID)
	title := game.Name
	if emuID != "" {
		title += " (" + c.ProfileName(emuID) + ")"
	}
	ui.Status("playing", "Playing "+title)
	ui.GameRunning(true)
	log.Printf("play %s: starting %s", key, filepath.Base(spec.Exe))
	started := time.Now()
	run, runErr := launcher.Run(ctx, spec, nil)
	ui.GameRunning(false)
	res = s.res
	res.ExitCode = run.ExitCode
	if runErr != nil {
		log.Printf("play %s: game ended after %s, exit code %d: %v", key, time.Since(started).Round(time.Second), run.ExitCode, runErr)
	} else {
		log.Printf("play %s: game ended after %s, exit code %d", key, time.Since(started).Round(time.Second), run.ExitCode)
	}

	if s.doSync {
		res.PostAction, res.PostError = s.end(ui)
	}
	if runErr != nil {
		return res, runErr
	}
	return res, nil
}

// playSession is a play session between its start (the sync before playing,
// the local and cloud locks) and its end (the upload, the release).
type playSession struct {
	c           *Core
	key, ckey   string // SyncKey (sync state) and CloudKey (cloud folder, locks, marker)
	emuID       string
	game        library.Game // as the library catalog left it during the sync before playing
	job         syncer.Job
	eng         *syncer.Engine
	doSync      bool
	lock        *syncer.LocalLock
	lockWritten chan struct{} // closed once the cloud lock is taken (or won't be)
	res         SessionResult // PreAction, PreOffline
}

// beginPlay starts a play session of game with the profile emuID: it takes the
// game's local lock, syncs before playing (asking ui what to do about
// conflicts, failures and another device's lock), writes the session marker
// and takes the cloud lock in the background. launch describes how the game
// starts, for the log. A detached session has no process waiting for the game:
// its marker says so, and no session watcher is started (the caller ends it
// with EndSession).
//
// On success the caller owns the session and must call close (after end). On
// error the session is already closed; the returned session is never nil and
// carries the results so far.
func (c *Core) beginPlay(ctx context.Context, game library.Game, emuID string, opt RunOptions, launch string, detached bool, ui SessionUI) (s *playSession, err error) {
	s = &playSession{c: c, key: SyncKey(game.ID, emuID), ckey: CloudKey(game, emuID), emuID: emuID, game: game}
	key, ckey := s.key, s.ckey
	lock, err := c.waitLocalLock(ctx, ckey, ui)
	if err != nil {
		log.Printf("play %s: %v", key, err)
		return s, err
	}
	s.lock = lock
	c.playing.Store(ckey, struct{}{})
	defer func() {
		if err != nil {
			s.close()
		}
	}()

	slots, _, _ := c.profileData(game, emuID)
	s.doSync = !opt.NoSync && c.CloudConfigured() && len(slots) > 0
	log.Printf("play %s (%s): profile %s, launch %s, sync %s", key, game.Name, profileLabel(emuID), launch, syncLabel(opt, c.CloudConfigured(), len(slots)))
	if s.doSync {
		if s.job, err = c.JobFor(game, emuID); err != nil {
			log.Printf("play %s: %v", key, err)
			return s, err
		}
		if s.eng, err = c.Engine(); err != nil {
			log.Printf("play %s: %v", key, err)
			return s, err
		}
	}

	s.lockWritten = make(chan struct{})
	if !s.doSync {
		close(s.lockWritten)
		return s, nil
	}
	preCtx, preDone := cancelablePhase(ctx, ui)
	ui.Status("sync", "Checking cloud saves…")
	var (
		action  syncer.Action
		other   *syncer.Lock
		planned bool
	)
	err = c.Online(preCtx)
	isOffline := offline(preCtx, err)
	if !isOffline {
		// Not reachable for another reason (e.g. the login expired): the
		// sync below fails with it and the user is asked as before.
		s.game, s.job, action, other, planned, err = c.syncBeforePlay(preCtx, s.eng, s.game, s.job, emuID, ckey, ui)
		// The connection dropped while comparing: nothing was changed yet,
		// so it is the same as being offline. Once a download has started
		// it isn't (the saves may be half replaced): the user decides.
		isOffline = !planned && offline(preCtx, err)
	}
	if preDone() || ctx.Err() != nil {
		// Cancelled by the user, or the whole session was: not a sync error,
		// and the game isn't launched.
		log.Printf("play %s: cancelled before launch", key)
		return s, ErrCancelled
	}
	if isOffline {
		log.Printf("play %s: offline, starting without syncing: %v", key, err)
		s.res.PreOffline = true
		ui.Status("offline", "Offline — playing without syncing")
		pause(ctx, offlineNotice)
		action, other, err = syncer.ActionNone, nil, nil
	}
	s.res.PreAction = action
	if err != nil {
		c.States.SetError(key, err.Error())
		if errors.Is(err, ErrCancelled) || !ui.ConfirmSyncFailure(ctx, s.game, err, true) {
			log.Printf("play %s: not launched after the sync before playing failed: %v", key, err)
			return s, ErrCancelled
		}
		log.Printf("play %s: launching although the sync before playing failed", key)
		err = nil
	}
	if other != nil {
		log.Printf("play %s: cloud lock held by another device since %s", key, other.Since.UTC().Format(time.RFC3339))
		if !ui.ConfirmLock(ctx, s.game, other) {
			log.Printf("play %s: not launched (locked by another device)", key)
			return s, ErrCancelled
		}
	}
	// Mark the game as being played without delaying its start; the
	// release afterwards waits for this write. The marker lets a later
	// run release the lock if this process is killed (RecoverSessions),
	// and the session watcher upload the saves (FinishSession), also
	// after playing offline, if the connection is back by then.
	c.writeSession(ckey, sessionInfo{Game: s.game.ID, Emulator: emuID, Detached: detached})
	if c.WatchSession != nil && !detached {
		go c.WatchSession(ckey)
	}
	if isOffline {
		close(s.lockWritten) // no cloud lock to take
	} else {
		go func() {
			defer close(s.lockWritten)
			_ = s.eng.AcquireLock(ctx, ckey)
		}()
	}
	return s, nil
}

// close releases what this process holds of the session: the local lock and
// the mark that it is playing. The session marker and the cloud lock stay
// until end (or EndSession) releases them.
func (s *playSession) close() {
	s.c.playing.Delete(s.ckey)
	if s.lock != nil {
		s.lock.Release()
		s.lock = nil
	}
}

// end is the end of a session that synced before playing: it uploads the
// saves (asking ui about a conflict or a failure), then releases the cloud
// lock and removes the marker. Offline, the marker is kept for the next start
// or sync. It returns the upload's action and error (ErrCancelled when the
// user cancelled it, wrapping ErrOffline when the cloud couldn't be reached).
func (s *playSession) end(ui SessionUI) (syncer.Action, error) {
	c, key, ckey := s.c, s.key, s.ckey
	// Use a fresh context: the game may have been closed because ctx was cancelled.
	postCtx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()
	upCtx, upDone := cancelablePhase(postCtx, ui)
	ui.Status("sync", "Uploading saves…")
	action := syncer.ActionNone
	err := c.Online(upCtx)
	isOffline := offline(upCtx, err)
	if !isOffline {
		action, err = c.syncWithUI(upCtx, s.eng, s.game, s.job, ui, false)
		// An upload cut short changes nothing locally, and the next push
		// repairs what it left in the cloud.
		isOffline = offline(upCtx, err)
	}
	switch {
	case upDone():
		// Saves stay as they are locally; the next sync uploads them.
		action, err = syncer.ActionNone, ErrCancelled
		log.Printf("play %s: upload after playing cancelled by the user", key)
	case isOffline:
		log.Printf("play %s: offline after playing, saves kept on this device: %v", key, err)
		action, err = syncer.ActionNone, ErrOffline
		ui.Status("offline", "Offline — saves kept on this device; they upload on the next sync")
	}
	if err != nil && !errors.Is(err, ErrCancelled) && !errors.Is(err, ErrOffline) {
		c.States.SetError(key, err.Error())
		ui.ConfirmSyncFailure(postCtx, s.game, err, false)
	}
	if isOffline {
		// The cloud lock (if it was taken) can't be released now: keep the
		// marker, so the next start or sync releases it (RecoverSessions).
		// Ended tells the session watcher this session wasn't killed.
		c.writeSession(ckey, sessionInfo{Game: s.game.ID, Emulator: s.emuID, Ended: true})
	} else {
		var wg sync.WaitGroup
		wg.Go(func() {
			<-s.lockWritten
			if err := s.eng.ReleaseLock(postCtx, ckey); err == nil {
				c.clearSession(ckey)
			} else {
				log.Printf("play %s: release cloud lock: %v", key, err)
			}
		})
		wg.Go(func() {
			if err := c.SyncLibrary(postCtx); err != nil {
				log.Printf("library sync: %v", err)
			}
		})
		wg.Wait()
	}
	return action, err
}

// ErrPlaying is returned for a game being played on this device in a session
// started with StartSession and not ended yet. It wraps syncer.ErrLocked, so
// callers that skip a game while it is played or synced skip it too.
var ErrPlaying = fmt.Errorf("being played on this device: %w", syncer.ErrLocked)

// SessionStart is what StartSession did.
type SessionStart struct {
	// Key is the profile's sync key (SyncKey), CloudKey its cloud folder's;
	// EndSession takes CloudKey.
	Key      string `json:"key"`
	CloudKey string `json:"cloudKey"`
	// PreAction is what the sync before playing did; PreOffline is set when
	// the cloud couldn't be reached, so the game starts without syncing.
	PreAction  syncer.Action `json:"preAction"`
	PreOffline bool          `json:"preOffline"`
	// Synced: the session syncs (a cloud is set up and the profile has save
	// locations), so it must be ended with EndSession.
	Synced bool `json:"synced"`
}

// StartSession is the first half of Play for a game that something else
// starts (the Android app launches an emulator): it syncs before playing,
// asking ui as Play does, takes the cloud lock and leaves a detached session
// marker, then returns without launching anything. No local lock is held
// while the game runs: the marker stops manual syncs (ErrPlaying) and
// RecoverSessions from touching the game until EndSession ends the session.
// emulator picks the profile as RunOptions.Emulator does. It returns
// ErrCancelled when the game shouldn't start.
func (c *Core) StartSession(ctx context.Context, gameID, emulator string, ui SessionUI) (SessionStart, error) {
	game, err := c.Library.Find(gameID)
	if err != nil {
		return SessionStart{}, err
	}
	emuID, err := c.ResolveProfile(game, emulator)
	if err != nil {
		return SessionStart{}, err
	}
	out := SessionStart{Key: SyncKey(game.ID, emuID), CloudKey: CloudKey(game, emuID)}
	if c.SessionOpen(out.CloudKey) {
		return out, ErrPlaying
	}
	s, err := c.beginPlay(ctx, game, emuID, RunOptions{Emulator: emulator}, "by the app", true, ui)
	out.PreAction, out.PreOffline, out.Synced = s.res.PreAction, s.res.PreOffline, s.doSync
	if err != nil {
		return out, err
	}
	defer s.close()
	<-s.lockWritten // the session must hold the cloud lock before the game starts
	c.rememberProfile(s.game, emuID)
	return out, nil
}

// SessionOpen reports whether a session started with StartSession for
// cloudKey is still open (not ended with EndSession).
func (c *Core) SessionOpen(cloudKey string) bool {
	info, ok := c.readSession(cloudKey)
	return ok && info.Detached && !info.Ended
}

// EndSession ends a session started with StartSession, after the game closed:
// as FinishSession, it uploads the saves when only this device changed them,
// flags a conflict when both sides did (never downloads), releases the cloud
// lock and removes the marker, and returns what to tell the user. When it
// fails (offline, an expired login, the lock can't be released) the game has
// closed all the same, so the session is over: the marker is kept but marked
// ended, which lets the next play sync before playing and manual syncs run
// again (and report the real error), and RecoverSessions or the next sync
// finishes it. An error wrapping syncer.ErrLocked means another EndSession of
// the key is running: it leaves the marker to that one.
func (c *Core) EndSession(ctx context.Context, cloudKey string) (finished bool, notice *Notice, err error) {
	info, ok := c.readSession(cloudKey)
	if !ok || !info.Detached || info.Ended {
		return c.FinishSession(ctx, cloudKey, 0)
	}
	if !c.CloudConfigured() {
		c.clearSession(cloudKey) // the cloud was removed meanwhile: nothing to upload or release
		return false, nil, nil
	}
	finished, notice, err = c.FinishSession(ctx, cloudKey, 0)
	if err != nil && !errors.Is(err, syncer.ErrLocked) {
		if cur, ok := c.readSession(cloudKey); ok && !cur.Ended {
			cur.Ended = true
			c.writeSession(cloudKey, cur)
		}
	}
	return finished, notice, err
}

// EndPending finishes a session that PendingSessions listed, once the network
// is back, like EndSession. It does nothing (finished=false, no error) when the
// session was finished meanwhile or the game was started again, so a retry in
// the background can never end a session that is being played.
func (c *Core) EndPending(ctx context.Context, cloudKey string) (finished bool, notice *Notice, err error) {
	return c.finishSession(ctx, cloudKey, 0, true)
}

// PendingSession is a session whose game has closed but whose saves aren't
// uploaded yet: its end failed (offline), so the marker was kept.
type PendingSession struct {
	CloudKey string `json:"cloudKey"`
	Key      string `json:"key"` // the profile's sync key
	Game     string `json:"game"`
	Emulator string `json:"emulator,omitempty"`
}

// PendingSessions lists the sessions started with StartSession that ended
// without finishing, for a retry when the network is back (EndSession
// finishes them).
func (c *Core) PendingSessions() []PendingSession {
	entries, err := os.ReadDir(c.Dirs.StateDir())
	if err != nil {
		return nil
	}
	var out []PendingSession
	for _, e := range entries {
		id, ok := strings.CutSuffix(e.Name(), sessionExt)
		if !ok || e.IsDir() {
			continue
		}
		if info, ok := c.readSession(id); ok && info.Detached && info.Ended {
			out = append(out, PendingSession{CloudKey: id, Key: SyncKey(info.Game, info.Emulator), Game: info.Game, Emulator: info.Emulator})
		}
	}
	return out
}

// syncBeforePlay is the sync before a game starts: it compares the saves,
// fetching the library catalog and the cloud lock meanwhile, and applies the
// result. It returns the game and job as the catalog left them, the lock of
// another device (nil if none), and planned=true once comparing succeeded
// (from then on, files may have changed).
func (c *Core) syncBeforePlay(ctx context.Context, eng *syncer.Engine, game library.Game, job syncer.Job, emuID, ckey string, ui SessionUI) (_ library.Game, _ syncer.Job, action syncer.Action, other *syncer.Lock, planned bool, err error) {
	// The library catalog and the cloud lock don't depend on this game's
	// saves: fetch them while the saves are compared (each round trip
	// costs seconds on some providers).
	libDone := make(chan struct{})
	go func() {
		defer close(libDone)
		if err := c.SyncCatalog(ctx); err != nil {
			log.Printf("library sync: %v", err)
		}
	}()
	lockChecked := make(chan struct{})
	go func() {
		defer close(lockChecked)
		if l, err := eng.CheckLock(ctx, ckey); err == nil {
			other = l
		}
	}()
	plan, err := eng.Plan(ctx, job)
	<-libDone
	<-lockChecked
	if err == nil && ctx.Err() == nil {
		// The catalog may have brought new save locations or filters for this
		// game. A plan made with the old ones could upload or delete files the
		// new ones treat differently, so plan again with the new job.
		if g, gerr := c.Library.Get(game.ID); gerr == nil {
			if j, jerr := c.JobFor(g, emuID); jerr == nil && !reflect.DeepEqual(j, job) {
				game, job = g, j
				plan, err = eng.Plan(ctx, job)
			}
		}
	}
	if err != nil {
		return game, job, syncer.ActionNone, other, false, err
	}
	action, err = c.resolveAndApply(ctx, eng, game, plan, ui, true)
	return game, job, action, other, true, err
}

// offlineNotice is how long "Offline — playing without syncing" stays up
// before the game starts, so the user sees why nothing was synced.
var offlineNotice = 1500 * time.Millisecond

// localLockWait is how long Play waits for another Carryover process to let
// go of the game: the session watcher of the previous session may still be
// finishing it (it settles for a few seconds, then checks the connection).
var localLockWait = 20 * time.Second

// waitLocalLock takes the game's local lock, waiting up to localLockWait while
// another process holds it.
func (c *Core) waitLocalLock(ctx context.Context, ckey string, ui SessionUI) (*syncer.LocalLock, error) {
	deadline := time.Now().Add(localLockWait)
	told := false
	for {
		lock, err := syncer.AcquireLocalLock(c.Dirs.StateDir(), ckey)
		if !errors.Is(err, syncer.ErrLocked) || time.Now().After(deadline) {
			return lock, err
		}
		if !told {
			told = true
			ui.Status("sync", "Waiting for the previous session to finish…")
		}
		if !pause(ctx, 500*time.Millisecond) {
			return nil, ErrCancelled
		}
	}
}

// pause waits d, or until ctx ends (then it returns false).
func pause(ctx context.Context, d time.Duration) bool {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-t.C:
		return true
	}
}

// profileLabel names a profile in the log.
func profileLabel(emuID string) string {
	if emuID == "" {
		return "native"
	}
	return "emulator " + emuID
}

// launchLabel says how a game is started, for the log.
func launchLabel(dev library.DeviceConfig, opt RunOptions, emuID string) string {
	switch {
	case len(opt.Command) > 0:
		return "given command (Steam)"
	case emuID != "":
		return "emulator"
	case dev.LaunchMode == "":
		return string(library.LaunchDirect)
	}
	return string(dev.LaunchMode)
}

// syncLabel says whether a session syncs, and why not, for the log.
func syncLabel(opt RunOptions, cloud bool, slots int) string {
	switch {
	case opt.NoSync:
		return "off (--no-sync)"
	case !cloud:
		return "off (no cloud set up)"
	case slots == 0:
		return "off (no save locations)"
	}
	return "on"
}

func (c *Core) syncWithUI(ctx context.Context, eng *syncer.Engine, game library.Game, job syncer.Job, ui SessionUI, before bool) (syncer.Action, error) {
	plan, err := eng.Plan(ctx, job)
	if err != nil {
		return syncer.ActionNone, err
	}
	return c.resolveAndApply(ctx, eng, game, plan, ui, before)
}

// resolveAndApply applies a plan, asking the user first when both sides changed.
func (c *Core) resolveAndApply(ctx context.Context, eng *syncer.Engine, game library.Game, plan *syncer.Plan, ui SessionUI, before bool) (syncer.Action, error) {
	res := syncer.ResolveAuto
	if plan.Action == syncer.ActionConflict {
		res = ui.ResolveConflict(ctx, game, plan, before)
		if res == syncer.ResolveAuto {
			// Leave both sides untouched; flag the conflict for later.
			_, _ = eng.Apply(ctx, plan, syncer.ResolveAuto)
			return syncer.ActionConflict, nil
		}
	}
	return eng.Apply(ctx, plan, res)
}

// romToken is replaced by the game's ROM/file in an emulator's arguments.
const romToken = "{rom}"

// emulatorSpec builds the launch of a game through an emulator on this device.
func (c *Core) emulatorSpec(game library.Game, dev library.DeviceConfig, opt RunOptions, emuID string) (launcher.Spec, error) {
	emu, err := c.Library.Emulator(emuID)
	if err != nil {
		return launcher.Spec{}, fmt.Errorf("the emulator of %s no longer exists", game.Name)
	}
	ed, ok := emu.Device(c.DeviceID())
	spec := launcher.Spec{WaitProcess: ed.WaitProcess, Env: ed.Env}
	if len(opt.Command) > 0 {
		spec.Exe, spec.Args = opt.Command[0], opt.Command[1:]
		return spec, nil
	}
	if !ok || ed.Exe == "" {
		return spec, fmt.Errorf("%s has no executable configured on this device", emu.Name)
	}
	pctx := c.PathContextFor(game, emuID)
	if spec.Exe, err = pctx.Expand(ed.Exe); err != nil {
		return spec, err
	}
	if ed.WorkDir != "" {
		if spec.WorkDir, err = pctx.Expand(ed.WorkDir); err != nil {
			return spec, err
		}
	}
	var rom string
	if r := dev.Emulators[emuID].Rom; r != "" {
		if rom, err = pctx.Expand(r); err != nil {
			return spec, err
		}
	}
	args, err := launcher.SplitArgs(ed.Args)
	if err != nil {
		return spec, err
	}
	used := false
	for i, a := range args {
		if strings.Contains(a, romToken) {
			args[i] = strings.ReplaceAll(a, romToken, rom)
			used = true
		}
	}
	if !used && rom != "" {
		args = append(args, rom)
	}
	spec.Args = args
	return spec, nil
}

// rememberProfile records the profile played last on this device, so it is
// the default next time. Only games that have emulators need it.
func (c *Core) rememberProfile(game library.Game, emuID string) {
	dev, _ := game.Device(c.DeviceID())
	if len(game.Emulators) == 0 || dev.LastEmulator == emuID {
		return
	}
	_, _ = c.Library.UpdateDevice(game.ID, c.DeviceID(), func(d *library.DeviceConfig) { d.LastEmulator = emuID })
}

func (c *Core) launchSpec(game library.Game, dev library.DeviceConfig, opt RunOptions, emuID string) (launcher.Spec, error) {
	if emuID != "" {
		return c.emulatorSpec(game, dev, opt, emuID)
	}
	pctx := c.PathContext(game)
	spec := launcher.Spec{WaitProcess: dev.WaitProcess, Env: dev.Env}
	if len(opt.Command) > 0 {
		spec.Exe, spec.Args = opt.Command[0], opt.Command[1:]
		return spec, nil
	}
	if dev.Exe == "" {
		return spec, fmt.Errorf("%s has no executable configured on this device", game.Name)
	}
	exe, err := pctx.Expand(dev.Exe)
	if err != nil {
		return spec, err
	}
	switch dev.LaunchMode {
	case library.LaunchHeroic:
		return heroicSpec(spec, exe)
	case library.LaunchLutris:
		return lutrisSpec(spec, exe)
	case library.LaunchBottles:
		return bottlesSpec(spec, exe)
	}
	spec.Exe = exe
	if dev.WorkDir != "" {
		if spec.WorkDir, err = pctx.Expand(dev.WorkDir); err != nil {
			return spec, err
		}
	}
	if spec.Args, err = launcher.SplitArgs(dev.Args); err != nil {
		return spec, err
	}
	return spec, nil
}

// HeadlessUI is a SessionUI that never blocks: conflicts are skipped (both
// sides kept, flagged for the GUI), locks and sync failures don't stop play.
type HeadlessUI struct {
	Logf func(format string, args ...any)
}

func (h HeadlessUI) logf(f string, a ...any) {
	if h.Logf != nil {
		h.Logf(f, a...)
	}
}

func (h HeadlessUI) Status(phase, message string) { h.logf("%s", message) }
func (h HeadlessUI) ResolveConflict(_ context.Context, g library.Game, _ *syncer.Plan, _ bool) syncer.Resolution {
	h.logf("%s: save conflict — skipped; resolve it in Carryover", g.Name)
	return syncer.ResolveAuto
}
func (h HeadlessUI) ConfirmLock(_ context.Context, g library.Game, l *syncer.Lock) bool {
	h.logf("%s: warning, also being played on %s since %s", g.Name, l.DeviceName, l.Since.Local().Format(time.Kitchen))
	return true
}
func (h HeadlessUI) ConfirmSyncFailure(_ context.Context, g library.Game, err error, _ bool) bool {
	h.logf("%s: sync failed: %v", g.Name, err)
	return true
}
func (h HeadlessUI) GameRunning(bool)      {}
func (h HeadlessUI) SyncCancelable(func()) {}

// launcherAppear is how long a launcher (Heroic, Lutris) may take to start the
// game: it may first sync its own cloud saves or download a Wine/Proton build.
const launcherAppear = 5 * time.Minute

// heroicSpec launches the Heroic game that exe belongs to through Heroic, and
// waits for the game's process (exe's name unless WaitProcess says otherwise).
func heroicSpec(spec launcher.Spec, exe string) (launcher.Spec, error) {
	home, _ := os.UserHomeDir()
	g, ok := heroic.ForExe(home, exe)
	if !ok {
		return spec, fmt.Errorf("%s is not in a game folder Heroic has installed on this device", exe)
	}
	cmd := g.Command()
	spec.Exe, spec.Args = cmd[0], cmd[1:]
	spec.WorkDir = home
	if spec.WaitProcess == "" {
		spec.WaitProcess = filepath.Base(exe)
	}
	spec.Detach, spec.WaitAppear = true, launcherAppear
	return spec, nil
}

// lutrisSpec launches the Lutris game that exe belongs to through Lutris, and
// waits for the game's process (exe's name unless WaitProcess says otherwise).
func lutrisSpec(spec launcher.Spec, exe string) (launcher.Spec, error) {
	if runtime.GOOS != "linux" {
		return spec, errors.New("starting a game through Lutris is only possible on Linux")
	}
	home, _ := os.UserHomeDir()
	g, ok := lutris.ForExe(home, exe)
	if !ok {
		return spec, fmt.Errorf("%s is not in a game Lutris has installed on this device", exe)
	}
	cmd := g.Command()
	spec.Exe, spec.Args = cmd[0], cmd[1:]
	spec.WorkDir = home
	if spec.WaitProcess == "" {
		spec.WaitProcess = filepath.Base(exe)
	}
	spec.Detach, spec.WaitAppear = true, launcherAppear
	return spec, nil
}

// bottlesSpec launches exe through Bottles, in the bottle it belongs to, and
// waits for exe's process: bottles-cli may return before the game has closed.
func bottlesSpec(spec launcher.Spec, exe string) (launcher.Spec, error) {
	if runtime.GOOS != "linux" {
		return spec, errors.New("starting a game through Bottles is only possible on Linux")
	}
	home, _ := os.UserHomeDir()
	g, ok := bottles.ForExe(home, exe)
	if !ok {
		return spec, fmt.Errorf("%s is not in a bottle on this device", exe)
	}
	cmd := g.Command()
	spec.Exe, spec.Args = cmd[0], cmd[1:]
	spec.WorkDir = home
	if spec.WaitProcess == "" {
		spec.WaitProcess = filepath.Base(exe)
	}
	spec.Detach, spec.WaitAppear = true, launcherAppear
	return spec, nil
}
