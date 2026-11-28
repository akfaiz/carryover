package kit

import (
	"time"

	"github.com/egoist/mygo/ui"
)

// How the app moves. It is short, so that nothing waits on it, and it is kept
// here, so that everything moves the same way. Motion never carries meaning of
// its own: while the desktop asks for less of it, MyGo shows changes at once
// (fades and colors stay).
//
// Two rules keep it cheap and safe. What loops or counts down moves from its
// Draw, whose frames paint without building the view. And what the gamepad can
// reach moves little: the Navigator picks the next control from the boxes of
// the last frame.

const (
	motionFast = 120 * time.Millisecond // a ring, a list leaving
	motionBase = 180 * time.Millisecond // content coming and going
)

var (
	// PageMotion fades a page in as it opens. It only fades: a page that slid
	// would move the controls under a D-pad press.
	PageMotion = ui.ElementTransition{Colors: true, Duration: 140 * time.Millisecond, Enter: &ui.Motion{}}

	// imageMotion fades in a picture that arrives after the box it is in.
	imageMotion = ui.ElementTransition{Colors: true, Duration: motionBase, Enter: &ui.Motion{}}
	// placeholderMotion fades out what stood in for the picture.
	placeholderMotion = ui.ElementTransition{Colors: true, Duration: motionBase, Exit: &ui.Motion{}}

	// ringMotion fades the gamepad's ring from one control to the next.
	ringMotion = ui.ElementTransition{Shadows: true, Duration: motionFast}

	// badgeMotion mixes a badge's tint as its state changes.
	badgeMotion = ui.ElementTransition{Colors: true, Duration: motionBase}

	// backdropMotion fades a dialog, and what dims the window behind it.
	backdropMotion = ui.ElementTransition{
		Colors: true, Duration: 150 * time.Millisecond, ExitDuration: motionFast,
		Enter: &ui.Motion{}, Exit: &ui.Motion{},
	}

	// dropdownMotion drops a list from its anchor, or raises it where it opens
	// above (Flip).
	dropdownMotion = ui.ElementTransition{
		Colors: true, Duration: motionFast, ExitDuration: 80 * time.Millisecond,
		Enter: &ui.Motion{Y: -6, Flip: true}, Exit: &ui.Motion{Y: -6, Flip: true},
	}

	// toastMotion slides a toast in from the right and back out; the others
	// slide to their places.
	toastMotion = ui.ElementTransition{
		Duration: 220 * time.Millisecond,
		Enter:    &ui.Motion{X: S6}, Exit: &ui.Motion{X: S6},
	}
)

// popIn grows a dialog's panel into place as it shows. A panel appears together
// with its backdrop, where an Enter doesn't play, so it is posed first where it
// comes from, and the next frame sends it to its place.
func popIn(c *ui.Context, panel ui.Element) {
	posed := ui.Local(panel, "pop-in", func() bool { return false })
	var p ui.Pose
	if !*posed {
		*posed = true
		p = ui.Pose{Y: S2, Scale: 0.97}
		c.Invalidate()
	}
	panel.AnimatePose(p, ui.Snappy)
}

// PadLift raises el while the gamepad's cursor is on one of handles: what is
// picked on a grid of covers shows from the couch. It does nothing for a mouse
// or a keyboard. Give it every frame, for the way down to move as well.
func (n *Navigator) PadLift(c *ui.Context, el ui.Element, handles ...*ui.Handle) {
	if !n.padUsed {
		return
	}
	var p ui.Pose
	for _, h := range handles {
		if h.Focused(c) {
			p = ui.Pose{Scale: 1.025}
			break
		}
	}
	el.AnimatePose(p, ui.Snappy)
}

// Reveal shows what fn builds while open, growing open and closing again, so
// that what is below moves with it. Closed, it takes no room; in a column with
// a gap it still counts as a child, so give the gap to what fn builds. key
// tells it from its siblings.
func Reveal(c *ui.Context, key any, open bool, fn func()) {
	box := ui.Column(c.Key(key)).Shrink(0)
	target := float32(0)
	if open {
		target = 1
	}
	shown := box.Animate("reveal", target, motionBase)
	if !open && shown == 0 {
		return
	}
	box.Children(func() {
		inner := ui.Column(c).Shrink(0)
		inner.Children(fn)
		if shown < 1 {
			// The height the content had in the last frame: none as it starts to
			// open, and more as it does.
			inner.Opacity(shown)
			box.ClipY().Height(shown * inner.Bounds().H)
		}
	})
}

// Reflow moves a list as rows come and go: a new row grows in, one removed
// collapses, and what they push along slides. It only does so right after the
// rows changed. A transition left on would also trail behind every other change
// of layout, such as help unfolding above the list.
//
// Call Watch once a frame, before the rows are built, then Row on every row and
// Box on what grows or moves with them (their card, and the cards below). Key
// the rows, and the card that holds them, so that MyGo can tell a row removed
// from another that took its place.
type Reflow struct {
	rows    any
	watched bool
	changed time.Time
	moving  bool
}

// Watch notes which rows there are: rows is a comparable value that differs
// whenever the rows do.
func (r *Reflow) Watch(c *ui.Context, rows any) {
	if r.watched && rows != r.rows {
		r.changed = c.Now()
	}
	r.rows, r.watched = rows, true
	// For a while, not just the one frame: a click that changes the rows builds
	// that frame twice, and the second build counts.
	r.moving = !r.changed.IsZero() && c.Now().Sub(r.changed) < motionBase
}

// duration is that of a Transition: negative shows changes at once.
func (r *Reflow) duration() time.Duration {
	if r.moving {
		return motionBase
	}
	return -1
}

// Row animates one row of the list.
func (r *Reflow) Row(row ui.Element) ui.Element {
	return row.Transition(ui.ElementTransition{
		Duration: r.duration(),
		// A row goes as its last build said, and that was before the rows
		// changed: its way out always takes its time.
		ExitDuration: motionBase,
		Enter:        &ui.Motion{Collapse: true},
		Exit:         &ui.Motion{Collapse: true},
	})
}

// Box animates what the rows push along: their card, and the cards below it.
func (r *Reflow) Box(box ui.Element) ui.Element {
	return box.Transition(ui.ElementTransition{Duration: r.duration()})
}
