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
	"fyne.io/fyne/v2/lang"

	"github.com/akfaiz/carryover/internal/app/singleinstance"
	"github.com/akfaiz/carryover/internal/core"
	"github.com/akfaiz/carryover/services"
	"github.com/akfaiz/carryover/ui/flows"
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
	language := c.Settings.Get().Language
	i18n.Load(language)
	fyneTranslations(language)
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

// fyneTranslations hands the catalogs to Fyne's lang package too, for the text
// of Fyne's own widgets and dialogs ("Cancel", the file dialog). With a
// language code only that catalog is registered, under the base locale "en":
// Fyne has no way to force a locale, but whatever the system language is, the
// base one is what it falls back to, so the chosen catalog always applies.
func fyneTranslations(setting string) {
	if data, ok := i18n.Catalog(setting); ok {
		err := lang.AddTranslationsForLocale(data, fyne.Locale("en"))
		if err == nil {
			return
		}
		log.Printf("translations: %s: %v", setting, err)
	}
	if err := lang.AddTranslationsFS(i18n.FS(), i18n.Dir); err != nil {
		log.Printf("translations: %v", err)
	}
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

// NoticeText is the desktop notification for what the session watcher did, in
// the language chosen in Settings (the watcher has no window).
func NoticeText(c *core.Core, n core.Notice) (title, body string) {
	return shell.NoticeText(c, n)
}
