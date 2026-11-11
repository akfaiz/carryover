package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/akfaiz/carryover/internal/core"
	"github.com/akfaiz/carryover/internal/syncer"
	"github.com/akfaiz/carryover/ui"
)

const usage = `Carryover — game launcher with rclone cloud saves

Usage:
  carryover                                  open the launcher
  carryover run <game> [--headless] [-- cmd] sync, play, sync (used by Steam shortcuts)
  carryover sync <game|--all> [--push|--pull] [--dry-run] [--yes]
                                             --dry-run shows what would happen; with --all,
                                             --push/--pull ask first unless --yes is given
  carryover list                             list games and their ids

<game> is a game id or its exact name. With "-- cmd" (e.g. Steam's
"-- %command%") the given command is launched instead of the configured exe.
`

func main() {
	attachConsole()
	args := os.Args[1:]
	if len(args) == 0 {
		os.Exit(runGUI("", core.RunOptions{}))
	}
	switch args[0] {
	case "run":
		os.Exit(cmdRun(args[1:]))
	case "sync":
		os.Exit(cmdSync(args[1:]))
	case "list":
		os.Exit(cmdList())
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
		log.Fatalf("carryover: %v", err)
	}
	setupLogFile(c)
	return c
}

// setupLogFile mirrors logs to <data>/logs/carryover.log (useful when launched from Steam).
func setupLogFile(c *core.Core) {
	f, err := os.OpenFile(filepath.Join(c.Dirs.LogsDir(), "carryover.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	if fi, err := f.Stat(); err == nil && fi.Size() > 5<<20 {
		_ = f.Truncate(0)
	}
	log.SetOutput(io.MultiWriter(os.Stderr, f))
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
	if err := fset.Parse(reorder(args)); err != nil || fset.NArg() != 1 {
		_, _ = os.Stderr.WriteString(usage)
		return 2
	}
	opts := core.RunOptions{Command: command, NoSync: *noSync}
	gameID := fset.Arg(0)
	survivePolite()
	if *headless {
		c := openCore()
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
func reorder(args []string) []string {
	var flags, pos []string
	for _, a := range args {
		if strings.HasPrefix(a, "-") {
			flags = append(flags, a)
		} else {
			pos = append(pos, a)
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
	if err := fset.Parse(reorder(args)); err != nil || (*push && *pull) || (fset.NArg() != 1 && !*all) {
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
		if n := readyGames(c, games); n > 0 && !confirmForce(*push, n) {
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
		job, err := c.Job(g)
		if err != nil {
			if !*all {
				log.Printf("%s: %v", g.Name, err)
				code = 1
			}
			continue
		}
		fmt.Printf("%s\n", g.Name)
		if *dryRun {
			if rc := dryRunGame(ctx, eng, job, res); rc > code {
				code = rc
			}
			if ctx.Err() != nil {
				return 130
			}
			continue
		}
		lock, err := syncer.AcquireLocalLock(c.Dirs.StateDir(), g.ID)
		if err != nil {
			log.Printf("%s: %v", g.Name, err)
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
			log.Printf("%s: %v", g.Name, err)
			c.States.SetError(g.ID, err.Error())
			code = 1
		default:
			fmt.Printf("  %s\n", action)
		}
	}
	return code
}

// readyGames counts the games (ids) that are set up on this device.
func readyGames(c *core.Core, ids []string) int {
	n := 0
	for _, id := range ids {
		if g, err := c.Library.Find(id); err == nil {
			if _, err := c.Job(g); err == nil {
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
		ready := "ready"
		job, err := c.Job(g)
		if err != nil {
			ready = "not set up on this device"
		} else if st, err := c.States.Load(g.ID); err == nil {
			if changed, _, err := syncer.LocalChanges(context.Background(), job.Slots, st.LastSynced, nil); err == nil && changed {
				ready = "ready, saves changed since the last sync (not uploaded)"
			}
		}
		fmt.Printf("%s  %-40s %s\n", g.ID, g.Name, ready)
	}
	return 0
}

func runGUI(wrapGameID string, opts core.RunOptions) int {
	return ui.Run(openCore(), wrapGameID, opts)
}
