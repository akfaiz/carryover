package core

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/akfaiz/carryover/internal/cloudsync/syncer"
	"github.com/akfaiz/carryover/internal/launcher"
	"github.com/akfaiz/carryover/internal/library"
	"github.com/akfaiz/carryover/internal/stores/heroic"
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

	emuID, err := c.ResolveProfile(game, opt.Emulator)
	if err != nil {
		return res, err
	}
	key := SyncKey(game.ID, emuID) // this profile's own sync state
	ckey := CloudKey(game, emuID)  // the cloud folder, locks and session marker; shared by emulators sharing saves
	spec, err := c.launchSpec(game, dev, opt, emuID)
	if err != nil {
		log.Printf("play %s: %v", key, err)
		return res, err
	}

	lock, err := syncer.AcquireLocalLock(c.Dirs.StateDir(), ckey)
	if err != nil {
		log.Printf("play %s: %v", key, err)
		return res, err
	}
	defer lock.Release()
	c.playing.Store(ckey, struct{}{})
	defer c.playing.Delete(ckey)

	var eng *syncer.Engine
	var job syncer.Job
	slots, _, _ := c.profileData(game, emuID)
	doSync := !opt.NoSync && c.CloudConfigured() && len(slots) > 0
	log.Printf("play %s (%s): profile %s, launch %s, sync %s", key, game.Name, profileLabel(emuID), launchLabel(dev, opt, emuID), syncLabel(opt, c.CloudConfigured(), len(slots)))
	if doSync {
		if job, err = c.JobFor(game, emuID); err != nil {
			log.Printf("play %s: %v", key, err)
			return res, err
		}
		if eng, err = c.Engine(); err != nil {
			log.Printf("play %s: %v", key, err)
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
			if l, err := eng.CheckLock(preCtx, ckey); err == nil {
				other = l
			}
		}()
		plan, err := eng.Plan(preCtx, job)
		<-libDone
		<-lockChecked
		if preCtx.Err() == nil {
			// The catalog may have brought new save locations or filters for this
			// game. A plan made with the old ones could upload or delete files the
			// new ones treat differently, so plan again with the new job.
			if g, gerr := c.Library.Get(game.ID); gerr == nil {
				if j, jerr := c.JobFor(g, emuID); jerr == nil && !reflect.DeepEqual(j, job) {
					game, job = g, j
					plan, err = eng.Plan(preCtx, job)
				}
			}
		}
		action := syncer.ActionNone
		if err == nil {
			action, err = c.resolveAndApply(preCtx, eng, game, plan, ui, true)
		}
		if preDone() || ctx.Err() != nil {
			// Cancelled by the user, or the whole session was: not a sync error,
			// and the game isn't launched.
			log.Printf("play %s: cancelled before launch", key)
			return res, ErrCancelled
		}
		res.PreAction = action
		if err != nil {
			c.States.SetError(key, err.Error())
			if errors.Is(err, ErrCancelled) || !ui.ConfirmSyncFailure(ctx, game, err, true) {
				log.Printf("play %s: not launched after the sync before playing failed: %v", key, err)
				return res, ErrCancelled
			}
			log.Printf("play %s: launching although the sync before playing failed", key)
		}
		if other != nil {
			log.Printf("play %s: cloud lock held by another device since %s", key, other.Since.UTC().Format(time.RFC3339))
			if !ui.ConfirmLock(ctx, game, other) {
				log.Printf("play %s: not launched (locked by another device)", key)
				return res, ErrCancelled
			}
		}
		// Mark the game as being played without delaying its start; the
		// release afterwards waits for this write. The marker lets a later
		// run release the lock if this process is killed (RecoverSessions),
		// and the session watcher upload the saves (FinishSession).
		c.markSession(ckey, game.ID, emuID)
		if c.WatchSession != nil {
			go c.WatchSession(ckey)
		}
		go func() {
			defer close(lockWritten)
			_ = eng.AcquireLock(ctx, ckey)
		}()
	} else {
		close(lockWritten)
	}

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
	res.ExitCode = run.ExitCode
	if runErr != nil {
		log.Printf("play %s: game ended after %s, exit code %d: %v", key, time.Since(started).Round(time.Second), run.ExitCode, runErr)
	} else {
		log.Printf("play %s: game ended after %s, exit code %d", key, time.Since(started).Round(time.Second), run.ExitCode)
	}

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
			log.Printf("play %s: upload after playing cancelled by the user", key)
		}
		res.PostAction, res.PostError = action, err
		if err != nil && !errors.Is(err, ErrCancelled) {
			c.States.SetError(key, err.Error())
			ui.ConfirmSyncFailure(postCtx, game, err, false)
		}
		var wg sync.WaitGroup
		wg.Go(func() {
			<-lockWritten
			if err := eng.ReleaseLock(postCtx, ckey); err == nil {
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
	if runErr != nil {
		return res, runErr
	}
	return res, nil
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
	if dev.LaunchMode == library.LaunchHeroic {
		return heroicSpec(spec, exe)
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

// heroicAppear is how long a Heroic launch may take to start the game: Heroic
// may first sync its own cloud saves or download a Wine/Proton build.
const heroicAppear = 5 * time.Minute

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
	spec.Detach, spec.WaitAppear = true, heroicAppear
	return spec, nil
}
