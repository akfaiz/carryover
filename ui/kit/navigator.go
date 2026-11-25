package kit

import (
	"time"

	"github.com/egoist/mygo/ui"
)

// Navigator moves the focus with a gamepad (and the arrow keys left over)
// across the controls a page declares, MyGo has no way to list the focusable controls of a window, so a
// view registers each control it wants the D-pad to reach with Item while it
// builds; the Navigator picks the next one in a direction from the boxes the
// last frame laid out (ui.Handle.Bounds).
//
// A dialog is a scope: while one is open (Overlay), only its controls are
// reachable and B runs its back. Everything runs on the UI goroutine.
//
// Per frame, a window's root view calls Begin, then registers its controls, and
// wraps a dialog's content in Overlay. A pad's presses arrive through Press
// (see StartPad).
type Navigator struct {
	opts NavigatorOptions

	// root is the page's scope; overlays are the dialogs open in the frame
	// being built, in order. prev is the last frame's, whose boxes are
	// committed.
	root, prev scope
	overlays   []*scope
	prevTop    *scope
	building   *scope
	ctx        *ui.Context

	pending []PadAction
	scroll  *ui.ScrollState
	present bool
	// revealing is the control the D-pad just moved to, which Begin makes room
	// around for the next few frames (see makeRoom); frames counts them.
	revealing *ui.Handle
	frames    int
	// padUsed is set by a pad press and cleared when the pad goes away: while
	// set, the focused control gets a ring even though the focus did not come
	// from the keyboard, which is all MyGo rings by itself.
	padUsed bool
	ringed  *ui.Handle // the control the last frame drew the ring on

	handles map[any]*ui.Handle
	opened  map[any]*bool // whether each select's list is open (see SelectOf)
	// OnInput, when set, runs on every press the Navigator acts on (a prompt
	// with a countdown stops it: whoever moves between the choices is choosing).
	OnInput func()

	tracker padTracker
}

// NavigatorOptions says what the gamepad does on a page (not in a dialog).
// Every hook is optional.
type NavigatorOptions struct {
	Back       func()             // B with no dialog open
	Section    func(dir int)      // LB (-1) and RB (1)
	Search     func()             // X
	Sync       func()             // Y
	View       func()             // the View button, also in a dialog
	TextField  func()             // A on a text field: e.g. open an on-screen keyboard
	PadChanged func(present bool) // a gamepad was connected or disconnected
	// DefaultFocus is where a first press starts on a page: nil for the page's
	// top left control.
	DefaultFocus func() *ui.Handle
}

// scope is the set of controls one layer (the page or a dialog) offers.
type scope struct {
	id     string
	items  []navItem
	back   func()
	safe   int // index of the item that starts focused, -1 for the first
	pageAt int // index of the first item of the page itself (after the window's own), -1 if unmarked
}

type navItem struct {
	h      *ui.Handle
	action func()
	text   bool
}

// NewNavigator returns a Navigator with the page's hooks.
func NewNavigator(opts NavigatorOptions) *Navigator {
	return &Navigator{opts: opts, root: scope{safe: -1, pageAt: -1}, prev: scope{safe: -1, pageAt: -1}}
}

// Handle returns the handle kept under key, made on first use: for controls
// a page draws in a loop (one per game, per row), where a field per control
// does not fit. The key identifies the control (an id, a struct of ids).
func (n *Navigator) Handle(key any) *ui.Handle {
	if n.handles == nil {
		n.handles = map[any]*ui.Handle{}
	}
	h, ok := n.handles[key]
	if !ok {
		h = new(ui.Handle)
		n.handles[key] = h
	}
	return h
}

// popup returns the flag that says a select's list is open, made on first use:
// the list stays open across frames until something picks or closes it.
func (n *Navigator) popup(key any) *bool {
	if n.opened == nil {
		n.opened = map[any]*bool{}
	}
	p, ok := n.opened[key]
	if !ok {
		p = new(bool)
		n.opened[key] = p
	}
	return p
}

