package ui

import (
	"image"
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/lang"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/akfaiz/carryover/services"
	"github.com/akfaiz/carryover/ui/env"
	"github.com/akfaiz/carryover/ui/kit"
	"github.com/akfaiz/carryover/ui/store"
)

// sessionPage is the wrapper window's content (carryover run <game>).
type sessionPage struct {
	u      env.Env
	w      services.WrapperState
	msg    *widget.Label
	prog   *kit.ProgressBar
	cancel *widget.Button
	phase  string // sync phase the Cancel button belongs to
}

func (p *sessionPage) View() fyne.CanvasObject {
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
		if p.u.Sess().Cancel(p.w.GameID) {
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
func (p *sessionPage) DefaultFocus() fyne.Focusable {
	if p.cancel == nil || !p.cancel.Visible() {
		return nil
	}
	return p.cancel
}

func (p *sessionPage) StateChanged(store.Topic) {
	st, ok := p.u.Store().Status[p.w.GameID]
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
		ev := p.u.Store().Progress[p.w.GameID]
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
			p.u.FocusForPad()
		}
	} else {
		p.phase = ""
		p.prog.Hide()
		p.cancel.Hide()
	}
}

// newSessionPage is the wrapper window's page.
func newSessionPage(e env.Env) env.Page { return &sessionPage{u: e, w: e.Sess().Wrapper()} }
