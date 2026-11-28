package session

import (
	"strconv"

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
	ended    bool
	step     Step
	sync     bool
	progress syncer.Event

	cancelLabel string // "" while Cancel is not offered
	cancelling  bool
}

// New is the wrapper window's page. It reads what to show from the session
// service, which must have been set up for the wrapper.
func New(e env.Env) env.Page {
	return &Page{u: e, vm: NewViewModel(), w: e.Sess().Wrapper(), msg: i18n.L("Starting…")}
}

// bigWindow is the size from which the page is laid out for full screen (a
// Steam Deck is 1280x800).
const (
	bigWidth  float32 = 900
	bigHeight float32 = 560
)

// View draws the page: the game's art behind, faded to the bottom, and over it
// the game's name, the steps of the session and a card with what happens now.
// While a dialog is open only the art and the app's name stay: the dialog would
// cover the rest halfway.
func (p *Page) View(c *ui.Context) {
	w, h := c.Size()
	big := w >= bigWidth && h >= bigHeight
	pad, gap := kit.S6, kit.S4
	if big {
		pad, gap = 2*kit.S8, kit.S6
	}
	name := p.w.GameName
	if name == "" {
		name = p.w.GameID
	}
	ui.Column(c).Fill().Children(func() {
		p.background(c, w, h)
		ui.Row(c).Padding(pad*0.75, pad, 0, pad).Children(func() {
			kit.IconLine(c, kit.IconCloud, "Carryover", c.Theme().TextMuted)
		})
		ui.Spacer(c)
		if p.u.Dialogs().Open() {
			return
		}
		ui.Column(c).Padding(pad).Gap(gap).MaxWidth(860).Children(func() {
			title := kit.HeroTitle(c, name).Wrap().MaxLines(2)
			if big {
				title.FontSize(kit.TextDisplay)
			}
			p.steps(c, big)
			p.card(c, big)
		})
	})
}

// sessionStep is one step of the tracker.
type sessionStep struct {
	icon  *ui.SVG
	label string
}

// steps draws the tracker: sync, play, upload, each done, current or to come.
func (p *Page) steps(c *ui.Context, big bool) {
	t := c.Theme()
	steps := []sessionStep{
		{kit.IconDownload, i18n.L("Sync saves")},
		{kit.IconPlay, i18n.L("Play")},
		{kit.IconUpload, i18n.L("Upload saves")},
	}
	line := float32(28)
	if big {
		line = 48
	}
	ui.Row(c).Gap(kit.S3).AlignItems(ui.Center).Wrap().Children(func() {
		for i, s := range steps {
			st := Step(i)
			if i > 0 {
				col := t.TextMuted.Alpha(0.35)
				if p.step >= st {
					col = t.Accent
				}
				ui.Box(c).Width(line).Height(2).Radius(1).Background(col)
			}
			p.stepMark(c, st, s, big)
		}
	})
}

// stepMark draws one step of the tracker.
func (p *Page) stepMark(c *ui.Context, st Step, s sessionStep, big bool) {
	t := c.Theme()
	size := float32(28)
	if big {
		size = 36
	}
	var (
		bg, fg, text = t.Surface, t.TextMuted, t.TextMuted
		border       = t.Border
		icon         = s.icon
		weight       = 500
	)
	switch {
	case p.step > st:
		bg, fg, text, border, icon = t.Accent, t.AccentText, t.Text, t.Accent, kit.IconCheck
	case p.step == st && p.failed:
		bg, fg, text, border, icon, weight = t.Danger.Alpha(0.18), t.Danger, t.Danger, t.Danger, kit.IconAlert, 600
	case p.step == st && !p.ended:
		bg, fg, text, border, weight = t.Accent.Alpha(0.18), t.Accent, t.Text, t.Accent, 600
	}
	ui.Row(c).Gap(kit.S2).AlignItems(ui.Center).Children(func() {
		ui.Row(c).Width(size).Height(size).Radius(size/2).Background(bg).Border(1.5, border).Center().Children(func() {
			ui.Icon(c, icon).FontSize(kit.IconSM).TextColor(fg)
		})
		label := ui.Text(c, s.label).FontWeight(weight).TextColor(text)
		if big {
			label.FontSize(kit.TextLG)
		}
	})
}

// card is what happens now: a spinner (or the outcome's icon), the message,
// the transfer's progress and Cancel.
func (p *Page) card(c *ui.Context, big bool) {
	t := c.Theme()
	pad := kit.S4
	if big {
		pad = kit.S6
	}
	ui.Column(c).Padding(pad).Gap(kit.S3).Radius(kit.RadiusLG).
		Background(t.Surface.Alpha(0.86)).Border(1, t.Border).Children(func() {
		ui.Row(c).Gap(kit.S3).AlignItems(ui.Center).Children(func() {
			p.indicator(c, big)
			ui.Column(c).Grow(1).Shrink(1).Gap(2).Children(func() {
				msg := ui.Text(c, p.msg).FontWeight(600).Wrap().MaxLines(3)
				if big {
					msg.FontSize(kit.TextCard)
				} else {
					msg.FontSize(kit.TextLG)
				}
				if p.failed {
					msg.TextColor(t.Danger)
				}
				if info := p.transferInfo(); info != "" {
					kit.Small(c, info)
				}
			})
			if pct := p.percent(); pct != "" {
				ui.Text(c, pct).FontWeight(600).TextColor(t.TextMuted)
			}
		})
		if p.sync {
			v := -1.0
			if p.progress.TotalBytes > 0 {
				v = float64(p.progress.Bytes) / float64(p.progress.TotalBytes)
			}
			ui.Progress(c, v)
		}
		if p.cancelLabel != "" {
			kit.Btn(c, p.u.Navigator(), "cancel", kit.BtnSecondary, kit.IconX, p.cancelLabel, p.cancel, kit.BtnDisabled(p.cancelling)).AlignSelf(ui.Start)
		}
	})
}

