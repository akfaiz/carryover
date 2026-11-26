package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/akfaiz/carryover/internal/app/update"
	"github.com/akfaiz/carryover/internal/core"
	"github.com/akfaiz/carryover/services"
)

// cmdUpdate looks for a newer version and installs it, for use without the
// window (over SSH, or from a script).
func cmdUpdate(args []string) int {
	fset := flag.NewFlagSet("update", flag.ContinueOnError)
	checkOnly := fset.Bool("check", false, "only say whether there is a newer version")
	channelFlag := fset.String("channel", "", "\"stable\" or \"prerelease\" (default: the setting)")
	if err := fset.Parse(reorderFlags(args, "channel")); err != nil || fset.NArg() != 0 {
		_, _ = os.Stderr.WriteString(usage)
		return 2
	}
	var channel *update.Channel
	switch *channelFlag {
	case "":
	case "stable":
		channel = new(update.Stable)
	case string(update.Prerelease):
		channel = new(update.Prerelease)
	default:
		_, _ = os.Stderr.WriteString(usage)
		return 2
	}
	c := openCore()
	exe, err := core.SelfExecutable()
	if err != nil {
		log.Printf("update: %v", err)
	}
	bus := services.NewBus()
	percent := -1
	bus.OnUpdate(func(st services.UpdateState) {
		if st.Status != services.UpdateDownloading || st.Total <= 0 {
			return
		}
		// A line every 10%, which also reads well in a log.
		if p := int(st.Done * 100 / st.Total / 10 * 10); p > percent {
			percent = p
			fmt.Printf("downloading: %d%%\n", p)
		}
	})
	target := update.DesktopTarget(exe)
	s := services.NewUpdateService(c, bus, services.UpdateOptions{Target: target, Channel: channel})
	if !s.State().Supported {
		fmt.Println("This is a development build: it doesn't update itself.")
		return 1
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	fail := func(err error) int {
		log.Printf("update: %v", err)
		_, _ = fmt.Fprintln(os.Stderr, "carryover: "+err.Error())
		return 1
	}
	st, err := s.Check(ctx, true)
	if err != nil {
		return fail(err)
	}
	if !st.Newer() {
		fmt.Printf("Carryover %s is up to date.\n", st.Current)
		return 0
	}
	fmt.Printf("Version %s is available (this is %s): %s\n", st.Latest, st.Current, st.URL)
	if *checkOnly {
		return 0
	}
	if !st.Installable {
		return fail(update.ErrUnsupported)
	}
	if _, err := s.Download(ctx); err != nil {
		return fail(err)
	}
	finish, err := s.Install(ctx)
	if err == nil {
		err = finish(false)
	}
	if err != nil {
		if errors.Is(err, services.ErrUpdateBusy) {
			fmt.Println("The update is downloaded; run this again once the game or sync is done.")
		}
		return fail(err)
	}
	if target.Method == update.MethodInstaller {
		// The setup program waits a while for this process, and the window, to exit.
		fmt.Printf("Installing %s. Close Carryover now if its window is open.\n", st.Latest)
		return 0
	}
	fmt.Printf("Updated to %s. Restart Carryover if its window is open.\n", st.Latest)
	return 0
}
