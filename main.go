package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"time"

	"github.com/akfaiz/carryover/internal/app/appdir"
	"github.com/akfaiz/carryover/internal/app/applog"
	"github.com/akfaiz/carryover/internal/app/detach"
	"github.com/akfaiz/carryover/internal/app/notify"
	"github.com/akfaiz/carryover/internal/app/version"
	"github.com/akfaiz/carryover/internal/cloudsync/syncer"
	"github.com/akfaiz/carryover/internal/core"
	"github.com/akfaiz/carryover/internal/library"
	"github.com/akfaiz/carryover/ui"
)

const usage = `Carryover — game launcher with rclone cloud saves

Usage:
  carryover                                  open the launcher
  carryover run <game> [--emulator <e>] [--headless] [--no-sync] [-- cmd]
                                             sync, play, sync (used by Steam shortcuts)
  carryover sync <game|--all> [--emulator <e>] [--push|--pull] [--dry-run] [--yes]
                                             --dry-run shows what would happen; with --all,
                                             --push/--pull ask first unless --yes is given
  carryover list                             list games and their ids
  carryover --version                        print the version

<game> is a game id or its exact name. With "-- cmd" (e.g. Steam's
"-- %command%") the given command is launched instead of the configured exe.
A game can be played with several emulators, each with its own cloud save.
<e> is an emulator id or name, or "native" for the game's own launch config.
"run" without --emulator uses the profile played last on this device; "sync"
without it syncs every profile set up on this device.
`

func main() {
	attachConsole()
	args := os.Args[1:]
	if len(args) == 0 {
		applog.Exit(runGUI("", core.RunOptions{}))
	}
	switch args[0] {
	case "run":
		applog.Exit(cmdRun(args[1:]))
	case "sync":
		applog.Exit(cmdSync(args[1:]))
	case "list":
		applog.Exit(cmdList())
	case finishSessionCmd: // internal: started by a play session, see watchSessions
		applog.Exit(cmdFinishSession(args[1:]))
	case "--version", "version":
		fmt.Println("carryover " + version.Get().String())
	case "-h", "--help", "help":
		_, _ = os.Stdout.WriteString(usage)
	default:
		_, _ = os.Stderr.WriteString(usage)
		os.Exit(2)
	}
}

func openCore() *core.Core {
	c, err := core.New()
	if err != nil {
		startupFailed(err)
	}
	setupLogFile(c)
	return c
}

// startupFailed reports an error that stops Carryover before its log is open
// (e.g. an unreadable settings file): to the log, if the data dir can be
// found, and on stderr or, for a Windows GUI launch, in a message box.
func startupFailed(err error) {
	logged := ""
	if d, derr := appdir.Resolve(); derr == nil && applog.WriteStartupError(d.LogsDir(), err) == nil {
		logged = filepath.Join(d.LogsDir(), applog.FileName)
	}
	msg := fmt.Sprintf("Carryover could not start:\n\n%v", err)
	if logged != "" {
		msg += "\n\nDetails were written to " + logged
	}
	_, _ = fmt.Fprintln(os.Stderr, "carryover: "+err.Error())
	showFatal(msg)
	os.Exit(1)
}

// logFile is the open log, kept for the life of the process.
var logFile *applog.Log

// closeLogFile closes the log file. The process normally keeps it open until
// it exits; tests that run commands in-process close it so the data dir can
// be deleted (Windows refuses to delete an open file).
func closeLogFile() {
	if logFile != nil {
		logFile.Close()
		logFile = nil
	}
}

// setupLogFile mirrors logs to <data>/logs/carryover.log (useful when launched
// from Steam) and records crashes there.
func setupLogFile(c *core.Core) {
	closeLogFile()
	// The header is only in the file: it tells which build wrote the log
	// (e.g. one sent with a bug report).
	data := "default data dir"
	if os.Getenv(appdir.EnvHome) != "" {
		data = appdir.EnvHome + " data dir"
	}
	l, err := applog.Open(c.Dirs.LogsDir(), fmt.Sprintf("Carryover %s started, args %q, %s", version.Get(), os.Args[1:], data))
	if err != nil {
		return
	}
	logFile = l
}