// indicator is the card's leading mark: a spinner while something runs, a
// tick when the session ended well, a warning when it failed.
func (p *Page) indicator(c *ui.Context, big bool) {
	t := c.Theme()
	size := float32(36)
	if big {
		size = 44
	}
	var icon *ui.SVG
	col := t.Accent
	switch {
	case p.failed:
		icon, col = kit.IconAlert, t.Danger
	case p.ended:
		icon, col = kit.IconCheck, t.Success
	case p.step == StepPlay:
		icon = kit.IconPlay
	}
	ui.Row(c).Width(size).Height(size).Radius(size / 2).Background(col.Alpha(0.16)).Center().Children(func() {
		if icon == nil {
			ui.Spinner(c).TextColor(col)
			return
		}
		ui.Icon(c, icon).FontSize(kit.IconMD).TextColor(col)
	})
}

// transferInfo is the line under the message during a sync: what is being
// transferred and how much of it.
func (p *Page) transferInfo() string {
	if !p.sync {
		return ""
	}
	ev := p.progress
	if ev.TotalBytes <= 0 {
		return i18n.Text(ev.Message, ev.Msg)
	}
	info := kit.FormatBytes(ev.Bytes) + " / " + kit.FormatBytes(ev.TotalBytes)
	if ev.Message != "" {
		info = i18n.Text(ev.Message, ev.Msg) + " · " + info
	}
	return info
}

// percent is the share transferred, "" when unknown.
func (p *Page) percent() string {
	if !p.sync || p.progress.TotalBytes <= 0 {
		return ""
	}
	return strconv.Itoa(int(100*p.progress.Bytes/p.progress.TotalBytes)) + "%"
}

func (p *Page) cancel() {
	if p.cancelling {
		return
	}
	if p.u.Sess().Cancel(p.w.GameID) {
		p.cancelLabel, p.cancelling = i18n.L("Cancelling…"), true
	}
}

// background draws the game's art behind the page, faded to the page's color
// toward the bottom so the text stays readable. Hero or wide artwork fills the
// window; a cover is portrait, so it sits at the right edge, full height and
// uncropped, fading into the page on its left.
func (p *Page) background(c *ui.Context, w, h float32) {
	bgc := c.Theme().Background
	if p.w.Background != "" {
		if bm := kit.Images.File(p.w.Background, 0, 960); bm != nil {
			art := ui.Image(c, bm).Absolute().Top(0).Bottom(0).ZIndex(-1)
			if p.w.BackgroundWide {
				art.Left(0).Right(0).Fit(ui.Cover).Opacity(0.6)
			} else {
				art.Right(0).Fit(ui.Contain).Opacity(0.5)
				edge := float32(0)
				if w > 0 {
					edge = max(0, (w-h*2/3)/w)
				}
				ui.Box(c).Absolute().Top(0).Bottom(0).Left(0).Right(0).ZIndex(-1).
					LinearGradient(ui.LinearGradient{Angle: 90, Stops: []ui.GradientStop{
						{Color: bgc, At: 0}, {Color: bgc, At: edge}, {Color: bgc.Alpha(0), At: min(1, edge+0.2)},
					}})
			}
		}
	}
	ui.Box(c).Absolute().Top(0).Bottom(0).Left(0).Right(0).ZIndex(-1).
		LinearGradient(ui.LinearGradient{Angle: 180, Stops: []ui.GradientStop{
			{Color: bgc.Alpha(0.2), At: 0}, {Color: bgc.Alpha(0.55), At: 0.4}, {Color: bgc.Alpha(0.96), At: 1},
		}})
}

// DefaultFocus is where the first pad press lands: Cancel while there is one.
func (p *Page) DefaultFocus() *ui.Handle {
	if p.cancelLabel == "" || p.cancelling {
		return nil
	}
	return kit.BtnHandle(p.u.Navigator(), "cancel")
}

// StateChanged follows the status of the wrapper's game.
func (p *Page) StateChanged(store.Topic) {
	s := p.u.Store()
	st, ok := s.Status[p.w.GameID]
	if !ok {
		return
	}
	up := p.vm.Apply(st, s.Progress[p.w.GameID])
	p.msg, p.failed, p.ended, p.step = i18n.Text(up.Message, up.Msg), up.Failed, up.Ended, up.Step
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
