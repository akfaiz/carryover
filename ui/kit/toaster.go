package kit

import (
	"time"

	"github.com/egoist/mygo/ui"
)

// How long a toast shows: errors stay longer, since they often need reading.
const (
	toastTime    = 5 * time.Second
	toastErrTime = 8 * time.Second
	toastWidth   = 360
	// toastCountdownFPS is how often the countdown bar is painted: it crosses
	// a toast in seconds, so it needs no more.
	toastCountdownFPS = 30
)

// Toast is the MyGo toast of text, of kind, as Toaster draws it.
func Toast(text string, kind ToastKind) ui.Toast {
	t := ui.Toast{Description: text, Type: kind.typ(), Timeout: toastTime}
	if kind == ToastErr {
		t.Timeout = toastErrTime
	}
	return t
}

// typ is the ui.Toast Type of the kind.
func (k ToastKind) typ() string {
	switch k {
	case ToastOK:
		return "success"
	case ToastErr:
		return "error"
	}
	return "info"
}

// toastLook is the color and icon a toast's Type is drawn with.
func toastLook(typ string) (ui.Color, *ui.SVG) {
	switch typ {
	case "success":
		return col(ColOK), IconCheck
	case "error":
		return col(ColErr), IconAlert
	}
	return col(ColAccent), IconInfo
}

// Toaster draws the window's toasts as cards in its bottom right corner, the
// newest at the bottom: each slides in, shows its kind by a tinted icon,
// counts down with a thin bar and has a close button labelled
// closeLabel. bottom is the room left below them (for a hint bar). Call it
// once per frame.
func Toaster(c *ui.Context, bottom float32, closeLabel string) {
	ui.ToastViewportBase(c, func(vp ui.Element, toasts []ui.Toast) {
		vp.Padding(S6, S6, bottom, S6).AlignItems(ui.End).Gap(S2)
		// As MyGo stops the toasts' time.
		paused := vp.Hovered() || vp.FocusWithin()
		for _, t := range toasts {
			toastCard(c, t, closeLabel, paused)
		}
	})
}

func toastCard(c *ui.Context, t ui.Toast, closeLabel string, paused bool) {
	th := c.Theme()
	fg, icon := toastLook(t.Type)
	p := ui.ToastBase(c, t)
	left := p.Left()
	life := t.Timeout

	shadow := ui.RGBA(0, 0, 0, 0.35)
	if LightOn {
		shadow = ui.RGBA(0, 0, 0, 0.15)
	}
	p.Root.Row().AlignItems(ui.Stretch).Width(toastWidth).MaxWidth(toastWidth).
		Radius(RadiusLG).Background(col(ColBg3)).Border(1, th.Border).
		Shadow(0, 10, 28, 0, shadow).
		Transition(toastMotion).
		Children(func() {
			ui.Row(c).Grow(1).Shrink(1).Gap(S3).AlignItems(ui.Center).Padding(S3, S2, S3, S3).Children(func() {
				ui.Row(c).Width(32).Height(32).Radius(16).Background(fg.Alpha(0.16)).Center().Children(func() {
					ui.Icon(c, icon).FontSize(IconMD).TextColor(fg)
				})
				ui.Column(c).Grow(1).Shrink(1).Gap(2).Children(func() {
					if t.Title != "" {
						ui.Text(c, t.Title).FontWeight(600).Wrap()
					}
					if t.Description != "" {
						ui.Text(c, t.Description).Wrap().MaxLines(4)
					}
				})
				cb := p.CloseButton().Label(closeLabel)
				owner := cb
				cb.Padding(S1).Radius(RadiusSM).AlignSelf(ui.Start).Draw(func(p *ui.Painter, r ui.Rect) {
					paintFace(p, r, HoverColor(owner, ui.Color{}, col(ColHover)), ui.Color{}, 0, RadiusSM)
				}).Children(func() {
					HoverIcon(c, owner, IconX, IconSM, th.TextMuted, th.Text)
				})
			})
			// The countdown along the bottom, between the rounded corners.
			if left > 0 && life > 0 {
				countdown(c, left, life, paused, fg.Alpha(0.7))
			}
		})
}

// countdown is the bar of the time a toast has left of its life. It runs down
// from its Draw: building the window at every frame, for as long as a toast
// shows, would redo the whole page behind it.
func countdown(c *ui.Context, left, life time.Duration, paused bool, color ui.Color) {
	built := c.Now()
	ui.Box(c).Absolute().Left(RadiusLG).Right(RadiusLG).Bottom(0).Height(2).
		Draw(func(p *ui.Painter, r ui.Rect) {
			rest := left
			if !paused {
				rest -= p.Now().Sub(built)
				if rest > 0 {
					p.AnimationFrameRate(toastCountdownFPS)
				}
			}
			r.W *= max(0, float32(rest)/float32(life))
			p.Fill(r, color, 1)
		})
}