func cmdRun(args []string) int {
	var command []string
	for i, a := range args {
		if a == "--" {
			command = args[i+1:]
			args = args[:i]
			break
		}
	}
	fset := flag.NewFlagSet("run", flag.ContinueOnError)
	headless := fset.Bool("headless", false, "don't open a window; conflicts are skipped")
	noSync := fset.Bool("no-sync", false, "launch without syncing")
	emulator := fset.String("emulator", "", "emulator id or name, or \"native\"")
	if err := fset.Parse(reorderFlags(args, "emulator")); err != nil || fset.NArg() != 1 {
		_, _ = os.Stderr.WriteString(usage)
		return 2
	}
	opts := core.RunOptions{Command: command, NoSync: *noSync, Emulator: *emulator}
	gameID := fset.Arg(0)
	survivePolite()
	if *headless {
		c := openCore()
		watchSessions(c)
		recoverSessions(c)
		log.Printf("run %s (headless)", gameID)
		res, err := c.Play(context.Background(), gameID, opts, core.HeadlessUI{Logf: log.Printf})
		if err != nil {
			log.Printf("run %s: %v", gameID, err)
			if res.ExitCode == 0 {
				return 1
			}
		}
		return res.ExitCode
	}
	return runGUI(gameID, opts)
}

// survivePolite keeps the wrapper alive when it is asked to terminate (SIGTERM/
// SIGHUP, e.g. Steam's "Stop" button signals the whole process tree). The game
// receives the same signal and exits by itself; the wrapper must outlive it to
// run the post-play sync. Steam may still SIGKILL the tree after a grace period.
func survivePolite() {
	ch := make(chan os.Signal, 4)
	signal.Notify(ch, syscall.SIGTERM, syscall.SIGHUP)
	go func() {
		for s := range ch {
			log.Printf("received %v: waiting for the game to exit, then syncing", s)
		}
	}()
}

// reorder moves flags before positional args so "run <id> --headless" works.
func reorder(args []string) []string { return reorderFlags(args) }

// reorderFlags is reorder for commands with flags that take a value
// (valued): "--emulator X" and "--emulator=X" both keep X with its flag.
func reorderFlags(args []string, valued ...string) []string {
	takesValue := func(a string) bool {
		name := strings.TrimLeft(a, "-")
		return !strings.Contains(a, "=") && slices.Contains(valued, name)
	}
	var flags, pos []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case !strings.HasPrefix(a, "-"):
			pos = append(pos, a)
		case takesValue(a) && i+1 < len(args):
			flags = append(flags, a, args[i+1])
			i++
		default:
			flags = append(flags, a)
		}
	}
	return append(flags, pos...)
}

