package kit

import (
	"time"

	"github.com/egoist/mygo/ui"
)

// progressStep is how long the bar takes to reach a new value.
const progressStep = 250 * time.Millisecond

// progressTween is where a progress bar is on its way: from one value to the
// next, by the clock.
type progressTween struct {
	from, to float32
	since    time.Time
}

// at is the value shown at now, and whether it is still on its way.
func (t progressTween) at(now time.Time) (float32, bool) {
	k := float32(now.Sub(t.since)) / float32(progressStep)
	if k >= 1 || t.from == t.to {
		return t.to, false
	}
	return t.from + (t.to-t.from)*ui.EaseOut(max(k, 0)), true
}

// head sends the bar to v from what it shows at now. Going back, or with jump,
// it is there at once: a lower value is another transfer starting.
func (t *progressTween) head(v float32, now time.Time, jump bool) {
	if v == t.to {
		return
	}
	shown, _ := t.at(now)
	if jump || v < shown {
		shown = v
	}
	t.from, t.to, t.since = shown, v, now
}

// progressBar is a ui.Progress that slides to each new value, 0 to 1. It moves
// from its Draw, so that a transfer doesn't build the window at every frame.
func progressBar(c *ui.Context, frac float64) ui.Element {
	t := c.Theme()
	bar := ui.Progress(c, frac)
	v := float32(max(0, min(frac, 1)))
	tween := ui.Local(bar, "tween", func() progressTween { return progressTween{from: v, to: v} })
	tween.head(v, c.Now(), c.Instant() || c.Preferences().ReduceMotion)
	fill, radius := t.Accent, t.Space(0.75)
	return bar.Draw(func(p *ui.Painter, r ui.Rect) {
		shown, moving := tween.at(p.Now())
		p.Fill(ui.Rect{X: r.X, Y: r.Y, W: r.W * shown, H: r.H}, fill, radius)
		if moving {
			p.AnimationFrame()
		}
	})
}
