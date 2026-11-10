package ui

import (
	"image"
	"image/color"
	"log"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	"github.com/akfaiz/playrelay/internal/core"
	"github.com/akfaiz/playrelay/services"
)

// sessionPage is the wrapper window's content (playrelay run <game>).
type sessionPage struct {
	u    *UI
	w    services.WrapperState
	msg  *widget.Label
	prog *progressBar
}

func (p *sessionPage) view() fyne.CanvasObject {
	brand := muted("PlayRelay")
	name := title(p.w.GameName)
	if p.w.GameName == "" {
		name.SetText(p.w.GameID)
	}
	p.msg = widget.NewLabel("Starting…")
	p.msg.Wrapping = fyne.TextWrapWord
	p.prog = newProgress()
	p.prog.set("", 0, 0)
	content := container.NewPadded(container.NewPadded(vbox(brand, name, p.msg, p.prog)))
	var bg fyne.CanvasObject = canvas.NewRectangle(colBg)
	if p.w.CoverPath != "" {
		img := canvas.NewImageFromImage(nil)
		img.FillMode = canvas.ImageFillCover
		img.Translucency = 0.75
		images.loadFile(p.w.CoverPath, 0, func(i image.Image) {
			if i != nil {
				img.Image = i
				img.Refresh()
			}
		})
		shade := canvas.NewLinearGradient(color.NRGBA{0x0e, 0x0f, 0x14, 0x80}, colBg, 0)
		bg = container.NewStack(canvas.NewRectangle(colBg), img, shade)
	}
	return container.NewStack(bg, container.NewBorder(nil, content, nil, nil))
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
		p.prog.set(ev.Message, ev.Bytes, ev.TotalBytes)
		p.prog.Show()
	} else {
		p.prog.Hide()
	}
}

// runWrapper shows the small session window, runs pull → play → push and
// quits when done. It returns the game's exit code.
func (u *UI) runWrapper(gameID string, opts core.RunOptions) int {
	done := services.ConfigureWrapper(u.sess, gameID, opts)
	u.win = u.app.NewWindow("PlayRelay")
	u.win.SetMaster()
	u.win.SetFixedSize(true)
	u.win.Resize(fyne.NewSize(460, 340))
	u.win.CenterOnScreen()

	page := &sessionPage{u: u, w: u.sess.Wrapper()}
	u.page = page
	u.win.SetContent(container.NewStack(page.view(), u.toasts.layer))

	u.sess.OnGameRunning = func(running bool) {
		onUI(func() {
			if running {
				u.win.Hide()
			} else {
				u.win.Show()
			}
		})
	}
	u.sess.OnPrompt = func() { onUI(func() { u.win.Show(); u.win.RequestFocus() }) }

	exitCode := 0
	u.app.Lifecycle().SetOnStarted(func() {
		setTopmost(u.win)
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
			onUI(u.app.Quit)
		}()
	})
	u.win.ShowAndRun()
	return exitCode
}