// SetOptions replaces the hooks (the shell sets them once it exists).
func (n *Navigator) SetOptions(opts NavigatorOptions) { n.opts = opts }

// ItemOption changes how Item registers a control.
type ItemOption int

// FocusSafe marks the control a dialog starts on: one that never overwrites
// saves (an overwriting action is never the default).
const FocusSafe ItemOption = 1

// TextField marks a text input: A on it runs NavigatorOptions.TextField (open
// Steam's on-screen keyboard) instead of the item's action.
const TextField ItemOption = 2

// Begin starts a frame: it runs the presses queued since the last one against
// the last frame's layout, then starts the new registry. Call it first in the
// window's view.
func (n *Navigator) Begin(c *ui.Context) {
	n.prev, n.root = n.root, scope{items: n.prev.items[:0], safe: -1, pageAt: -1}
	n.prevTop = n.top(n.overlays)
	n.overlays = nil
	n.building = &n.root
	n.ctx = c
	pending := n.pending
	n.pending = nil
	for _, a := range pending {
		n.apply(c, a)
	}
	n.makeRoom(c)
}

// focusRoom is the room left between a control the D-pad moved to and the edge
// of the page's scroll container: MyGo scrolls a focused control into view and
// no more, so its ring would sit on the edge, cut off by it or by what is
// beside the container (the gamepad's hints, the title above it).
const focusRoom float32 = S3

// makeRoom scrolls the page by what keeps focusRoom around the control the D-pad
// moved to. The frame after the move laid it out where MyGo scrolled it to, so
// its box, as the last frame committed it, is where it shows; a few frames are
// watched because the scroll takes one or two to show. A control that is not in
// the scrolling area (the filters and the search above a grid, the sidebar) is
// left alone: there is nothing to scroll for it, and the room it would "need"
// is however far it is from the area.
func (n *Navigator) makeRoom(c *ui.Context) {
	if n.revealing == nil || n.scroll == nil || n.frames <= 0 {
		n.revealing = nil
		return
	}
	n.frames--
	box, view := n.revealing.Bounds(c), n.scroll.Bounds(c)
	if box.H <= 0 || view.H <= 0 || box.H+2*focusRoom > view.H {
		return // not laid out yet, or too tall to keep the room around
	}
	if box.X+box.W <= view.X || box.X >= view.X+view.W || box.Y+box.H <= view.Y || box.Y >= view.Y+view.H {
		return // not in the area
	}
	y := n.scroll.Y
	if over := view.Y + focusRoom - box.Y; over > 0 {
		y -= over
	} else if over := box.Y + box.H - (view.Y + view.H - focusRoom); over > 0 {
		y += over
	}
	n.scroll.Y = min(max(y, 0), max(n.scroll.MaxY, 0))
}

// End finishes the frame's registry: a dialog that opened in it gets the
// focus on its default control, next frame (Handle.Focus waits for the
// control to exist). Call it last in the window's view.
func (n *Navigator) End(c *ui.Context) {
	if top := n.top(n.overlays); top != nil && (n.prevTop == nil || n.prevTop.id != top.id) {
		if it := top.defaultItem(); it != nil {
			it.h.Focus(c)
		}
	}
}

func (n *Navigator) top(o []*scope) *scope {
	if len(o) == 0 {
		return nil
	}
	return o[len(o)-1]
}

// MarkPage says the controls registered from here on belong to the page, not
// to the window around it: a first press starts on the page's.
func (n *Navigator) MarkPage() { n.building.pageAt = len(n.building.items) }

