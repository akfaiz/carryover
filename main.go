package main

import (
	"context"
	"embed"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"

	"github.com/akfaiz/playrelay/internal/core"
	"github.com/akfaiz/playrelay/internal/syncer"
	"github.com/akfaiz/playrelay/services"
)

//go:embed all:frontend/dist
var assets embed.FS

const usage = `PlayRelay — game launcher with rclone cloud saves

Usage:
  playrelay                                  open the launcher
  playrelay run <game> [--headless] [-- cmd] sync, play, sync (used by Steam shortcuts)
  playrelay sync <game|--all> [--push|--pull]
  playrelay list                             list games and their ids

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
		log.Fatalf("playrelay: %v", err)
	}
	setupLogFile(c)
	return c
}

// setupLogFile mirrors logs to <data>/logs/playrelay.log (useful when launched from Steam).
func setupLogFile(c *core.Core) {
	f, err := os.OpenFile(filepath.Join(c.Dirs.LogsDir(), "playrelay.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
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
	ctx := context.Background()
	if err := c.SyncLibrary(ctx); err != nil {
		log.Printf("library sync: %v", err)
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
		lock, err := syncer.AcquireLocalLock(c.Dirs.StateDir(), g.ID)
		if err != nil {
			log.Printf("%s: %v", g.Name, err)
			code = 1
			continue
		}
		_, action, err := eng.Sync(ctx, job, res)
		lock.Release()
		switch {
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

func cmdList() int {
	c := openCore()
	for _, g := range c.Library.List() {
		ready := "ready"
		if _, err := c.Job(g); err != nil {
			ready = "not set up on this device"
		}
		fmt.Printf("%s  %-40s %s\n", g.ID, g.Name, ready)
	}
	return 0
}

func runGUI(wrapGameID string, opts core.RunOptions) int {
	c := openCore()
	session := services.NewSessionService(c)
	wrapper := wrapGameID != ""

	appOpts := application.Options{
		Name:        "PlayRelay",
		Description: "Game launcher with rclone cloud saves",
		Services: []application.Service{
			application.NewService(services.NewLibraryService(c, session.IsRunning)),
			application.NewService(services.NewSyncService(c)),
			application.NewService(services.NewCloudService(c)),
			application.NewService(services.NewSteamService(c)),
			application.NewService(session),
		},
		Assets: application.AssetOptions{
			Handler: services.CoversHandler(c, application.AssetFileServerFS(assets)),
		},
		Mac: application.MacOptions{ApplicationShouldTerminateAfterLastWindowClosed: true},
	}
	if !wrapper {
		appOpts.SingleInstance = &application.SingleInstanceOptions{
			UniqueID: "com.akfaiz.playrelay",
			OnSecondInstanceLaunch: func(application.SecondInstanceData) {
				if w := application.Get().Window.Current(); w != nil {
					w.Show()
					w.Focus()
				}
			},
		}
	}
	app := application.New(appOpts)

	winOpts := application.WebviewWindowOptions{
		Title:            "PlayRelay",
		Width:            1180,
		Height:           760,
		MinWidth:         720,
		MinHeight:        480,
		BackgroundColour: application.NewRGB(14, 15, 20),
		URL:              "/",
	}
	if wrapper {
		winOpts.Width, winOpts.Height = 460, 340
		winOpts.MinWidth, winOpts.MinHeight = 0, 0
		winOpts.DisableResize = true
		winOpts.AlwaysOnTop = true
		winOpts.URL = "/#/session"
	}
	win := app.Window.NewWithOptions(winOpts)

	exitCode := 0
	if wrapper {
		done := services.ConfigureWrapper(session, wrapGameID, opts)
		session.OnGameRunning = func(running bool) {
			if running {
				win.Hide()
			} else {
				win.Show()
			}
		}
		session.OnPrompt = func() { win.Show(); win.Focus() }
		go func() {
			out := <-done
			exitCode = out.ExitCode
			if out.Err != nil {
				log.Printf("run %s: %v", wrapGameID, out.Err)
				if exitCode == 0 {
					exitCode = 1
				}
				// Leave the error visible briefly.
				time.Sleep(4 * time.Second)
			} else {
				time.Sleep(1200 * time.Millisecond)
			}
			app.Quit()
		}()
	}

	if err := app.Run(); err != nil {
		log.Print(err)
		return 1
	}
	return exitCode
}
