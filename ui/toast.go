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
}

func newToaster() *toaster {
	t := &toaster{box: container.NewVBox()}
	t.layer = container.NewBorder(nil, container.NewHBox(layout.NewSpacer(), container.NewPadded(t.box)), nil, nil)
	return t
}

// show must be called on the UI thread.
func (t *toaster) show(text string, kind toastKind) {
	accent := colAccent
	ttl := 3500 * time.Millisecond
	switch kind {
	case toastOK:
		accent = colOK
	case toastErr:
		accent = colErr
		ttl = 7 * time.Second
	}
	bg := canvas.NewRectangle(colBg3)
	bg.CornerRadius = 8
	bg.StrokeColor = color.NRGBA{accent.R, accent.G, accent.B, 0xaa}
	bg.StrokeWidth = 1
	l := widget.NewLabel(text)
	l.Wrapping = fyne.TextWrapWord
	var item fyne.CanvasObject
	item = fixedWidth(360, newTappable(container.NewStack(bg, container.NewPadded(l)), func() { t.remove(item) }))
	t.box.Add(item)
	time.AfterFunc(ttl, func() { onUI(func() { t.remove(item) }) })
}

func (t *toaster) remove(o fyne.CanvasObject) {
	if slices.Contains(t.box.Objects, o) {
		t.box.Remove(o)
	}
}
