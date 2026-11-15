package kit

import (
	"reflect"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver"
	"fyne.io/fyne/v2/driver/mobile"
	"fyne.io/fyne/v2/widget"
)

// A Navigator lets a whole window be used with a gamepad, as in Steam Deck
// Game Mode where Carryover is a non-Steam game without a mouse. The D-pad (or
// the left stick) moves the focus to the control on screen in that direction,
// A presses it, B goes back or closes the dialog, LB/RB switch tabs or
// sections, X searches, Y syncs and the right stick scrolls. The arrow keys,
// Enter and Escape do the same where Fyne lets them through: while nothing is
// focused, and on the kit's own controls (Tappable, NavButton).
//
// What the keys do on a page is up to the app (NavigatorOptions); in a dialog
// B closes it, the safe way: a dialog registers what that is with OnBack, and
// one that didn't is just hidden. A Nav (padnav.go) is such a dialog too.
type Navigator struct {
	win  fyne.Window
	opts NavigatorOptions
	pad  PadSource

	tracker padTracker
	scopes  map[fyne.CanvasObject]*navScope // by overlay
	stop    chan struct{}
	present bool // a gamepad is connected
	idle    int  // polls to skip while there is no gamepad
	// last is where the navigator last put the focus, and in which root: a
	// page that redraws itself replaces the focused control, and the focus
	// goes back to the one now in its place (see keepFocus).
	last     navRect
	lastRoot fyne.CanvasObject
	polls    int
}

// NavigatorOptions says what the gamepad does on the window's pages (not in a
// dialog). Every hook is optional.
type NavigatorOptions struct {
	Back         func()                 // B with no dialog open
	Section      func(dir int)          // LB (-1) and RB (1) when there are no tabs to switch
	Search       func()                 // X
	Sync         func()                 // Y
	TextField    func(f fyne.Focusable) // A on a text field: e.g. open an on-screen keyboard
	DefaultFocus func() fyne.Focusable  // where a first press starts on a page
	PadChanged   func(present bool)     // a gamepad was connected or disconnected
}

// navScope is what B does in one dialog, and what else a press there runs.
type navScope struct {
	back    func()
	onInput func()
	nav     *Nav
}

const (
	padIdlePolls  = 12 // without a gamepad, look for one every 12 polls (~0.5s)
	padKeepPolls  = 5  // check that the focus survived a redraw every 5 polls (~0.2s)
	padScrollStep = 22 // pixels per poll at full right-stick deflection
	padKeepRadius = 48 // how far a redrawn control may have moved and still get the focus back
)

var (
	navigatorsMu sync.Mutex
	navigators   = map[fyne.Canvas]*Navigator{}
)

// NavigatorFor returns the Navigator of the window that c belongs to, or nil.
func NavigatorFor(c fyne.Canvas) *Navigator {
	if c == nil {
		return nil
	}
	navigatorsMu.Lock()
	defer navigatorsMu.Unlock()
	return navigators[c]
}

func navigatorOf(o fyne.CanvasObject) *Navigator {
	app := fyne.CurrentApp()
	if app == nil {
		return nil
	}
	return NavigatorFor(app.Driver().CanvasForObject(o))
}

// NewNavigator makes the gamepad navigation of win. It reads the gamepad once
// SetActive(true) is called.
func NewNavigator(win fyne.Window, opts NavigatorOptions) *Navigator {
	n := &Navigator{win: win, opts: opts, scopes: map[fyne.CanvasObject]*navScope{}}
	if _, native := win.(driver.NativeWindow); native {
		n.pad = glfwPad // not in Fyne's headless test driver
	}
	navigatorsMu.Lock()
	navigators[win.Canvas()] = n
	navigatorsMu.Unlock()
	return n
}

