package kit

import (
	"math"
	"slices"
	_ "unsafe" // go:linkname, see widgetRenderer

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
)

// widgetRenderer is Fyne's own renderer cache lookup, which Fyne uses to walk
// the object tree (focus, taps, painting). Fyne exports no way to reach a
// widget's children, and the Navigator needs every focusable control on screen
// with its position, so it borrows this one. A Fyne release that renames it
// fails the build here rather than misbehaving.
//
//go:linkname widgetRenderer fyne.io/fyne/v2/internal/cache.Renderer
func widgetRenderer(wid fyne.Widget) fyne.WidgetRenderer

// navRect is an object's place on the canvas.
type navRect struct{ X, Y, W, H float32 }

func (r navRect) cx() float32 { return r.X + r.W/2 }
func (r navRect) cy() float32 { return r.Y + r.H/2 }

// scrollFrame is a scroll container an object sits in, and where it is.
type scrollFrame struct {
	s   *container.Scroll
	pos fyne.Position
}

// navTarget is a control the focus can move to.
type navTarget struct {
	obj     fyne.Focusable
	rect    navRect
	scrolls []scrollFrame // the scrolls it is in, outermost first
}

// navTree is what a walk of a scope found: its focusable controls in tree
// order (Fyne's Tab order) and its scroll containers.
type navTree struct {
	targets []navTarget
	scrolls []scrollFrame
}

// walkNav collects the visible, enabled focusable controls under root.
func walkNav(root fyne.CanvasObject) navTree {
	var t navTree
	var walk func(o fyne.CanvasObject, offset fyne.Position, scrolls []scrollFrame)
	walk = func(o fyne.CanvasObject, offset fyne.Position, scrolls []scrollFrame) {
		if o == nil || !o.Visible() {
			return
		}
		pos := offset.Add(o.Position())
		size := o.Size()
		if f, ok := o.(fyne.Focusable); ok && size.Width > 0 && size.Height > 0 && enabled(o) {
			t.targets = append(t.targets, navTarget{obj: f, rect: navRect{pos.X, pos.Y, size.Width, size.Height}, scrolls: scrolls})
		}
		if s, ok := o.(*container.Scroll); ok {
			f := scrollFrame{s: s, pos: pos}
			t.scrolls = append(t.scrolls, f)
			scrolls = append(slices.Clip(scrolls), f)
		}
		for _, ch := range childrenOf(o) {
			walk(ch, pos, scrolls)
		}
	}
	if root != nil {
		// Children are placed relative to their parent; root's own position
		// counts too (an overlay may sit anywhere on the canvas).
		walk(root, fyne.Position{}, nil)
	}
	return t
}

func enabled(o fyne.CanvasObject) bool {
	d, ok := o.(fyne.Disableable)
	return !ok || !d.Disabled()
}

// index is the position of obj among the targets, or -1.
func (t navTree) index(obj fyne.Focusable) int {
	if obj == nil {
		return -1
	}
	for i, x := range t.targets {
		if x.obj == obj {
			return i
		}
	}
	return -1
}

// topLeft is the target nearest the top left corner: where a first press
// starts when nothing has the focus.
func (t navTree) topLeft() int {
	best, bestScore := -1, float32(math.MaxFloat32)
	for i, x := range t.targets {
		// Rows first: a control a little lower but far to the left still comes
		// after the row above it.
		if s := x.rect.Y*4 + x.rect.X; s < bestScore {
			best, bestScore = i, s
		}
	}
	return best
}

// FirstFocusable returns the enabled control nearest the top left of root, or
// nil: where a gamepad starts in that part of the window.
func FirstFocusable(root fyne.CanvasObject) fyne.Focusable {
	t := walkNav(root)
	if i := t.topLeft(); i >= 0 {
		return t.targets[i].obj
	}
	return nil
}

