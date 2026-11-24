package shell

import (
	"log"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"github.com/akfaiz/carryover/internal/app/gamemode"
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
		// In Game Mode and Big Picture the window is full screen, as the game
		// will be. It stays resizable: gamescope can't fill the screen with a
		// window of a fixed size, and the desktop window grows for a prompt.
		u.wrapFull = gamemode.Active() || gamemode.BigPicture()
		u.win = mygo.NewWindow(mygo.WindowOptions{
			Title:       "Carryover",
			Width:       wrapperW,
			Height:      wrapperH,
			MinWidth:    wrapperW,
			MinHeight:   wrapperH,
			AlwaysOnTop: true, // the game's own window must not bury it
			Content:     ui.View(u.wrapperView),
		})
		if u.wrapFull {
			u.win.SetFullScreen(true)
		}
		// Steam's Stop (or the window's close button) must not end the process
		// while the game runs: the upload and the cloud lock release are its job.
		u.setWindowIcon()
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
					if u.wrapFull {
						u.win.SetFullScreen(true)
					}
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

// The wrapper window's size on the desktop, and its size while a prompt (a
// dialog up to 560 wide, a conflict's two sides) shows.
const (
	wrapperW       = 600
	wrapperH       = 380
	wrapperPromptW = 780
	wrapperPromptH = 640
)

// fitWrapper grows the desktop wrapper window while a dialog shows and gives it
// back its size after, keeping it centered. A full screen window keeps its size.
func (u *Shell) fitWrapper() {
	open := u.dialogs.Open() || u.confirm.Open()
	if u.wrapFull || open == u.wrapGrown || u.win == nil {
		return
	}
	u.wrapGrown = open
	w, h := wrapperW, wrapperH
	if open {
		w, h = wrapperPromptW, wrapperPromptH
	}
	kit.OnUI(func() { // not in the middle of building a frame
		if u.win.IsFullScreen() {
			return
		}
		u.win.SetSize(w, h)
		u.win.Center()
	})
}