// SetActive starts or stops reading the gamepad. Only the window in front
// should: a game started from Carryover is played with the same controller.
func (n *Navigator) SetActive(on bool) {
	if !on {
		if n.stop != nil {
			close(n.stop)
			n.stop = nil
		}
		return
	}
	if n.stop != nil || n.pad == nil {
		return
	}
	stop := make(chan struct{})
	n.stop = stop
	// What is held now (the A that started Carryover from Steam, a stick
	// leaning on the way back from the game) must not count as a press.
	n.idle = 0
	n.tracker = padTracker{}
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

// PadPresent reports whether a gamepad is connected (as far as the last poll
// saw).
func (n *Navigator) PadPresent() bool { return n.present }

// poll reads the gamepad once and does what its presses mean.
func (n *Navigator) poll(now time.Time) {
	if !n.present && n.idle > 0 {
		n.idle--
		return
	}
	s, ok := n.pad()
	if ok != n.present {
		n.present = ok
		if ok {
			n.tracker = padTracker{}
			n.tracker.prime(s, now)
		}
		if n.opts.PadChanged != nil {
			n.opts.PadChanged(ok)
		}
	}
	if !ok {
		n.idle = padIdlePolls
		return
	}
	for _, a := range n.tracker.update(s, now) {
		n.apply(a, true)
	}
	if s.Scroll != 0 {
		n.scrollBy(s.Scroll * padScrollStep)
	}
	if n.polls++; n.polls%padKeepPolls == 0 {
		n.keepFocus()
	}
}

// keepFocus gives the focus back after the page redrew the focused control:
// to the control now nearest the place it had.
func (n *Navigator) keepFocus() {
	root, _, _ := n.root()
	if root == nil || root != n.lastRoot {
		return
	}
	tree := walkNav(root)
	if tree.index(n.win.Canvas().Focused()) >= 0 {
		return
	}
	if i := tree.nearest(n.last, padKeepRadius); i >= 0 {
		n.win.Canvas().Focus(tree.targets[i].obj)
	}
}

// HandleKey does what an arrow key, Enter or Escape means when it reaches the
// window (nothing is focused) and reports whether it was one of those.
func (n *Navigator) HandleKey(ev *fyne.KeyEvent) bool {
	a := directionForKey(ev.Name)
	if a == PadNone {
		return false
	}
	n.apply(a, false)
	return true
}

// Apply does what a gamepad press means.
func (n *Navigator) Apply(a PadAction) { n.apply(a, true) }

// OnBack sets what B (and Escape) does while the dialog shown last is open,
// e.g. its Cancel. Call it right after showing the dialog.
func OnBack(win fyne.Window, back func()) {
	c := win.Canvas()
	if n := NavigatorFor(c); n != nil {
		if top := c.Overlays().Top(); top != nil {
			n.setScope(top, &navScope{back: back})
		}
	}
}

// FocusSafe focuses obj, the button a press should answer a dialog with
// (Cancel, Keep), when a gamepad is connected. Without one the focus stays
// where it is, so nothing looks pre-selected to a mouse user.
func FocusSafe(win fyne.Window, obj fyne.Focusable) {
	c := win.Canvas()
	if n := NavigatorFor(c); n != nil && n.present && obj != nil {
		c.Focus(obj)
	}
}

func (n *Navigator) setScope(overlay fyne.CanvasObject, s *navScope) {
	if overlay == nil {
		return
	}
	n.pruneScopes()
	n.scopes[overlay] = s
}

func (n *Navigator) dropScope(nav *Nav) {
	for o, s := range n.scopes {
		if s.nav == nav {
			delete(n.scopes, o)
		}
	}
}

// pruneScopes forgets the dialogs that are no longer shown.
func (n *Navigator) pruneScopes() {
	open := map[fyne.CanvasObject]bool{}
	for _, o := range n.win.Canvas().Overlays().List() {
		open[o] = true
	}
	for o := range n.scopes {
		if !open[o] {
			delete(n.scopes, o)
		}
	}
}

// root is what the presses act on: the dialog in front, or the window's
// content. scope is set for a dialog that said what B does.
func (n *Navigator) root() (root fyne.CanvasObject, overlay bool, scope *navScope) {
	c := n.win.Canvas()
	if top := c.Overlays().Top(); top != nil {
		return top, true, n.scopes[top]
	}
	return c.Content(), false, nil
}

func (n *Navigator) apply(a PadAction, fromPad bool) {
	root, overlay, scope := n.root()
	if root == nil {
		return
	}
	if scope != nil && scope.onInput != nil {
		scope.onInput()
	}
	c := n.win.Canvas()
	focused := c.Focused()
	switch a {
	case PadUp, PadDown, PadLeft, PadRight:
		if focused != nil && ownsKey(focused, a) {
			// A list or grid moves its own highlight; at its edge, where it
			// stays put, the D-pad leaves it for the control that way.
			before, ok := highlightOf(focused)
			focused.TypedKey(&fyne.KeyEvent{Name: keyOf(a)})
			if after, _ := highlightOf(focused); !ok || after != before || isMenu(focused) {
				return
			}
		}
		n.move(root, overlay, focused, a)
	case PadAccept:
		n.accept(root, overlay, focused, fromPad)
	case PadBack:
		switch {
		case isMenu(focused):
			focused.TypedKey(&fyne.KeyEvent{Name: fyne.KeyEscape})
		case overlay && scope != nil && scope.back != nil:
			scope.back()
		case overlay:
			n.hideOverlay(root)
		case n.opts.Back != nil:
			n.opts.Back()
		}
	case PadTabPrev, PadTabNext:
		dir := 1
		if a == PadTabPrev {
			dir = -1
		}
		if tabs := findTabs(root); tabs != nil {
			if k := len(tabs.Items); k > 0 {
				tabs.SelectIndex((tabs.SelectedIndex() + dir + k) % k)
			}
		} else if !overlay && n.opts.Section != nil {
			n.opts.Section(dir)
		}
	case PadSearch:
		if !overlay && n.opts.Search != nil {
			n.opts.Search()
		}
	case PadSync:
		if !overlay && n.opts.Sync != nil {
			n.opts.Sync()
		}
	}
}

// move takes the focus to the control in direction a. When nothing in root
// has it yet, the first press only picks where to start.
func (n *Navigator) move(root fyne.CanvasObject, overlay bool, focused fyne.Focusable, a PadAction) {
	tree := walkNav(root)
	cur := tree.index(focused)
	if cur < 0 && root == n.lastRoot {
		cur = tree.nearest(n.last, padKeepRadius) // redrawn: carry on from its place
	}
	if cur < 0 {
		n.focusStart(root, overlay, tree)
		return
	}
	next := tree.pick(cur, a)
	if next < 0 && overlay {
		// Nothing that way in a dialog: go round, in Tab order, like a Nav.
		k := len(tree.targets)
		if a.linear() == PadNext {
			next = (cur + 1) % k
		} else {
			next = (cur - 1 + k) % k
		}
	}
	if next >= 0 && next != cur {
		n.focusTarget(tree.targets[next])
	}
}

// FocusDefault focuses where the page wants a gamepad to start (or the top
// left control). It does nothing while a dialog is open.
func (n *Navigator) FocusDefault() {
	root, overlay, _ := n.root()
	if root == nil || overlay {
		return
	}
	n.focusStart(root, false, walkNav(root))
}

// Focus focuses obj and scrolls it into view, as a D-pad press onto it would.
func (n *Navigator) Focus(obj fyne.Focusable) {
	root, _, _ := n.root()
	tree := walkNav(root)
	if i := tree.index(obj); i >= 0 {
		n.focusTarget(tree.targets[i])
	} else if obj != nil {
		n.win.Canvas().Focus(obj)
	}
}

func (n *Navigator) focusStart(root fyne.CanvasObject, overlay bool, tree navTree) {
	if !overlay && n.opts.DefaultFocus != nil {
		if i := tree.index(n.opts.DefaultFocus()); i >= 0 {
			n.focusTarget(tree.targets[i])
			return
		}
	}
	if i := tree.topLeft(); i >= 0 {
		n.focusTarget(tree.targets[i])
	}
}

func (n *Navigator) focusTarget(t navTarget) {
	n.win.Canvas().Focus(t.obj)
	n.last, n.lastRoot = scrollIntoView(t), n.rootNow()
}

func (n *Navigator) rootNow() fyne.CanvasObject {
	r, _, _ := n.root()
	return r
}

// accept presses the focused control the way the space bar does, which every
// Fyne control answers to: buttons, checks, selects, radio items, links,
// lists, menus. A text field instead gets typed into.
func (n *Navigator) accept(root fyne.CanvasObject, overlay bool, focused fyne.Focusable, fromPad bool) {
	tree := walkNav(root)
	if focused == nil || (tree.index(focused) < 0 && !isMenu(focused)) {
		n.focusStart(root, overlay, tree) // first press: only show where we are
		return
	}
	if isTextField(focused) {
		if fromPad && n.opts.TextField != nil {
			n.opts.TextField(focused)
		}
		return
	}
	if isMenu(focused) {
		focused.TypedKey(&fyne.KeyEvent{Name: fyne.KeyReturn})
		return
	}
	focused.TypedKey(&fyne.KeyEvent{Name: fyne.KeySpace})
	focused.TypedRune(' ')
}

// scrollBy scrolls what holds the focus, or else the biggest scroll in view.
func (n *Navigator) scrollBy(dy float32) {
	root, _, _ := n.root()
	if root == nil {
		return
	}
	tree := walkNav(root)
	var f *scrollFrame
	if i := tree.index(n.win.Canvas().Focused()); i >= 0 && len(tree.targets[i].scrolls) > 0 {
		f = &tree.targets[i].scrolls[len(tree.targets[i].scrolls)-1]
	} else {
		var area float32
		for i, s := range tree.scrolls {
			if sz := s.s.Size(); sz.Width*sz.Height > area {
				area, f = sz.Width*sz.Height, &tree.scrolls[i]
			}
		}
	}
	if f == nil {
		return
	}
	s := f.s
	maxY := max(0, s.Content.Size().Height-s.Size().Height)
	if y := clamp(s.Offset.Y+dy, 0, maxY); y != s.Offset.Y {
		s.ScrollToOffset(fyne.NewPos(s.Offset.X, y))
		s.Refresh()
	}
}

// hideOverlay closes a dialog that didn't say how (OnBack). Fyne wraps a
// pop-up (which a dialog is) in an overlay container of its own: hide the
// pop-up, which takes the overlay down with it.
func (n *Navigator) hideOverlay(top fyne.CanvasObject) {
	var pop *widget.PopUp
	var walk func(o fyne.CanvasObject, depth int)
	walk = func(o fyne.CanvasObject, depth int) {
		if pop != nil || o == nil || depth > 3 {
			return
		}
		if p, ok := o.(*widget.PopUp); ok {
			pop = p
			return
		}
		for _, ch := range childrenOf(o) {
			walk(ch, depth+1)
		}
	}
	walk(top, 0)
	if pop != nil {
		pop.Hide()
		return
	}
	n.win.Canvas().Overlays().Remove(top)
}

// childrenOf returns the objects directly inside o.
func childrenOf(o fyne.CanvasObject) []fyne.CanvasObject {
	switch c := o.(type) {
	case *fyne.Container:
		return c.Objects
	case fyne.Widget:
		if r := widgetRenderer(c); r != nil {
			return r.Objects()
		}
	}
	return nil
}

// findTabs returns the first tab container shown under root, or nil.
func findTabs(root fyne.CanvasObject) *container.AppTabs {
	var found *container.AppTabs
	var walk func(o fyne.CanvasObject)
	walk = func(o fyne.CanvasObject) {
		if found != nil || o == nil || !o.Visible() {
			return
		}
		if t, ok := o.(*container.AppTabs); ok {
			found = t
			return
		}
		for _, ch := range childrenOf(o) {
			walk(ch)
		}
	}
	walk(root)
	return found
}

// isTextField reports whether f takes typed text (an entry of any kind).
func isTextField(f fyne.Focusable) bool {
	_, ok := f.(mobile.Keyboardable)
	return ok
}

func isMenu(f fyne.Focusable) bool {
	_, ok := f.(*widget.PopUpMenu)
	return ok
}

// ownsKey reports whether the focused control uses the direction itself: an
// open menu, a list moving its highlight up and down, a grid (the file
// picker's) in every direction. Anything else (a text field too: a gamepad
// doesn't move a cursor) lets the focus move on.
func ownsKey(f fyne.Focusable, a PadAction) bool {
	switch f.(type) {
	case *widget.PopUpMenu, *widget.GridWrap:
		return true
	case *widget.List:
		return a == PadUp || a == PadDown
	}
	return false
}

// highlightOf reads which item of a list or grid is highlighted, to tell
// whether a key moved it. Fyne keeps it unexported; ok is false if it can't be
// read (a menu, or a Fyne release that renamed it: TestHighlightOf fails then).
func highlightOf(f fyne.Focusable) (id int64, ok bool) {
	switch f.(type) {
	case *widget.List, *widget.GridWrap:
	default:
		return 0, false
	}
	v := reflect.ValueOf(f).Elem().FieldByName("currentHighlight")
	if !v.IsValid() || !v.CanInt() {
		return 0, false
	}
	return v.Int(), true
}

func keyOf(a PadAction) fyne.KeyName {
	switch a {
	case PadUp:
		return fyne.KeyUp
	case PadDown:
		return fyne.KeyDown
	case PadLeft:
		return fyne.KeyLeft
	}
	return fyne.KeyRight
}
