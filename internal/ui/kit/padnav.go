package kit

import (
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver"
	"fyne.io/fyne/v2/widget"
)

// Keyboard and gamepad navigation for dialogs.
//
// Fyne moves focus with Tab only, a focused button only reacts to Space, and
// there is no gamepad support. A Steam shortcut (Steam Deck game mode) is
// driven with a controller, whose D-pad Steam may map to arrow keys, A to Enter
// and B to Escape, or hand to the app as a plain gamepad. A Nav makes a
// dialog's buttons work with all of these: the arrow keys and the D-pad (or the
// left stick) move between the buttons, Enter/Space/A press the focused one,
// and Escape/B steps back.

// PadAction is what a key or gamepad press does in a dialog.
type PadAction int

const (
	PadNone   PadAction = iota
	PadPrev             // up, left: the previous button
	PadNext             // down, right: the next button
	PadAccept           // press the focused button
	PadBack             // leave the dialog the safe way
)

// actionForKey maps a key to its action (PadNone for any other key).
func actionForKey(k fyne.KeyName) PadAction {
	switch k {
	case fyne.KeyLeft, fyne.KeyUp:
		return PadPrev
	case fyne.KeyRight, fyne.KeyDown:
		return PadNext
	case fyne.KeyReturn, fyne.KeyEnter:
		return PadAccept
	case fyne.KeyEscape, fyne.KeyBackspace:
		return PadBack
	}
	return PadNone
}

// PadState is the part of a gamepad that dialogs use: the D-pad (or the left
// stick), A to accept and B to go back.
type PadState struct {
	Up, Down, Left, Right bool
	Accept, Back          bool
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
	up, down, left, right, accept, back padButton
}

// prime treats everything held in s as held before the dialog opened.
func (t *padTracker) prime(s PadState, now time.Time) {
	for _, x := range []struct {
		b    *padButton
		held bool
	}{{&t.up, s.Up}, {&t.down, s.Down}, {&t.left, s.Left}, {&t.right, s.Right}, {&t.accept, s.Accept}, {&t.back, s.Back}} {
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
	add(t.up.fire(s.Up, true, now), PadPrev)
	add(t.left.fire(s.Left, true, now), PadPrev)
	add(t.down.fire(s.Down, true, now), PadNext)
	add(t.right.fire(s.Right, true, now), PadNext)
	add(t.accept.fire(s.Accept, false, now), PadAccept)
	add(t.back.fire(s.Back, false, now), PadBack)
	return out
}

// Nav drives one dialog with the keyboard and a gamepad. Create it before the
// dialog's buttons, build the buttons with Button (and ChoiceTile), show the
// dialog, then Start it; Stop it when the dialog goes away.
type Nav struct {
	win     fyne.Window
	back    func()
	onInput func()
	pad     PadSource

	tracker  padTracker
	prevKey  func(*fyne.KeyEvent)
	stopPoll chan struct{}
}

// NewNav makes the navigation of a dialog in win. back runs for Escape and B:
// it should leave the dialog the safe way (cancel, decide later).
func NewNav(win fyne.Window, back func()) *Nav {
	n := &Nav{win: win, back: back}
	if _, native := win.(driver.NativeWindow); native {
		n.pad = glfwPad // not in Fyne's headless test driver
	}
	return n
}

// OnInput sets a function that runs on every navigation press, e.g. to stop a
// countdown: the user is choosing.
func (n *Nav) OnInput(f func()) { n.onInput = f }

// Apply does what a key or gamepad press means.
func (n *Nav) Apply(a PadAction) {
	if a == PadNone {
		return
	}
	if n.onInput != nil {
		n.onInput()
	}
	c := n.win.Canvas()
	switch a {
	case PadPrev:
		c.FocusPrevious()
	case PadNext:
		c.FocusNext()
	case PadAccept:
		if t, ok := c.Focused().(fyne.Tappable); ok {
			t.Tapped(&fyne.PointEvent{})
		} else {
			c.FocusNext() // nothing is focused yet: a first press picks the first button
		}
	case PadBack:
		if n.back != nil {
			n.back()
		}
	}
}

// Start focuses first (the button a press should answer by default: pick the
// safe one) and begins listening. Keys that reach the window while no button is
// focused (after a click on empty space) are handled too.
func (n *Nav) Start(first fyne.Focusable) {
	c := n.win.Canvas()
	if first != nil {
		c.Focus(first)
	}
	n.prevKey = c.OnTypedKey()
	c.SetOnTypedKey(func(ev *fyne.KeyEvent) { n.Apply(actionForKey(ev.Name)) })
	if n.pad != nil {
		n.startPad()
	}
}

// Stop ends the listening and restores the window's own key handling.
func (n *Nav) Stop() {
	if n.stopPoll != nil {
		close(n.stopPoll)
		n.stopPoll = nil
	}
	if c := n.win.Canvas(); c != nil && n.prevKey != nil {
		c.SetOnTypedKey(n.prevKey)
	}
	n.prevKey = nil
}

func (n *Nav) startPad() {
	stop := make(chan struct{})
	n.stopPoll = stop
	if s, ok := n.pad(); ok {
		n.tracker.prime(s, time.Now())
	}
	go func() {
		t := time.NewTicker(padPollEvery)
		defer t.Stop()
		for {
			select {
			case <-stop:
				return
			case <-t.C:
				// Gamepad state is read on the main thread, like the rest of the UI.
				fyne.Do(func() {
					select {
					case <-stop:
						return
					default:
					}
					n.poll(time.Now())
				})
			}
		}
	}()
}

// poll reads the gamepad once and does what its presses mean.
func (n *Nav) poll(now time.Time) {
	if s, ok := n.pad(); ok {
		for _, a := range n.tracker.update(s, now) {
			n.Apply(a)
		}
	}
}

// NavButton is a button that reacts to the dialog's navigation keys while it
// has the focus.
type NavButton struct {
	widget.Button
	nav *Nav
}

// TypedKey implements fyne.Focusable.
func (b *NavButton) TypedKey(ev *fyne.KeyEvent) {
	if a := actionForKey(ev.Name); a != PadNone && b.nav != nil {
		b.nav.Apply(a)
		return
	}
	b.Button.TypedKey(ev)
}

// Button makes a button of the dialog.
func (n *Nav) Button(label string, importance widget.Importance, tapped func()) *NavButton {
	b := &NavButton{nav: n}
	b.Text, b.Importance, b.OnTapped = label, importance, tapped
	b.ExtendBaseWidget(b)
	return b
}
