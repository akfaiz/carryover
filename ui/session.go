package ui

import (
	"image"
	"image/color"
	"log"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/lang"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/akfaiz/carryover/internal/core"
	"github.com/akfaiz/carryover/internal/ui/kit"
	"github.com/akfaiz/carryover/services"
)

// sessionPage is the wrapper window's content (carryover run <game>).
type sessionPage struct {
	u      *UI
	w      services.WrapperState
	msg    *widget.Label
	prog   *kit.ProgressBar
	cancel *widget.Button
	phase  string // sync phase the Cancel button belongs to
}

func (p *sessionPage) view() fyne.CanvasObject {
	brand := kit.Muted("Carryover")
	name := kit.Title(p.w.GameName)
	if p.w.GameName == "" {
		name.SetText(p.w.GameID)
	}
	p.msg = widget.NewLabel(lang.L("Starting…"))
	p.msg.Wrapping = fyne.TextWrapWord
	p.prog = kit.NewProgress()
	p.prog.Set("", 0, 0)
	p.cancel = widget.NewButtonWithIcon(lang.L("Cancel"), theme.CancelIcon(), func() {
		if p.u.sess.Cancel(p.w.GameID) {
			p.cancel.SetText(lang.L("Cancelling…"))
			p.cancel.Disable()
		}
	})
	p.cancel.Hide()
	content := container.NewPadded(container.NewPadded(kit.VBox(brand, name, p.msg, p.prog, kit.HBox(p.cancel))))
	var bg fyne.CanvasObject = canvas.NewRectangle(kit.ColBg)
	if p.w.CoverPath != "" {
		img := canvas.NewImageFromImage(nil)
		img.FillMode = canvas.ImageFillCover
		img.Translucency = 0.75
		kit.Images.LoadFile(p.w.CoverPath, 0, func(i image.Image) {
			if i != nil {
				img.Image = i
				img.Refresh()
			}
		})
		shade := canvas.NewLinearGradient(color.NRGBA{kit.ColBg.R, kit.ColBg.G, kit.ColBg.B, 0x80}, kit.ColBg, 0)
		bg = container.NewStack(canvas.NewRectangle(kit.ColBg), img, shade)
	}
	return container.NewStack(bg, container.NewBorder(nil, content, nil, nil))
}

// defaultFocus starts a gamepad on Cancel while there is one.
func (p *sessionPage) defaultFocus() fyne.Focusable {
	if p.cancel == nil || !p.cancel.Visible() {
		return nil
	}
	return p.cancel
}

func (p *sessionPage) stateChanged(change) {
	st, ok := p.u.status[p.w.GameID]
	if !ok {
		return
	}
	p.msg.SetText(st.Message)
	if st.Phase == "error" {
		p.msg.Importance = widget.DangerImportance
	} else {
		p.msg.Importance = widget.MediumImportance
	}
	p.msg.Refresh()
	if st.Phase == "sync" {
		ev := p.u.progress[p.w.GameID]
		p.prog.Set(ev.Message, ev.Bytes, ev.TotalBytes)
		p.prog.Show()
		if p.phase != st.Message {
			// A new sync phase (before or after playing): offer Cancel again.
			p.phase = st.Message
			p.cancel.SetText(lang.L("Cancel upload"))
			if st.BeforePlay {
				p.cancel.SetText(lang.L("Cancel and don't play"))
			}
			p.cancel.Enable()
			p.cancel.Show()
			p.u.focusForPad()
		}
	} else {
		p.phase = ""
		p.prog.Hide()
		p.cancel.Hide()
	}
}

// runWrapper shows the small session window, runs pull → play → push and
// quits when done. It returns the game's exit code.
func (u *UI) runWrapper(gameID string, opts core.RunOptions) int {
	done := services.ConfigureWrapper(u.sess, gameID, opts)
	u.win = u.app.NewWindow("Carryover")
	u.win.SetMaster()
	u.win.SetFixedSize(true)
	u.win.Resize(fyne.NewSize(460, 340))
	u.win.CenterOnScreen()

	page := &sessionPage{u: u, w: u.sess.Wrapper()}
	u.page = page
	// The prompts and Cancel work with the controller the shortcut was started
	// with; while the game runs, the controller is the game's.
	u.nav = kit.NewNavigator(u.win, kit.NavigatorOptions{DefaultFocus: page.defaultFocus})
	u.win.SetContent(container.NewStack(page.view(), u.toasts.Layer, u.nav.RingLayer()))
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
