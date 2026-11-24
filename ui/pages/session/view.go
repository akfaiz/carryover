package session

import (
	"github.com/egoist/mygo/ui"

	"github.com/akfaiz/carryover/internal/cloudsync/syncer"
	"github.com/akfaiz/carryover/services"
	"github.com/akfaiz/carryover/ui/env"
	"github.com/akfaiz/carryover/ui/i18n"
	"github.com/akfaiz/carryover/ui/kit"
	"github.com/akfaiz/carryover/ui/store"
)

// Page is the wrapper window's content. The ViewModel decides what a status
// change means; the page keeps the result and draws it every frame.
type Page struct {
	u  env.Env
	vm *ViewModel
	w  services.WrapperState

	msg      string
	failed   bool
	sync     bool
	progress syncer.Event

	cancelLabel string // "" while Cancel is not offered
	cancelling  bool
	cancelH     ui.Handle
}

// New is the wrapper window's page. It reads what to show from the session
// service, which must have been set up for the wrapper.
func New(e env.Env) env.Page {
	return &Page{u: e, vm: NewViewModel(), w: e.Sess().Wrapper(), msg: i18n.L("Starting…")}
}

// View draws the page: the game's art behind, its name, the message, the
// progress bar and Cancel at the bottom.
func (p *Page) View(c *ui.Context) {
	t := c.Theme()
	name := p.w.GameName
	if name == "" {
		name = p.w.GameID
	}
	ui.Column(c).Fill().Children(func() {
		p.background(c)
		ui.Spacer(c)
		ui.Column(c).Padding(24).Gap(8).Children(func() {
			ui.Text(c, "Carryover").TextColor(t.TextMuted)
			ui.Text(c, name).FontSize(22).Bold()
			msg := ui.Text(c, p.msg).Wrap()
			if p.failed {
				msg.TextColor(t.Danger)
			}
			if p.sync {
				p.progressBar(c)
			}
			if p.cancelLabel != "" {
				b := ui.Button(c.Key("cancel"), p.cancelLabel).Disabled(p.cancelling)
				if p.u.Navigator().Item(&p.cancelH, b, p.cancel).Clicked() {
					p.cancel()
				}
			}
		})
	})
}

func (p *Page) progressBar(c *ui.Context) {
	ev := p.progress
	if ev.TotalBytes > 0 {
		ui.Progress(c, float64(ev.Bytes)/float64(ev.TotalBytes))
		info := kit.FormatBytes(ev.Bytes) + " / " + kit.FormatBytes(ev.TotalBytes)
		if ev.Message != "" {
			info = ev.Message + " · " + info
		}
		ui.Text(c, info).FontSize(12).TextColor(c.Theme().TextMuted)
		return
	}
	ui.Progress(c, -1)
	if ev.Message != "" {
		ui.Text(c, ev.Message).FontSize(12).TextColor(c.Theme().TextMuted)
	}
}

func (p *Page) cancel() {
	if p.cancelling {
		return
	}
	if p.u.Sess().Cancel(p.w.GameID) {
		p.cancelLabel, p.cancelling = i18n.L("Cancelling…"), true
	}
}

// background draws the game's art behind the page, faded so the text stays
// readable. Hero or wide artwork fills the window; a cover is portrait, so it
// sits at the right edge, full height and uncropped.
func (p *Page) background(c *ui.Context) {
	if p.w.Background == "" {
		return
	}
	bm := kit.Images.File(p.w.Background, 0, 960)
	if bm == nil {
		return
	}
	art := ui.Image(c, bm).Absolute().Top(0).Bottom(0).Opacity(0.4).ZIndex(-1)
	if p.w.BackgroundWide {
		art.Left(0).Right(0).Fit(ui.Cover)
		return
	}
	art.Right(0).Fit(ui.Contain)
}

// DefaultFocus is where the first pad press lands: Cancel while there is one.
func (p *Page) DefaultFocus() *ui.Handle {
	if p.cancelLabel == "" || p.cancelling {
		return nil
	}
	return &p.cancelH
}

// StateChanged follows the status of the wrapper's game.
func (p *Page) StateChanged(store.Topic) {
	s := p.u.Store()
	st, ok := s.Status[p.w.GameID]
	if !ok {
		return
	}
	up := p.vm.Apply(st, s.Progress[p.w.GameID])
	p.msg, p.failed = up.Message, up.Failed
	p.sync, p.progress = up.Sync, up.Progress
	if up.HideCancel {
		p.cancelLabel, p.cancelling = "", false
	}
	if up.OfferCancel != nil {
		p.cancelLabel, p.cancelling = i18n.L("Cancel upload"), false
		if up.OfferCancel.BeforePlay {
			p.cancelLabel = i18n.L("Cancel and don't play")
		}
	}
	kit.Redraw()
}
