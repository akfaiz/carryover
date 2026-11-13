package kit

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// ChoiceTileSize is the side of the square tile of one way to play, and
// ChoicesPerRow how many fit in a row of the dialog. The rows are laid out by
// hand: a wrapping grid works out its height from the width it had when it was
// first laid out, which leaves a tall gap under the tiles in a dialog.
const (
	ChoiceTileSize = 124
	ChoicesPerRow  = 4
)

// ChoiceTile is a square button for one way to play a game: an icon over its
// name, with a ring around the highlighted one. A real button sits underneath,
// so the tile takes focus and taps like any other button; the icon and the name
// are drawn over it and let taps through.
type ChoiceTile struct {
	widget.BaseWidget
	Name        string
	Highlighted bool
	Btn         *widget.Button
	body        fyne.CanvasObject
}

func NewChoiceTile(name string, icon fyne.Resource, highlighted bool, tapped func()) *ChoiceTile {
	btn := widget.NewButton("", tapped)
	img := canvas.NewImageFromResource(icon)
	img.FillMode = canvas.ImageFillContain
	img.SetMinSize(fyne.NewSize(40, 40))
	label := widget.NewLabelWithStyle(name, fyne.TextAlignCenter, fyne.TextStyle{Bold: highlighted})
	label.Wrapping = fyne.TextWrapWord
	layers := []fyne.CanvasObject{
		btn,
		container.NewPadded(container.NewVBox(layout.NewSpacer(), container.NewCenter(img), label, layout.NewSpacer())),
	}
	if highlighted {
		ring := canvas.NewRectangle(color.Transparent)
		ring.StrokeColor, ring.StrokeWidth = ColAccent, 2
		ring.CornerRadius = theme.InputRadiusSize()
		layers = append(layers, ring)
	}
	t := &ChoiceTile{Name: name, Highlighted: highlighted, Btn: btn, body: container.NewStack(layers...)}
	t.ExtendBaseWidget(t)
	return t
}

// CreateRenderer implements fyne.Widget.
func (t *ChoiceTile) CreateRenderer() fyne.WidgetRenderer { return widget.NewSimpleRenderer(t.body) }