// Item registers el as a control the D-pad can reach; action is what A does
// on it (nil: nothing beyond what the control's own click does is needed, so A
// leaves it to it). It returns el bound to h.
func (n *Navigator) Item(h *ui.Handle, el ui.Element, action func(), opts ...ItemOption) ui.Element {
	s := n.building
	text := false
	for _, o := range opts {
		switch o {
		case FocusSafe:
			s.safe = len(s.items)
		case TextField:
			text = true
		}
	}
	s.items = append(s.items, navItem{h: h, action: action, text: text})
	if n.padUsed && n.ctx != nil && h.Focused(n.ctx) && !el.FocusVisible() {
		n.ringed = h
		// A shadow with a spread is a ring outside the box: it changes no layout.
		el = el.Shadow(0, 0, 0, 2, col(ColAccent))
	}
	return el.Bind(h)
}

// Overlay declares a dialog's controls: while open is true, fn runs with the
// dialog as the scope controls register in, back is what B does there, and
// the page behind is out of reach. id tells dialogs apart (a new one gets the
// focus when it opens).
func (n *Navigator) Overlay(id string, open bool, back func(), fn func()) {
	if !open {
		return
	}
	s := &scope{id: id, back: back, safe: -1, pageAt: -1}
	n.overlays = append(n.overlays, s)
	outer := n.building
	n.building = s
	fn()
	n.building = outer
}

func (s *scope) defaultItem() *navItem {
	if len(s.items) == 0 {
		return nil
	}
	if s.safe >= 0 && s.safe < len(s.items) {
		return &s.items[s.safe]
	}
	return &s.items[0]
}

// active layer for acting on a press: the topmost dialog of the last frame,
// else the page.
func (n *Navigator) layer() *scope {
	if n.prevTop != nil {
		return n.prevTop
	}
	return &n.prev
}

// Press queues a press from any goroutine's Window.Update (or the UI
// goroutine); it runs at the start of the next frame.
func (n *Navigator) Press(a PadAction) {
	n.padUsed = true
	n.pending = append(n.pending, a)
}

// Apply runs a press now, as Begin would; for tests and the arrow keys.
func (n *Navigator) Apply(c *ui.Context, a PadAction) { n.apply(c, a) }

// PadPresent reports whether a gamepad is connected.
func (n *Navigator) PadPresent() bool { return n.present }

// Keys turns the arrow keys nothing else took into D-pad presses, for a
// controller Steam maps to keys. Call it in the window's view after the
// controls were built, so a text field or list has had its chance.
func (n *Navigator) Keys(c *ui.Context) {
	for _, k := range []struct {
		key ui.Key
		a   PadAction
	}{{ui.KeyUp, PadUp}, {ui.KeyDown, PadDown}, {ui.KeyLeft, PadLeft}, {ui.KeyRight, PadRight}} {
		if c.Shortcut(0, k.key) {
			n.apply(c, k.a)
		}
	}
}

// SetScroll sets the page's main scroll container, which the right stick
// scrolls; nil when the page has none.
func (n *Navigator) SetScroll(s *ui.ScrollState) { n.scroll = s }

func (n *Navigator) apply(c *ui.Context, a PadAction) {
	if n.OnInput != nil {
		n.OnInput()
	}
	l := n.layer()
	switch a {
	case PadUp, PadDown, PadLeft, PadRight:
		n.move(c, l, a)
	case PadAccept:
		for _, it := range l.items {
			if it.h.Focused(c) {
				switch {
				case it.text && n.opts.TextField != nil:
					n.opts.TextField()
				case it.action != nil:
					it.action()
				}
				return
			}
		}
		// Nothing has the focus: the first press only picks where to start,
		// so a stray A (still held from the press that opened a dialog)
		// can't answer it.
		n.move(c, l, PadDown)
	case PadBack:
		switch {
		case n.prevTop != nil && l.back != nil:
			l.back()
		case n.prevTop == nil && n.opts.Back != nil:
			n.opts.Back()
		}
	case PadTabPrev, PadTabNext:
		if n.prevTop == nil && n.opts.Section != nil {
			dir := 1
			if a == PadTabPrev {
				dir = -1
			}
			n.opts.Section(dir)
		}
	case PadSearch:
		if n.prevTop == nil && n.opts.Search != nil {
			n.opts.Search()
		}
	case PadSync:
		if n.prevTop == nil && n.opts.Sync != nil {
			n.opts.Sync()
		}
	case PadView:
		if n.opts.View != nil {
			n.opts.View()
		}
	}
}