// pickInDirection returns the candidate the focus should move to from from in
// direction dir (PadUp, PadDown, PadLeft or PadRight), or -1 if there is none.
// A candidate qualifies when it lies beyond from in that direction, overlapping
// it by at most half the smaller of the two along that axis. The nearest one
// wins, where being off to the side counts double: moving down from a tile
// prefers the button right below it over a closer one in the next column.
func pickInDirection(from navRect, dir PadAction, cands []navRect) int {
	best, bestScore := -1, float32(math.MaxFloat32)
	for i, c := range cands {
		if c == from {
			continue
		}
		var gap, side float32 // distance along the direction, and across it
		switch dir {
		case PadDown:
			if c.cy() <= from.cy() || c.Y < from.Y+from.H-min(from.H, c.H)/2 {
				continue
			}
			gap, side = max(0, c.Y-(from.Y+from.H)), spanGap(from.X, from.W, c.X, c.W)
		case PadUp:
			if c.cy() >= from.cy() || c.Y+c.H > from.Y+min(from.H, c.H)/2 {
				continue
			}
			gap, side = max(0, from.Y-(c.Y+c.H)), spanGap(from.X, from.W, c.X, c.W)
		case PadRight:
			if c.cx() <= from.cx() || c.X < from.X+from.W-min(from.W, c.W)/2 {
				continue
			}
			gap, side = max(0, c.X-(from.X+from.W)), spanGap(from.Y, from.H, c.Y, c.H)
		case PadLeft:
			if c.cx() >= from.cx() || c.X+c.W > from.X+min(from.W, c.W)/2 {
				continue
			}
			gap, side = max(0, from.X-(c.X+c.W)), spanGap(from.Y, from.H, c.Y, c.H)
		default:
			return -1
		}
		// Among equally good ones, the one whose middle lines up best.
		align := float32(math.Abs(float64(c.cx() - from.cx())))
		if dir == PadLeft || dir == PadRight {
			align = float32(math.Abs(float64(c.cy() - from.cy())))
		}
		if s := gap + 2*side + align/100; s < bestScore {
			best, bestScore = i, s
		}
	}
	return best
}

// spanGap is the distance between two spans on one axis, 0 if they overlap.
func spanGap(a, aLen, b, bLen float32) float32 {
	switch {
	case b > a+aLen:
		return b - (a + aLen)
	case a > b+bLen:
		return a - (b + bLen)
	}
	return 0
}

// nearest is the target whose middle is closest to r's, within radius, or -1.
func (t navTree) nearest(r navRect, radius float32) int {
	best, bestD := -1, radius*radius
	for i, x := range t.targets {
		dx, dy := x.rect.cx()-r.cx(), x.rect.cy()-r.cy()
		if d := dx*dx + dy*dy; d <= bestD {
			best, bestD = i, d
		}
	}
	return best
}

// scrollIntoView scrolls every scroll container target sits in so it shows,
// innermost first, with a small margin. It returns where target is then.
func scrollIntoView(target navTarget) navRect {
	const margin = 12
	r := target.rect
	for _, f := range slices.Backward(target.scrolls) {
		view := f.s.Size()
		content := f.s.Content.Size()
		off := f.s.Offset
		dx := scrollDelta(r.X-f.pos.X, r.W, view.Width, margin)
		dy := scrollDelta(r.Y-f.pos.Y, r.H, view.Height, margin)
		nx := clamp(off.X+dx, 0, max(0, content.Width-view.Width))
		ny := clamp(off.Y+dy, 0, max(0, content.Height-view.Height))
		if nx == off.X && ny == off.Y {
			continue
		}
		f.s.ScrollToOffset(fyne.NewPos(nx, ny))
		f.s.Refresh()
		// The target moved with the content: carry that to the outer scrolls.
		r.X -= nx - off.X
		r.Y -= ny - off.Y
	}
	return r
}

// scrollDelta is how far to scroll so a span at pos (relative to the view)
// of length size is inside a view of length view.
func scrollDelta(pos, size, view, margin float32) float32 {
	switch {
	case pos < margin || size > view-2*margin:
		return pos - margin // too tall to fit: show its top
	case pos+size > view-margin:
		return pos + size - (view - margin)
	}
	return 0
}

func clamp(v, lo, hi float32) float32 { return max(lo, min(v, hi)) }
