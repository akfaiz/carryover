package kit

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/widget"
)

// Tappable makes any object clickable. It is highlighted while the mouse is
// over it, so it reads as clickable like a button does. It takes the focus
// too (Tab, a gamepad's D-pad), shown as a ring, and Space or Enter press it.
type Tappable struct {
	widget.BaseWidget
	content fyne.CanvasObject
	onTap   func()
	hover   *canvas.Rectangle
	ring    *canvas.Rectangle
}

func NewTappable(content fyne.CanvasObject, onTap func()) *Tappable {
	t := &Tappable{content: content, onTap: onTap, hover: canvas.NewRectangle(color.Transparent), ring: canvas.NewRectangle(color.Transparent)}
	t.hover.CornerRadius = 8
	t.ring.CornerRadius = 8
	t.ring.StrokeWidth = 2
	t.ExtendBaseWidget(t)
	return t
}

func (t *Tappable) Tapped(*fyne.PointEvent) {
	if t.onTap != nil {
		t.onTap()
	}
}

func (t *Tappable) Cursor() desktop.Cursor { return desktop.PointerCursor }

// MouseIn implements desktop.Hoverable.
func (t *Tappable) MouseIn(*desktop.MouseEvent) {
	t.hover.FillColor = ColHover
	t.hover.Refresh()
}

// MouseMoved implements desktop.Hoverable.
func (t *Tappable) MouseMoved(*desktop.MouseEvent) {}

// MouseOut implements desktop.Hoverable.
func (t *Tappable) MouseOut() {
	t.hover.FillColor = color.Transparent
	t.hover.Refresh()
}

// FocusGained implements fyne.Focusable.
func (t *Tappable) FocusGained() {
	t.ring.StrokeColor = ColAccent
	t.ring.FillColor = ColHover
	t.ring.Refresh()
}

// FocusLost implements fyne.Focusable.
func (t *Tappable) FocusLost() {
	t.ring.StrokeColor = color.Transparent
	t.ring.FillColor = color.Transparent
	t.ring.Refresh()
}

// TypedKey implements fyne.Focusable: Space and Enter press it; the arrow keys
// and Escape move on, like a gamepad's D-pad and B, in a window with a
// Navigator.
func (t *Tappable) TypedKey(ev *fyne.KeyEvent) {
	switch ev.Name {
	case fyne.KeySpace, fyne.KeyReturn, fyne.KeyEnter:
		t.Tapped(nil)
	default:
		if n := navigatorOf(t); n != nil {
			n.HandleKey(ev)
		}
	}
}

// TypedRune implements fyne.Focusable.
func (t *Tappable) TypedRune(rune) {}

func (t *Tappable) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(container.NewStack(t.hover, t.ring, t.content))
}
