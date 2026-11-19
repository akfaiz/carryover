package session

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

// Page is the wrapper window's content.
type Page struct {
	u      env.Env
	vm     *ViewModel
	w      services.WrapperState
	msg    *widget.Label
	prog   *kit.ProgressBar
	cancel *widget.Button
}

// New is the wrapper window's page. It reads what to show from the session
// service, which must have been set up for the wrapper.
func New(e env.Env) env.Page {
	return &Page{u: e, vm: NewViewModel(), w: e.Sess().Wrapper()}
}

func (p *Page) View() fyne.CanvasObject {
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

// DefaultFocus is where the first pad press lands: Cancel while there is one.
func (p *Page) DefaultFocus() fyne.Focusable {
	if p.cancel == nil || !p.cancel.Visible() {
		return nil
	}
	return p.cancel
}

func (p *Page) StateChanged(store.Topic) {
	s := p.u.Store()
	st, ok := s.Status[p.w.GameID]
	if !ok {
		return
	}
	up := p.vm.Apply(st, s.Progress[p.w.GameID])
	p.msg.SetText(up.Message)
	if up.Failed {
		p.msg.Importance = widget.DangerImportance
	} else {
		p.msg.Importance = widget.MediumImportance
	}
	p.msg.Refresh()
	if up.Sync {
		p.prog.Set(up.Progress.Message, up.Progress.Bytes, up.Progress.TotalBytes)
		p.prog.Show()
	} else {
		p.prog.Hide()
	}
	if up.HideCancel {
		p.cancel.Hide()
	}
	if up.OfferCancel != nil {
		p.cancel.SetText(lang.L("Cancel upload"))
		if up.OfferCancel.BeforePlay {
			p.cancel.SetText(lang.L("Cancel and don't play"))
		}
		p.cancel.Enable()
		p.cancel.Show()
		// Leave the focus off Cancel, before and after the game: the player may
		// still be pressing A, and the first D-pad or A press only focuses it
		// (Navigator.accept), so a stray press can't abort the sync.
		p.u.Window().Canvas().Unfocus()
	}
}
