package shell

import (
	"log"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"github.com/akfaiz/carryover/internal/core"
	"github.com/akfaiz/carryover/services"
	"github.com/akfaiz/carryover/ui/env"
	"github.com/akfaiz/carryover/ui/kit"
)

// RunWrapper is the wrapper window of `carryover run <game>`: it syncs before
// the game, hides while it plays and syncs after. It returns the game's exit
// code once the session is over.
func (u *Shell) RunWrapper(newPage func(e env.Env) env.Page, gameID string, opts core.RunOptions) int {
	done := services.ConfigureWrapper(u.sess, gameID, opts)
	closer := &wrapperClose{gameID: gameID, quit: mygo.App.Quit}
	exitCode := 0

	mygo.App.WhenReady(func() {
		u.wrapper = true
		u.prompts.Wrapper = true
		u.page = newPage(u)
		u.win = mygo.NewWindow(mygo.WindowOptions{
			Title:         "Carryover",
			Width:         460,
			Height:        340,
			DisableResize: true,
			AlwaysOnTop:   true, // the game's own window must not bury it
			Content:       ui.View(u.wrapperView),
		})
		// Steam's Stop (or the window's close button) must not end the process
		// while the game runs: the upload and the cloud lock release are its job.
		u.win.OnClose(func(e *mygo.CloseEvent) {
			if closer.vetoes() {
				e.PreventDefault()
				closer.request()
			}
		})
		u.sess.OnGameRunning = func(running bool) {
			if running {
				closer.started.Store(true)
				closer.holdSignals()
			}
			kit.OnUI(func() {
				u.readPad(!running)
				if running {
					u.win.Hide()
				} else {
					u.win.Show()
				}
			})
		}
		u.sess.OnPrompt = func() {
			kit.OnUI(func() { u.win.Show(); u.win.Focus(); u.readPad(true) })
		}
		u.readPad(true)
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
				time.Sleep(4 * time.Second) // long enough to read the error
			} else {
				time.Sleep(1200 * time.Millisecond)
			}
			closer.finish() // or Quit closes the window, and the close handler vetoes it
			kit.OnUI(mygo.App.Quit)
		}()
	})
	if err := mygo.App.Run(); err != nil {
		log.Printf("ui: %v", err)
		return 1
	}
	return exitCode
}
