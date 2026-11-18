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
	// Focus is what takes the focus (and key presses) for the tile: the button
	// itself, or the NavButton that wraps it when the tile belongs to a Nav.
	Focus fyne.Focusable
	body  fyne.CanvasObject
}

func NewChoiceTile(name string, icon fyne.Resource, highlighted bool, tapped func()) *ChoiceTile {
	btn := widget.NewButton("", tapped)
	return newChoiceTile(name, icon, highlighted, btn, btn, btn)
}

// ChoiceTile makes a tile of the dialog that n drives: it can be reached with
// the arrow keys and the D-pad and pressed with Enter and A.
func (n *Nav) ChoiceTile(name string, icon fyne.Resource, highlighted bool, tapped func()) *ChoiceTile {
	nb := n.Button("", widget.MediumImportance, tapped)
	return newChoiceTile(name, icon, highlighted, nb, nb, &nb.Button)
}

// newChoiceTile builds a tile over layer, the button object drawn under the
// icon and name; btn is its widget.Button and focus its Focusable.
func newChoiceTile(name string, icon fyne.Resource, highlighted bool, layer fyne.CanvasObject, focus fyne.Focusable, btn *widget.Button) *ChoiceTile {
	img := canvas.NewImageFromResource(icon)
	img.FillMode = canvas.ImageFillContain
	img.SetMinSize(fyne.NewSize(40, 40))
	label := widget.NewLabelWithStyle(name, fyne.TextAlignCenter, fyne.TextStyle{Bold: highlighted})
	label.Wrapping = fyne.TextWrapWord
	layers := []fyne.CanvasObject{
		layer,
		container.NewPadded(container.NewVBox(layout.NewSpacer(), container.NewCenter(img), label, layout.NewSpacer())),
	}
	if highlighted {
		ring := canvas.NewRectangle(color.Transparent)
		ring.StrokeColor, ring.StrokeWidth = ColAccent, 2
		ring.CornerRadius = theme.InputRadiusSize()
		layers = append(layers, ring)
	}
	t := &ChoiceTile{Name: name, Highlighted: highlighted, Btn: btn, Focus: focus, body: container.NewStack(layers...)}
	t.ExtendBaseWidget(t)
	return t
}

// CreateRenderer implements fyne.Widget.
func (t *ChoiceTile) CreateRenderer() fyne.WidgetRenderer { return widget.NewSimpleRenderer(t.body) }
