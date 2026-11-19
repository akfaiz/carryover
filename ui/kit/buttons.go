package kit

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

func Primary(label string, f func()) *widget.Button {
	b := widget.NewButton(label, f)
	b.Importance = widget.HighImportance
	return b
}

func LowButton(label string, f func()) *widget.Button {
	b := widget.NewButton(label, f)
	b.Importance = widget.LowImportance
	return b
}

func DangerButton(label string, f func()) *widget.Button {
	b := widget.NewButton(label, f)
	b.Importance = widget.DangerImportance
	return b
}

func SetEnabled(w fyne.Disableable, on bool) {
	if on {
		w.Enable()
	} else {
		w.Disable()
	}
}

// NewCheck is a checkbox that calls changed when the user toggles it. Its
// initial state is set before the callback is attached: SetChecked calls the
// callback, so a callback that redraws the page would build the checkbox again,
// without end, and one that saves would save before the form is complete.
func NewCheck(label string, on bool, changed func(bool)) *widget.Check {
	c := widget.NewCheck(label, nil)
	c.SetChecked(on)
	c.OnChanged = changed
	return c
}

// SetVisible shows or hides an object.
func SetVisible(o fyne.CanvasObject, on bool) {
	if on {
		o.Show()
	} else {
		o.Hide()
	}
}

// Tonal draws the HighImportance (and DangerImportance) buttons in o tinted
// with the accent (error) colour instead of filled with it: still clearly the
// action, but quieter where many sit side by side (one per game) or next to
// ordinary buttons.
func Tonal(o fyne.CanvasObject) fyne.CanvasObject {
	return container.NewThemeOverride(o, tonalTheme{})
}

type tonalTheme struct{ AppTheme }

func (t tonalTheme) Color(n fyne.ThemeColorName, v fyne.ThemeVariant) color.Color {
	tint := func(c color.NRGBA, a uint8) color.Color { return color.NRGBA{c.R, c.G, c.B, a} }
	switch n {
	case theme.ColorNamePrimary:
		return tint(ColAccent, 0x30)
	case theme.ColorNameError:
		return tint(ColErr, 0x26)
	case theme.ColorNameFocus:
		// Blended over the tint: a stronger tint, not the solid accent, which
		// would hide the accent-coloured label.
		return tint(ColAccent, 0x40)
	case theme.ColorNameForegroundOnPrimary:
		return ColAccent
	case theme.ColorNameForegroundOnError:
		return ColErr
	}
	return t.AppTheme.Color(n, v)
}
