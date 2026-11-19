package kit

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
)

// Card is a rounded panel with an optional heading.
func Card(title string, objs ...fyne.CanvasObject) fyne.CanvasObject {
	bg := canvas.NewRectangle(ColBg2)
	bg.CornerRadius = 10
	bg.StrokeColor = ColLine
	bg.StrokeWidth = 1
	items := objs
	if title != "" {
		items = append([]fyne.CanvasObject{Heading(title)}, objs...)
	}
	return container.NewStack(bg, container.New(layout.NewCustomPaddedLayout(14, 14, 16, 16), container.NewVBox(items...)))
}

// Panel is a group inside a card (one emulator of a game, say): darker than
// the card, so what belongs together reads as one block.
func Panel(objs ...fyne.CanvasObject) fyne.CanvasObject {
	bg := canvas.NewRectangle(ColBg)
	bg.CornerRadius = 8
	bg.StrokeColor = ColLine
	bg.StrokeWidth = 1
	return container.NewStack(bg, container.New(layout.NewCustomPaddedLayout(10, 12, 12, 12), container.NewVBox(objs...)))
}

// CardHead is a card heading with right-aligned actions.
func CardHead(title string, actions ...fyne.CanvasObject) fyne.CanvasObject {
	objs := append([]fyne.CanvasObject{Heading(title), layout.NewSpacer()}, actions...)
	return container.NewHBox(objs...)
}
