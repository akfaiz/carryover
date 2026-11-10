package core

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/akfaiz/carryover/internal/launcher"
	"github.com/akfaiz/carryover/internal/library"
	"github.com/akfaiz/carryover/internal/syncer"
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
}

// SessionResult summarises a play session.
type SessionResult struct {
	ExitCode   int
	PreAction  syncer.Action
	PostAction syncer.Action
	PostError  error
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

	spec, err := c.launchSpec(game, dev, opt)
	if err != nil {
		return res, err
	}

	lock, err := syncer.AcquireLocalLock(c.Dirs.StateDir(), game.ID)
	if err != nil {
		return res, err
	}
	defer lock.Release()

	var eng *syncer.Engine
	var job syncer.Job
	doSync := !opt.NoSync && c.CloudConfigured() && len(game.Slots) > 0
	if doSync {
		if job, err = c.Job(game); err != nil {
			return res, err
		}
		if eng, err = c.Engine(); err != nil {
			return res, err
		}
	}

	lockWritten := make(chan struct{})
	if doSync {
		preCtx, preDone := cancelablePhase(ctx, ui)
		ui.Status("sync", "Checking cloud saves…")
		// The library catalog and the cloud lock don't depend on this game's
		// saves: fetch them while the saves are compared (each round trip
		// costs seconds on some providers).
		libDone := make(chan struct{})
		go func() {
			defer close(libDone)
			if err := c.SyncCatalog(preCtx); err != nil {
				log.Printf("library sync: %v", err)
			}
		}()
		var other *syncer.Lock
		lockChecked := make(chan struct{})
		go func() {
			defer close(lockChecked)
			if l, err := eng.CheckLock(preCtx, game.ID); err == nil {
				other = l
			}
		}()
		action, err := c.syncWithUI(preCtx, eng, game, job, ui, true)
		<-libDone
		<-lockChecked
		if preDone() {
			return res, ErrCancelled // the game isn't launched
		}
		res.PreAction = action
		if err != nil {
			c.States.SetError(game.ID, err.Error())
			if errors.Is(err, ErrCancelled) || !ui.ConfirmSyncFailure(ctx, game, err, true) {
				return res, ErrCancelled
			}
		}
		if other != nil && !ui.ConfirmLock(ctx, game, other) {
			return res, ErrCancelled
		}
		// Mark the game as being played without delaying its start; the
		// release afterwards waits for this write.
		go func() {
			defer close(lockWritten)
			_ = eng.AcquireLock(ctx, game.ID)
		}()
	} else {
		close(lockWritten)
	}

	ui.Status("playing", "Playing "+game.Name)
	ui.GameRunning(true)
	run, runErr := launcher.Run(ctx, spec, nil)
	ui.GameRunning(false)
	res.ExitCode = run.ExitCode

	if doSync {
		// Use a fresh context: the game may have been closed because ctx was cancelled.
		postCtx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
		defer cancel()
		upCtx, upDone := cancelablePhase(postCtx, ui)
		ui.Status("sync", "Uploading saves…")
		action, err := c.syncWithUI(upCtx, eng, game, job, ui, false)
		if upDone() {
			// Saves stay as they are locally; the next sync uploads them.
			action, err = syncer.ActionNone, ErrCancelled
		}
		res.PostAction, res.PostError = action, err
		if err != nil && !errors.Is(err, ErrCancelled) {
			c.States.SetError(game.ID, err.Error())
			ui.ConfirmSyncFailure(postCtx, game, err, false)
		}
		var wg sync.WaitGroup
		wg.Go(func() {
			<-lockWritten
			_ = eng.ReleaseLock(postCtx, game.ID)
		})
		wg.Go(func() {
			if err := c.SyncLibrary(postCtx); err != nil {
				log.Printf("library sync: %v", err)
			}
		})
		wg.Wait()
	}
	if runErr != nil {
		return res, runErr
	}
	return res, nil
}

func (c *Core) syncWithUI(ctx context.Context, eng *syncer.Engine, game library.Game, job syncer.Job, ui SessionUI, before bool) (syncer.Action, error) {
	plan, err := eng.Plan(ctx, job)
	if err != nil {
		return syncer.ActionNone, err
	}
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

func (c *Core) launchSpec(game library.Game, dev library.DeviceConfig, opt RunOptions) (launcher.Spec, error) {
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