func cmdSync(args []string) int {
	fset := flag.NewFlagSet("sync", flag.ContinueOnError)
	push := fset.Bool("push", false, "force upload (keep local)")
	pull := fset.Bool("pull", false, "force download (keep cloud)")
	all := fset.Bool("all", false, "sync every game set up on this device")
	dryRun := fset.Bool("dry-run", false, "show what would happen without changing anything")
	yes := fset.Bool("yes", false, "don't ask before forcing --push/--pull on every game")
	fset.BoolVar(yes, "y", false, "same as --yes")
	emulator := fset.String("emulator", "", "sync only this emulator's saves (id, name or \"native\")")
	if err := fset.Parse(reorderFlags(args, "emulator")); err != nil || (*push && *pull) || (fset.NArg() != 1 && !*all) {
		_, _ = os.Stderr.WriteString(usage)
		return 2
	}
	c := openCore()
	c.SetEventHandler(func(ev syncer.Event) {
		if ev.Phase != "upload" && ev.Phase != "download" && ev.Phase != "backup" || ev.Bytes == 0 {
			fmt.Printf("  %s\n", ev.Message)
		}
	})
	res := syncer.ResolveAuto
	if *push {
		res = syncer.ResolveLocal
	} else if *pull {
		res = syncer.ResolveCloud
	}
	// Ctrl+C cancels cleanly: transfers stop and partial backups are removed.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	if !*dryRun { // a dry run changes nothing, not even the cloud library
		if err := c.SyncLibrary(ctx); err != nil {
			log.Printf("library sync: %v", err)
		}
	}
	if !*dryRun {
		recoverSessions(c)
	}
	eng, err := c.Engine()
	if err != nil {
		log.Print(err)
		return 1
	}
	var games []string
	if *all {
		for _, g := range c.Library.List() {
			games = append(games, g.ID)
		}
	} else {
		games = []string{fset.Arg(0)}
	}
	if *all && res != syncer.ResolveAuto && !*dryRun && !*yes {
		if n := readyGames(c, games, *emulator); n > 0 && !confirmForce(*push, n) {
			fmt.Println("aborted; nothing was changed")
			return 1
		}
	}
	code := 0
	for _, id := range games {
		g, err := c.Library.Find(id)
		if err != nil {
			log.Printf("%s: %v", id, err)
			code = 1
			continue
		}
		profiles, err := syncProfiles(c, g, *emulator)
		if err != nil {
			log.Printf("%s: %v", g.Name, err)
			code = 1
			continue
		}
		for _, emuID := range profiles {
			name := profileTitle(c, g, emuID)
			job, err := c.JobFor(g, emuID)
			if err != nil {
				if !*all {
					log.Printf("%s: %v", name, err)
					code = 1
				}
				continue
			}
			fmt.Printf("%s\n", name)
			if *dryRun {
				if rc := dryRunGame(ctx, eng, job, res); rc > code {
					code = rc
				}
				if ctx.Err() != nil {
					return 130
				}
				continue
			}
			lock, err := syncer.AcquireLocalLock(c.Dirs.StateDir(), job.GameID)
			if err != nil {
				log.Printf("%s: %v", name, err)
				code = 1
				continue
			}
			_, action, err := eng.Sync(ctx, job, res)
			lock.Release()
			switch {
			case ctx.Err() != nil:
				fmt.Println("  cancelled")
				return 130
			case errors.Is(err, syncer.ErrConflict):
				fmt.Println("  conflict: both local and cloud changed; rerun with --push or --pull")
				code = 3
			case err != nil:
				log.Printf("%s: %v", name, err)
				c.States.SetError(job.StateID(), err.Error())
				code = 1
			default:
				fmt.Printf("  %s\n", action)
			}
		}
	}
	return code
}

// syncProfiles lists the emulator ids ("" = native) to sync for a game: the
// one asked for, else every profile set up on this device (the native one if
// there is none, so its "not set up" error is reported as before).
func syncProfiles(c *core.Core, g library.Game, want string) ([]string, error) {
	if want != "" {
		id, err := c.ResolveProfile(g, want)
		return []string{id}, err
	}
	if p := c.Profiles(g); len(p) > 0 {
		return p, nil
	}
	return []string{""}, nil
}

// profileTitle is the game's name, plus the emulator's for an emulator profile.
func profileTitle(c *core.Core, g library.Game, emuID string) string {
	if emuID == "" {
		return g.Name
	}
	return g.Name + " (" + c.ProfileName(emuID) + ")"
}

// readyGames counts the profiles of the games (ids) that are set up on this device.
func readyGames(c *core.Core, ids []string, emulator string) int {
	n := 0
	for _, id := range ids {
		g, err := c.Library.Find(id)
		if err != nil {
			continue
		}
		profiles, err := syncProfiles(c, g, emulator)
		if err != nil {
			continue
		}
		for _, emuID := range profiles {
			if _, err := c.JobFor(g, emuID); err == nil {
				n++
			}
		}
	}
	return n
}

