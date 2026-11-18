package shell

import (
	"log"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"

	"github.com/akfaiz/carryover/internal/core"
	"github.com/akfaiz/carryover/services"
	"github.com/akfaiz/carryover/ui/env"
	"github.com/akfaiz/carryover/ui/kit"
)

// padFocus is where a gamepad starts on the open page, nil when it doesn't say.
func (u *Shell) padFocus() fyne.Focusable {
	if p, ok := u.page.(env.PadFocuser); ok {
		return p.DefaultFocus()
	}
	return nil
}

// RunWrapper shows the small session window, runs pull → play → push and
// quits when done. It returns the game's exit code.
func (u *Shell) RunWrapper(newPage func(e env.Env) env.Page, gameID string, opts core.RunOptions) int {
	done := services.ConfigureWrapper(u.sess, gameID, opts)
	u.win = u.app.NewWindow("Carryover")
	u.win.SetMaster()
	u.win.SetFixedSize(true)
	u.win.Resize(fyne.NewSize(460, 340))
	u.win.CenterOnScreen()

	page := newPage(u)
	u.page = page
	// The prompts and Cancel work with the controller the shortcut was started
	// with; while the game runs, the controller is the game's.
	u.nav = kit.NewNavigator(u.win, kit.NavigatorOptions{DefaultFocus: u.padFocus})
	u.prompts.Wrapper = true
	u.win.SetContent(container.NewStack(page.View(), u.toasts.Layer, u.nav.RingLayer()))
	u.nav.SetActive(true)

	closer := &wrapperClose{gameID: gameID, quit: u.app.Quit}
	u.win.SetCloseIntercept(closer.request)
	u.sess.OnGameRunning = func(running bool) {
		if running {
			closer.started.Store(true)
			closer.holdSignals()
		}
		kit.OnUI(func() {
			u.nav.SetActive(!running)
			if running {
				u.win.Hide()
			} else {
				u.win.Show()
			}
		})
	}
	u.sess.OnPrompt = func() { kit.OnUI(func() { u.win.Show(); u.win.RequestFocus(); u.nav.SetActive(true) }) }

	exitCode := 0
	u.app.Lifecycle().SetOnStarted(func() {
		kit.SetTopmost(u.win)
		u.showNotices()
		u.sess.Start()
		go func() {
			out := <-done
			exitCode = out.ExitCode
			if out.Err != nil {
				log.Printf("run %s: %v", gameID, out.Err)
				if exitCode == 0 {
					exitCode = 1
				}
				// Leave the error visible briefly.
				time.Sleep(4 * time.Second)
			} else {
				time.Sleep(1200 * time.Millisecond)
			}
			kit.OnUI(u.app.Quit)
		}()
	})
	u.win.ShowAndRun()
	return exitCode
}

// wrapperClose decides what closing the session window does. Once the game
// has started, closing doesn't quit: the upload after playing and the release
// of the cloud lock happen in this process. Steam's "Stop" on Windows closes
// this window (WM_CLOSE) while the game runs, and on macOS asks the app to
// quit, which reaches here as a close request too; the session then finishes
// and quits by itself (signals: holdSignals). Before the game starts, closing
// quits.
type wrapperClose struct {
	gameID  string
	quit    func()
	started atomic.Bool
	hold    sync.Once
}

// holdSignals keeps a SIGINT, SIGTERM or SIGHUP from quitting the wrapper once
// the game runs. Fyne's driver catches SIGINT and SIGTERM and quits the app,
// past the close intercept: on macOS, Steam's Stop asks the app to quit and
// then signals it, so the wrapper quit before uploading. signal.Ignore undoes
// every earlier Notify (Fyne's too, which is installed at the latest once the
// app has started, long before the game does); the signals are then only
// logged, and the session finishes when the game exits. If Steam kills the
// wrapper instead, the session watcher finishes it.
func (w *wrapperClose) holdSignals() {
	w.hold.Do(func() {
		sigs := []os.Signal{syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP}
		signal.Ignore(sigs...)
		ch := make(chan os.Signal, 4)
		signal.Notify(ch, sigs...)
		go func() {
			for s := range ch {
				log.Printf("run %s: received %v during the session (Steam's Stop?): finishing the session first", w.gameID, s)
			}
		}()
	})
}

func (w *wrapperClose) request() {
	if !w.started.Load() {
		w.quit()
		return
	}
	log.Printf("run %s: window close requested during the session (Steam's Stop?): finishing the session first", w.gameID)
}
