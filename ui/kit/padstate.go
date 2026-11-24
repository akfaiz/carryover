package kit

import "time"

// Gamepad and keyboard actions, as the Navigator reads them.

// PadAction is what a key or gamepad press does in a dialog.
type PadAction int

const (
	PadNone   PadAction = iota
	PadPrev             // the previous button (arrow keys in a Nav)
	PadNext             // the next button
	PadAccept           // press the focused button
	PadBack             // leave the dialog the safe way
	PadUp               // the D-pad: the control above, below, ...
	PadDown
	PadLeft
	PadRight
	PadTabPrev // LB: the previous tab or section
	PadTabNext // RB: the next tab or section
	PadSearch  // X
	PadSync    // Y
	PadView    // View (Back, Select): full screen
)

// PadState is the part of a gamepad the app uses: the D-pad (or the left
// stick), A to accept, B to go back, X and Y, the shoulder buttons and the
// right stick's vertical axis (-1 up to 1 down, 0 inside the dead zone).
type PadState struct {
	Up, Down, Left, Right bool
	Accept, Back          bool
	X, Y, LB, RB          bool
	View                  bool // the View button (Back, Select on older pads)
	Scroll                float32
}

// PadSource reads the current gamepad state; ok is false without a gamepad.
type PadSource func() (state PadState, ok bool)

const (
	padRepeatDelay = 400 * time.Millisecond // holding a direction repeats after this
	padRepeatEvery = 120 * time.Millisecond // ... and then this often
	padPollEvery   = 40 * time.Millisecond
)

// padButton turns the held/released state of one input into presses, with
// auto-repeat for directions.
type padButton struct {
	down        bool
	since, last time.Time
}

func (b *padButton) fire(down, repeat bool, now time.Time) bool {
	if !down {
		b.down = false
		return false
	}
	if !b.down {
		b.down, b.since, b.last = true, now, now
		return true
	}
	if repeat && now.Sub(b.since) >= padRepeatDelay && now.Sub(b.last) >= padRepeatEvery {
		b.last = now
		return true
	}
	return false
}

// held marks the input as already held, so it fires only after a release: a
// button still down from the press that opened the dialog must not answer it.
func (b *padButton) held(now time.Time) {
	b.down, b.since, b.last = true, now.Add(time.Hour), now
}

// padTracker turns gamepad states into actions.
type padTracker struct {
	up, down, left, right, accept, back, x, y, lb, rb, view padButton
}

// prime treats everything held in s as held before the dialog opened.
func (t *padTracker) prime(s PadState, now time.Time) {
	for _, x := range []struct {
		b    *padButton
		held bool
	}{
		{&t.up, s.Up}, {&t.down, s.Down}, {&t.left, s.Left}, {&t.right, s.Right}, {&t.accept, s.Accept}, {&t.back, s.Back},
		{&t.x, s.X}, {&t.y, s.Y}, {&t.lb, s.LB}, {&t.rb, s.RB}, {&t.view, s.View},
	} {
		if x.held {
			x.b.held(now)
		}
	}
}

// update returns the actions s triggers.
func (t *padTracker) update(s PadState, now time.Time) []PadAction {
	var out []PadAction
	add := func(fire bool, a PadAction) {
		if fire {
			out = append(out, a)
		}
	}
	add(t.up.fire(s.Up, true, now), PadUp)
	add(t.left.fire(s.Left, true, now), PadLeft)
	add(t.down.fire(s.Down, true, now), PadDown)
	add(t.right.fire(s.Right, true, now), PadRight)
	add(t.accept.fire(s.Accept, false, now), PadAccept)
	add(t.back.fire(s.Back, false, now), PadBack)
	add(t.lb.fire(s.LB, false, now), PadTabPrev)
	add(t.rb.fire(s.RB, false, now), PadTabNext)
	add(t.x.fire(s.X, false, now), PadSearch)
	add(t.y.fire(s.Y, false, now), PadSync)
	add(t.view.fire(s.View, false, now), PadView)
	return out
}
