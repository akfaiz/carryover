package kit

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
)

// ringGap is the space between a control and the gamepad's focus ring around
// it; ringWidth is the ring's line.
const (
	ringGap   = 3
	ringWidth = 3
)

// A focusRing shows which control a gamepad is on: a ring around it, apart
// from the control's own look. Fyne fills a focused button with the focus
// colour, which is the primary colour: a focused Play button looks just like
// one that isn't.
//
// The ring lies in a layer over a window's content or a dialog's body, which
// must be stacked on top of it (Navigator.RingLayer, ShowCustom).
type focusRing struct {
	layer *fyne.Container // no layout: the stack sizes it, the ring is placed by hand
	rect  *canvas.Rectangle
}

func newFocusRing() *focusRing {
	r := canvas.NewRectangle(color.Transparent)
	r.StrokeWidth = ringWidth
	r.Hide()
	return &focusRing{layer: container.NewWithoutLayout(r), rect: r}
}

func (r *focusRing) hide() {
	if r.rect.Visible() {
		r.rect.Hide()
	}
}

// show puts the ring around rect, given in the layer's coordinates.
func (r *focusRing) show(rect navRect) {
	pos := fyne.NewPos(rect.X-ringGap, rect.Y-ringGap)
	size := fyne.NewSize(rect.W+2*ringGap, rect.H+2*ringGap)
	radius := theme.InputRadiusSize() + ringGap
	if r.rect.Visible() && r.rect.Position() == pos && r.rect.Size() == size &&
		r.rect.StrokeColor == ColAccent && r.rect.CornerRadius == radius {
		return
	}
	r.rect.StrokeColor = ColAccent
	r.rect.CornerRadius = radius
	r.rect.Move(pos)
	r.rect.Resize(size)
	r.rect.Show()
	r.rect.Refresh()
}

// ownRing is a control that shows its focus well enough itself (a Tappable's
// ring).
type ownRing interface{ drawsFocus() }

func (t *Tappable) drawsFocus() {}

// RingLayer is the layer the focus ring is drawn in on the window's pages: stack
// it over the content, e.g. container.NewStack(content, nav.RingLayer()).
func (n *Navigator) RingLayer() fyne.CanvasObject {
	if n.mainRing == nil {
		n.mainRing = newFocusRing()
	}
	return n.mainRing.layer
}

// ringedBody stacks a focus ring layer over a dialog's body; register, called
// right after the dialog is shown, gives it to the dialog.
func ringedBody(win fyne.Window, body fyne.CanvasObject) (wrapped fyne.CanvasObject, register func()) {
	n := NavigatorFor(win.Canvas())
	if n == nil {
		return body, func() {}
	}
	r := newFocusRing()
	return container.NewStack(body, r.layer), func() {
		if top := win.Canvas().Overlays().Top(); top != nil {
			n.pruneScopes()
			n.rings[top] = r
			n.updateRing()
		}
	}
}

// updateRing draws the focus ring around the focused control, in the dialog in
// front or on the page, while a gamepad is connected; a mouse or keyboard user
// gets Fyne's own look only.
func (n *Navigator) updateRing() {
	root, overlay, _ := n.root()
	var r *focusRing
	if overlay {
		r = n.rings[root]
	} else {
		r = n.mainRing
	}
	if n.shown != nil && n.shown != r {
		n.shown.hide()
	}
	n.shown = r
	if r == nil {
		return
	}
	focused := n.win.Canvas().Focused()
	if _, own := focused.(ownRing); !n.present || focused == nil || own {
		r.hide()
		return
	}
	tree := walkNav(root)
	i := tree.index(focused)
	if i < 0 {
		r.hide()
		return
	}
	rect, ok := tree.targets[i].visibleRect()
	if !ok {
		r.hide()
		return
	}
	// walkNav measures from the canvas, like the layer's absolute position.
	if app := fyne.CurrentApp(); app != nil {
		at := app.Driver().AbsolutePositionForObject(r.layer)
		rect.X -= at.X
		rect.Y -= at.Y
	}
	r.show(rect)
}

// visibleRect is the part of the target its scrolls show, or false when they
// show none of it.
func (t navTarget) visibleRect() (navRect, bool) {
	x0, y0, x1, y1 := t.rect.X, t.rect.Y, t.rect.X+t.rect.W, t.rect.Y+t.rect.H
	for _, f := range t.scrolls {
		v := f.s.Size()
		x0, y0 = max(x0, f.pos.X), max(y0, f.pos.Y)
		x1, y1 = min(x1, f.pos.X+v.Width), min(y1, f.pos.Y+v.Height)
	}
	if x1 <= x0 || y1 <= y0 {
		return navRect{}, false
	}
	return navRect{x0, y0, x1 - x0, y1 - y0}, true
}
