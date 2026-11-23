package session

import (
	"image"
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/akfaiz/carryover/ui/i18n"

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
	p.msg = widget.NewLabel(i18n.L("Starting…"))
	p.msg.Wrapping = fyne.TextWrapWord
	p.prog = kit.NewProgress()
	p.prog.Set("", 0, 0)
	p.cancel = widget.NewButtonWithIcon(i18n.L("Cancel"), theme.CancelIcon(), func() {
		if p.u.Sess().Cancel(p.w.GameID) {
			p.cancel.SetText(i18n.L("Cancelling…"))
			p.cancel.Disable()
		}
	})
	p.cancel.Hide()
	content := container.NewPadded(container.NewPadded(kit.VBox(brand, name, p.msg, p.prog, kit.HBox(p.cancel))))
	var bg fyne.CanvasObject = canvas.NewRectangle(kit.ColBg)
	if p.w.Background != "" {
		bg = p.background()
	}
	return container.NewStack(bg, container.NewBorder(nil, content, nil, nil))
}

// background draws the game's art behind the page. Hero or wide artwork fills the
// window. A cover is portrait, so it sits at the right edge, full height and
// uncropped, with its left edge fading into the page colour. A vertical shade
// over either keeps the text readable.
func (p *Page) background() fyne.CanvasObject {
	img := canvas.NewImageFromImage(nil)
	var art *fyne.Container
	if p.w.BackgroundWide {
		img.FillMode = canvas.ImageFillCover
		img.Translucency = 0.6
		art = container.NewStack(img)
	} else {
		img.FillMode = canvas.ImageFillStretch // the layout gives it the cover's own shape
		img.Translucency = 0.6
		fade := canvas.NewHorizontalGradient(kit.ColBg, color.NRGBA{kit.ColBg.R, kit.ColBg.G, kit.ColBg.B, 0})
		art = container.New(&coverLayout{aspect: 2.0 / 3}, img, fade)
	}
	kit.Images.LoadFile(p.w.Background, 0, func(i image.Image) {
		if i == nil {
			return
		}
		if l, ok := art.Layout.(*coverLayout); ok {
			if b := i.Bounds(); b.Dy() > 0 {
				l.aspect = float32(b.Dx()) / float32(b.Dy())
			}
		}
		img.Image = i
		img.Refresh()
		art.Refresh()
	})
	shade := canvas.NewLinearGradient(color.NRGBA{kit.ColBg.R, kit.ColBg.G, kit.ColBg.B, 0x80}, kit.ColBg, 0)
	return container.NewStack(canvas.NewRectangle(kit.ColBg), art, shade)
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
		p.cancel.SetText(i18n.L("Cancel upload"))
		if up.OfferCancel.BeforePlay {
			p.cancel.SetText(i18n.L("Cancel and don't play"))
		}
		p.cancel.Enable()
		p.cancel.Show()
		// Leave the focus off Cancel, before and after the game: the player may
		// still be pressing A, and the first D-pad or A press only focuses it
		// (Navigator.accept), so a stray press can't abort the sync.
		p.u.Window().Canvas().Unfocus()
	}
}

// coverLayout puts the first object (the cover) at the right edge, as tall as
// the area, with the cover's own width/height ratio, and the second (a fade)
// over its left third.
type coverLayout struct{ aspect float32 }

func (l *coverLayout) Layout(objs []fyne.CanvasObject, size fyne.Size) {
	w := min(size.Height*l.aspect, size.Width)
	x := size.Width - w
	if len(objs) > 0 {
		objs[0].Move(fyne.NewPos(x, 0))
		objs[0].Resize(fyne.NewSize(w, size.Height))
	}
	if len(objs) > 1 {
		objs[1].Move(fyne.NewPos(x, 0))
		objs[1].Resize(fyne.NewSize(w/3, size.Height))
	}
}

func (l *coverLayout) MinSize([]fyne.CanvasObject) fyne.Size { return fyne.NewSize(0, 0) }
