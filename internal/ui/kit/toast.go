package kit

import (
	"image/color"
	"slices"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

type ToastKind int

const (
	ToastInfo ToastKind = iota
	ToastOK
	ToastErr
)

// Toaster shows short messages stacked in the bottom-right corner.
type Toaster struct {
	Box   *fyne.Container
	Layer fyne.CanvasObject
	BGs   map[fyne.CanvasObject]*ToastBG // the colours of the toasts on screen
}

// ToastBG is the part of a toast that takes its colours from the palette.
type ToastBG struct {
	Rect *canvas.Rectangle
	kind ToastKind
}

func (b *ToastBG) Recolor() {
	accent := ColAccent
	switch b.kind {
	case ToastOK:
		accent = ColOK
	case ToastErr:
		accent = ColErr
	}
	b.Rect.FillColor = ColBg3
	b.Rect.StrokeColor = color.NRGBA{accent.R, accent.G, accent.B, 0xaa}
	b.Rect.Refresh()
}

// Recolor repaints the toasts on screen with the current palette. Their text
// follows the theme by itself, so after a theme change (the palette is
// swapped first) a toast would otherwise keep the old background under the new
// text colour.
func (t *Toaster) Recolor() {
	for _, b := range t.BGs {
		b.Recolor()
	}
}

func NewToaster() *Toaster {
	t := &Toaster{Box: container.NewVBox(), BGs: map[fyne.CanvasObject]*ToastBG{}}
	t.Layer = container.NewBorder(nil, container.NewHBox(layout.NewSpacer(), container.NewPadded(t.Box)), nil, nil)
	return t
}

// maxToasts is how many toasts are shown at once; a new one pushes out the
// oldest.
const maxToasts = 4

// Show must be called on the UI thread. Errors stay until they are clicked
// away, so one that came up while the user looked elsewhere isn't missed;
// other toasts go by themselves.
func (t *Toaster) Show(text string, kind ToastKind) {
	bg := &ToastBG{Rect: canvas.NewRectangle(ColBg3), kind: kind}
	bg.Rect.CornerRadius = 8
	bg.Rect.StrokeWidth = 1
	bg.Recolor()
	l := widget.NewLabel(text)
	l.Wrapping = fyne.TextWrapWord
	var body fyne.CanvasObject = l
	if kind == ToastErr {
		body = container.NewBorder(nil, nil, nil, widget.NewIcon(theme.CancelIcon()), l)
	}
	var item fyne.CanvasObject
	item = FixedWidth(360, NewTappable(container.NewStack(bg.Rect, container.NewPadded(body)), func() { t.remove(item) }))
	t.BGs[item] = bg
	for len(t.Box.Objects) >= maxToasts {
		t.remove(t.Box.Objects[0])
	}
	t.Box.Add(item)
	if kind != ToastErr {
		time.AfterFunc(3500*time.Millisecond, func() { OnUI(func() { t.remove(item) }) })
	}
}

func (t *Toaster) remove(o fyne.CanvasObject) {
	delete(t.BGs, o)
	if slices.Contains(t.Box.Objects, o) {
		t.Box.Remove(o)
	}
}