// confirmForce asks before overwriting one side for many games. Without a
// terminal to ask on it refuses: pass --yes to force in scripts.
func confirmForce(push bool, n int) bool {
	what, over := "download the cloud saves of", "local saves"
	if push {
		what, over = "upload the local saves of", "the cloud saves"
	}
	refuse := func() bool {
		fmt.Fprintf(os.Stderr, "refusing to %s %d games without confirmation; rerun with --yes (or --dry-run to preview)\n", what, n)
		return false
	}
	if fi, err := os.Stdin.Stat(); err != nil || fi.Mode()&os.ModeCharDevice == 0 {
		return refuse()
	}
	fmt.Printf("This will %s %d games, overwriting %s even where they differ.\nTip: --dry-run shows what would happen. Continue? [y/N] ", what, n, over)
	line, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil && strings.TrimSpace(line) == "" {
		// No answer could be read (e.g. NUL, which Windows reports as a
		// character device): treat it like having no terminal.
		fmt.Println()
		return refuse()
	}
	line = strings.ToLower(strings.TrimSpace(line))
	return line == "y" || line == "yes"
}

// dryRunGame prints what syncing a game would do, without changing anything.
// It returns the exit code (1 on error, 3 on an unresolved conflict).
func dryRunGame(ctx context.Context, eng *syncer.Engine, job syncer.Job, res syncer.Resolution) int {
	plan, err := eng.Plan(ctx, job)
	if err != nil {
		log.Printf("  %v", err)
		return 1
	}
	action, err := plan.ActionFor(res)
	if err != nil {
		fmt.Printf("  %v\n", err)
		return 1
	}
	local, remote := plan.LocalSummary, plan.RemoteSummary
	fmt.Printf("  would %s (local: %s; cloud: %s)\n", dryRunVerb(action), describeSummary(local, true), describeSummary(remote, plan.HasRemote))
	if action == syncer.ActionConflict {
		fmt.Println("  both sides changed; rerun with --push or --pull to choose")
		return 3
	}
	return 0
}

func dryRunVerb(a syncer.Action) string {
	switch a {
	case syncer.ActionPush:
		return "upload local saves to the cloud"
	case syncer.ActionPull:
		return "download cloud saves, replacing local"
	case syncer.ActionConflict:
		return "report a conflict"
	}
	return "do nothing, saves are up to date"
}

func describeSummary(s syncer.Summary, exists bool) string {
	if !exists {
		return "none"
	}
	return fmt.Sprintf("%d files, %s", s.Files, humanSize(s.Bytes))
}

func humanSize(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for m := n / unit; m >= unit; m /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(n)/float64(div), "KMGTPE"[exp])
}

func cmdList() int {
	c := openCore()
	for _, g := range c.Library.List() {
		profiles, _ := syncProfiles(c, g, "")
		for _, emuID := range profiles {
			ready := "ready"
			job, err := c.JobFor(g, emuID)
			if err != nil {
				ready = "not set up on this device"
			} else if st, err := c.States.Load(job.StateID()); err == nil {
				if changed, _, err := syncer.LocalChanges(context.Background(), job.Slots, st.LastSynced, nil); err == nil && changed {
					ready = "ready, saves changed since the last sync (not uploaded)"
				}
			}
			fmt.Printf("%s  %-40s %s\n", g.ID, profileTitle(c, g, emuID), ready)
		}
	}
	return 0
}

func runGUI(wrapGameID string, opts core.RunOptions) int {
	c := openCore()
	watchSessions(c)
	if wrapGameID != "" { // the launcher recovers on its own start
		recoverSessions(c)
	}
	return ui.Run(c, wrapGameID, opts)
}

// recoverSessions releases the cloud locks of play sessions that were killed
// before they could release them. Without a killed session it does no I/O.
func recoverSessions(c *core.Core) {
	if _, err := c.RecoverSessions(context.Background()); err != nil {
		log.Printf("recover sessions: %v", err)
	}
}

// finishSessionCmd is the internal command of the session watcher.
const finishSessionCmd = "finish-session"

