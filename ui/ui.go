// Package ui is Carryover's desktop GUI, built with Fyne. This package is the
// composition root: it builds the shell (the main window, see package shell)
// and plugs the pages into it.
package ui

import (
	_ "embed"
	"errors"
	"log"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"

	"github.com/akfaiz/carryover/internal/app/singleinstance"
	"github.com/akfaiz/carryover/internal/core"
	"github.com/akfaiz/carryover/services"
	"github.com/akfaiz/carryover/ui/i18n"
	"github.com/akfaiz/carryover/ui/kit"
	"github.com/akfaiz/carryover/ui/pages/session"
	"github.com/akfaiz/carryover/ui/shell"
)

//go:embed assets/appicon.png
var appIcon []byte

// AppID identifies the app (Fyne preferences, single instance).
const AppID = "com.akfaiz.carryover"

// Run opens the GUI and blocks until it closes. With wrapGameID set it runs
// the wrapper session window (carryover run <game>) and returns the game's
// exit code.
func Run(c *core.Core, wrapGameID string, opts core.RunOptions) int {
	if wrapGameID == "" {
		inst, err := singleinstance.Acquire(c.Dirs.StateDir())
		if errors.Is(err, singleinstance.ErrRunning) {
			return 0 // the running instance was asked to show itself
		}
		if err != nil {
			log.Printf("single instance: %v", err)
		}
		if inst != nil {
			defer inst.Close()
		}
		start(c).RunMain(inst)
		return 0
	}
	return start(c).RunWrapper(session.New, wrapGameID, opts)
}

func start(c *core.Core) *shell.Shell {
	i18n.Load(c.Settings.Get().Language)
	a := app.NewWithID(AppID)
	kit.SystemDark.Store(kit.InitialSystemDark(a))
	kit.SetPalette(kit.PaletteFor(c.Settings.Get().Theme))
	a.Settings().SetTheme(kit.AppTheme{})
	a.SetIcon(fyne.NewStaticResource("appicon.png", appIcon))
	sh := shell.New(c, a, services.NewBus(), appConfig())
	kit.WatchSystemTheme(a, func(dark bool) {
		if kit.SystemDark.Swap(dark) == dark {
			return
		}
		kit.OnUI(func() { sh.ApplyTheme(c.Settings.Get().Theme) })
	})
	return sh
}

// appConfig plugs this application's pages and dialogs into the shell.
func appConfig() shell.Config {
	return shell.Config{
		Icon:         appIcon,
		Page:         pageFor,
		AddGame:      showAddGame,
		SyncProblems: showSyncProblems,
	}
}

// NoticeText is the desktop notification for what the session watcher did, in
// the language chosen in Settings (the watcher has no window).
func NoticeText(c *core.Core, n core.Notice) (title, body string) {
	return shell.NoticeText(c, n)
}