// move takes the focus to the control in direction a. When nothing in the
// layer has it yet, the first press only picks where to start.
func (n *Navigator) move(c *ui.Context, l *scope, a PadAction) {
	if len(l.items) == 0 {
		return
	}
	cur := -1
	rects := make([]ui.Rect, len(l.items))
	for i, it := range l.items {
		rects[i] = it.h.Bounds(c)
		if cur < 0 && it.h.Focused(c) {
			cur = i
		}
	}
	if cur < 0 {
		n.focusStart(c, l, rects)
		return
	}
	nr := make([]navRect, len(rects))
	for i, r := range rects {
		nr[i] = navRect{r.X, r.Y, r.W, r.H}
	}
	if next := pickInDirection(nr[cur], a, nr); next >= 0 {
		l.items[next].h.Focus(c) // focusing also scrolls it into view
		n.revealing, n.frames = l.items[next].h, 4
	}
}

// focusStart puts the focus where a layer starts: a dialog's safe control; on
// the page, its declared default, else its top left control, else the
// window's.
func (n *Navigator) focusStart(c *ui.Context, l *scope, rects []ui.Rect) {
	if n.prevTop != nil {
		if it := l.defaultItem(); it != nil {
			it.h.Focus(c)
			return
		}
	}
	if n.opts.DefaultFocus != nil {
		if h := n.opts.DefaultFocus(); h != nil {
			for _, it := range l.items {
				if it.h == h {
					h.Focus(c)
					return
				}
			}
		}
	}
	for _, from := range []int{max(l.pageAt, 0), 0} {
		if best := topLeft(rects, from); best >= 0 {
			l.items[best].h.Focus(c)
			return
		}
	}
}

// topLeft is the index (from index from on) of the control nearest the top
// left corner that has a box, or -1.
func topLeft(rects []ui.Rect, from int) int {
	best, bestScore := -1, float32(1e30)
	for i := from; i < len(rects); i++ {
		r := rects[i]
		if r.W == 0 || r.H == 0 {
			continue
		}
		if s := r.Y*4 + r.X; s < bestScore {
			best, bestScore = i, s
		}
	}
	return best
}

// FocusDefault puts the focus on the page's first control, as a first press
// does.
func (n *Navigator) FocusDefault(c *ui.Context) { n.apply(c, PadDown) }

// StartPad reads the gamepad src until the returned stop function is called,
// and queues what it presses. update runs a function on the UI goroutine and
// redraws (Window.Update). The reading is off the UI goroutine; presses held
// when it starts do not count.
func (n *Navigator) StartPad(src PadSource, update func(func())) (stop func()) {
	done := make(chan struct{})
	go func() {
		t := time.NewTicker(padPollEvery)
		defer t.Stop()
		var primed, present bool
		for {
			select {
			case <-done:
				return
			case now := <-t.C:
				s, ok := src()
				if ok != present {
					present = ok
					update(func() {
						n.present = ok
						if !ok {
							n.padUsed = false
						}
						if n.opts.PadChanged != nil {
							n.opts.PadChanged(ok)
						}
					})
				}
				if !ok {
					primed = false
					continue
				}
				if !primed {
					n.tracker.prime(s, now)
					primed = true
				}
				acts := n.tracker.update(s, now)
				if len(acts) > 0 || s.Scroll != 0 {
					scroll := s.Scroll
					update(func() {
						if len(acts) > 0 {
							n.padUsed = true
						}
						n.pending = append(n.pending, acts...)
						if scroll != 0 && n.scroll != nil && n.prevTop == nil {
							n.scroll.Y += scroll * padScrollStep
						}
					})
				}
			}
		}
	}()
	return func() { close(done) }
}

const padScrollStep = 22 // DIPs per poll at full right-stick deflection
