package ui

import (
	"image/color"
	"slices"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
)

type toastKind int

const (
	toastInfo toastKind = iota
	toastOK
	toastErr
)

// toaster shows short messages stacked in the bottom-right corner.
type toaster struct {
	box   *fyne.Container
	layer fyne.CanvasObject
	bgs   map[fyne.CanvasObject]*toastBG // the colours of the toasts on screen
}

// toastBG is the part of a toast that takes its colours from the palette.
type toastBG struct {
	rect *canvas.Rectangle
	kind toastKind
}

func (b *toastBG) recolor() {
	accent := colAccent
	switch b.kind {
	case toastOK:
		accent = colOK
	case toastErr:
		accent = colErr
	}
	b.rect.FillColor = colBg3
	b.rect.StrokeColor = color.NRGBA{accent.R, accent.G, accent.B, 0xaa}
	b.rect.Refresh()
}

// recolor repaints the toasts on screen with the current palette. Their text
// follows the theme by itself, so after a theme change (the palette is
// swapped first) a toast would otherwise keep the old background under the new
// text colour.
func (t *toaster) recolor() {
	for _, b := range t.bgs {
		b.recolor()
	}
}

func newToaster() *toaster {
	t := &toaster{box: container.NewVBox(), bgs: map[fyne.CanvasObject]*toastBG{}}
	t.layer = container.NewBorder(nil, container.NewHBox(layout.NewSpacer(), container.NewPadded(t.box)), nil, nil)
	return t
}

// show must be called on the UI thread.
func (t *toaster) show(text string, kind toastKind) {
	ttl := 3500 * time.Millisecond
	if kind == toastErr {
		ttl = 7 * time.Second
	}
	bg := &toastBG{rect: canvas.NewRectangle(colBg3), kind: kind}
	bg.rect.CornerRadius = 8
	bg.rect.StrokeWidth = 1
	bg.recolor()
	l := widget.NewLabel(text)
	l.Wrapping = fyne.TextWrapWord
	var item fyne.CanvasObject
	item = fixedWidth(360, newTappable(container.NewStack(bg.rect, container.NewPadded(l)), func() { t.remove(item) }))
	t.bgs[item] = bg
	t.box.Add(item)
	time.AfterFunc(ttl, func() { onUI(func() { t.remove(item) }) })
}

func (t *toaster) remove(o fyne.CanvasObject) {
	delete(t.bgs, o)
	if slices.Contains(t.box.Objects, o) {
		t.box.Remove(o)
	}
}
