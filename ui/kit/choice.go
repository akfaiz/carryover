package kit

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// Choice maps Select labels to values.
type Choice struct {
	Labels []string
	Values []string
}

func (c *Choice) Add(value, label string) {
	c.Values = append(c.Values, value)
	c.Labels = append(c.Labels, label)
}

func (c *Choice) Label(v string) string {
	for i, x := range c.Values {
		if x == v {
			return c.Labels[i]
		}
	}
	return ""
}

func (c *Choice) Value(l string) string {
	for i, x := range c.Labels {
		if x == l {
			return c.Values[i]
		}
	}
	return ""
}

// SelectOf builds a Select for c with value v preselected.
func SelectOf(c *Choice, v string, onChange func(string)) *widget.Select {
	s := widget.NewSelect(c.Labels, nil)
	s.SetSelected(c.Label(v))
	s.OnChanged = func(l string) {
		if onChange != nil {
			onChange(c.Value(l))
		}
	}
	return s
}

// Outlined draws an input's border over o. A Select is filled with the input
// background, which the theme makes the page colour, so without it a Select
// is bare text next to a bordered Entry.
func Outlined(o fyne.CanvasObject) fyne.CanvasObject {
	border := canvas.NewRectangle(color.Transparent)
	border.StrokeColor = ColLine
	border.StrokeWidth = theme.Size(theme.SizeNameInputBorder)
	border.CornerRadius = theme.Size(theme.SizeNameInputRadius)
	return container.NewStack(o, border)
}
