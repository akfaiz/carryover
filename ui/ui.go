// Package ui is Carryover's desktop GUI, built with MyGo. This package is the
// composition root: it builds the shell (the main window, see package shell)
// and plugs the pages into it.
package ui

import (
	"errors"
	"log"

	"github.com/akfaiz/carryover/internal/app/singleinstance"
	"github.com/akfaiz/carryover/internal/core"
	"github.com/akfaiz/carryover/services"
	"github.com/akfaiz/carryover/ui/env"
	"github.com/akfaiz/carryover/ui/flows"
	"github.com/akfaiz/carryover/ui/i18n"
	"github.com/akfaiz/carryover/ui/kit"
	"github.com/akfaiz/carryover/ui/pages/session"
	"github.com/akfaiz/carryover/ui/shell"
)

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
	// The palette follows the system once the app runs (BuildMain asks it); the
	// chosen one applies from the first frame.
	kit.SetPalette(kit.PaletteFor(c.Settings.Get().Theme))
	return shell.New(c, services.NewBus(), appConfig())
}

// appConfig plugs this application's pages and dialogs into the shell.
func appConfig() shell.Config {
	return shell.Config{
		Icon:         appIcon,
		Page:         pageFor,
		AddGame:      flows.ShowAddGame,
		SyncProblems: showSyncProblems,
	}
}

// showSyncProblems says how many games a "Sync all" could not sync. The list
// of them (each opening its page) comes with the dialogs.
func showSyncProblems(u env.Env, _ int, problems []services.SyncResult) {
	u.Toast(i18n.N("{{.Count}} game needs attention", len(problems), map[string]any{"Count": len(problems)}), kit.ToastErr)
}