const (
	// watchPoll is how often the session watcher checks whether the session
	// it watches is still running.
	watchPoll = 2 * time.Second
	// watchSettle is how long the watcher waits after the session's process
	// is gone before uploading: the game may still be shutting down (Proton
	// flushing files) after Steam killed the wrapper.
	watchSettle = 5 * time.Second
	// watchMax bounds a watcher whose session never seems to end (e.g. the
	// wrapper's process id was reused by another process).
	watchMax = 72 * time.Hour
)

// startDetached starts the session watcher (replaced in tests, whose binary
// can't run it).
var startDetached = detach.Start

// watchSessions makes every play session of c start a session watcher: a
// Carryover process outside the game's process tree that finishes the session
// (uploads the saves, releases the cloud lock) if this process is killed with
// the game, as Steam's "Exit game" does.
func watchSessions(c *core.Core) {
	// Not os.Executable(): in an AppImage that is a file in the AppImage's
	// FUSE mount, which goes away when Steam kills this process, and the
	// watcher then dies with SIGBUS on its next page of code. The AppImage
	// file itself mounts its own copy.
	exe, err := core.SelfExecutable()
	if err != nil {
		log.Printf("session watcher: %v", err)
		return
	}
	// The watcher doesn't inherit this environment (systemd-run and WMI start
	// it from the user's systemd instance or the WMI service): pass the data
	// dir explicitly.
	home := c.Dirs.Root
	c.WatchSession = func(key string) {
		if err := startDetached(exe, []string{finishSessionCmd, "--home", home, key}); err != nil {
			log.Printf("session watcher %s: %v", key, err)
		}
	}
}

// cmdFinishSession is the session watcher: it waits until the play session of
// the given cloud key is no longer running, then finishes it if its wrapper
// was killed (see core.Core.FinishSession). A session that ended normally
// needs nothing.
func cmdFinishSession(args []string) int {
	fset := flag.NewFlagSet(finishSessionCmd, flag.ContinueOnError)
	home := fset.String("home", "", "data dir of the session")
	if err := fset.Parse(reorderFlags(args, "home")); err != nil || fset.NArg() != 1 {
		_, _ = os.Stderr.WriteString(usage)
		return 2
	}
	if *home != "" {
		_ = os.Setenv(appdir.EnvHome, *home)
	}
	key := fset.Arg(0)
	c := openCore()
	log.Printf("session watcher %s: waiting for the session to end (%s)", key, detach.Where())
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	deadline := time.Now().Add(watchMax)
	for {
		finished, notice, err := c.FinishSession(ctx, key, watchSettle)
		if notice != nil {
			tellUser(c, *notice)
		}
		switch {
		case errors.Is(err, syncer.ErrLocked): // still being played
			if time.Now().After(deadline) {
				log.Printf("session watcher %s: gave up, the session is still running after %s", key, watchMax)
				return 1
			}
			select {
			case <-ctx.Done():
				return 130
			case <-time.After(watchPoll):
			}
			continue
		case ctx.Err() != nil:
			return 130
		case err != nil:
			log.Printf("session watcher %s: %v", key, err)
			return 1
		}
		if finished {
			log.Printf("session watcher %s: finished the interrupted session", key)
		}
		return 0
	}
}

// notifySend shows a desktop notification (replaced in tests).
var notifySend = notify.Send

// tellUser tells the user what the session watcher did: as a desktop
// notification, and, where none can be seen, saved for Carryover to show the
// next time it opens. Steam Deck Game Mode (gamescope) has no notification
// service, and one that accepts the call there may still not show anything.
func tellUser(c *core.Core, n core.Notice) {
	title, body := ui.NoticeText(c, n)
	err := notifySend(notify.App{ID: ui.AppID, Name: "Carryover"}, title, body)
	if err != nil {
		log.Printf("notification: %v", err)
	}
	if err != nil || os.Getenv("XDG_CURRENT_DESKTOP") == "gamescope" {
		if err := c.SaveNotice(n); err != nil {
			log.Printf("save notice: %v", err)
		}
	}
}
