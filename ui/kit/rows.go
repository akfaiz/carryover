package kit

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// ListRow is a clickable list entry: a truncated name with grey detail text.
func ListRow(name, detail string, onTap func()) fyne.CanvasObject {
	n := widget.NewLabelWithStyle(name, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	n.Truncation = fyne.TextTruncateEllipsis
	return NewTappable(container.NewBorder(nil, nil, nil, Small(detail), n), onTap)
}

// IconRow is a clickable line with a leading icon and a text that ends in "…"
// when it doesn't fit. For narrow places such as a sidebar, where a button
// would widen to its whole text.
func IconRow(icon fyne.Resource, text string, onTap func()) *Tappable {
	l := widget.NewLabel(text)
	l.Truncation = fyne.TextTruncateEllipsis
	// Inset like a button's icon.
	lead := container.New(layout.NewCustomPaddedLayout(0, 0, theme.InnerPadding(), 0), widget.NewIcon(icon))
	return NewTappable(container.NewBorder(nil, nil, lead, nil, l), onTap)
}

// DetailRow is a clickable list entry for a long detail such as a folder path:
// the name on the first line, the detail wrapped below it in a small monospace
// font, and an optional badge last. A plain vertical stack: wrapped labels in
// nested Border layouts reserve the wrong height.
func DetailRow(name, detail string, badgeObj fyne.CanvasObject, onTap func()) fyne.CanvasObject {
	n := widget.NewLabelWithStyle(name, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	n.Wrapping = fyne.TextWrapWord
	d := widget.NewLabelWithStyle(detail, fyne.TextAlignLeading, fyne.TextStyle{Monospace: true})
	d.Wrapping = fyne.TextWrapBreak
	d.Importance = widget.LowImportance
	d.SizeName = theme.SizeNameCaptionText
	rows := container.NewVBox(n, d)
	if badgeObj != nil {
		rows.Add(container.NewHBox(badgeObj))
	}
	return NewTappable(rows, onTap)
}

// LabeledRow is a fixed-width label, a stretching value and optional objects
// at the right.
func LabeledRow(labelWidth float32, label, value fyne.CanvasObject, right ...fyne.CanvasObject) fyne.CanvasObject {
	var r fyne.CanvasObject
	if len(right) > 0 {
		r = HBox(right...)
	}
	return container.NewBorder(nil, nil, FixedWidth(labelWidth, label), r, value)
}

// MediaRow is a clickable list entry with a leading image, a truncated name
// and an optional trailing object (such as a badge).
func MediaRow(lead fyne.CanvasObject, name string, trailing fyne.CanvasObject, onTap func()) fyne.CanvasObject {
	n := widget.NewLabel(name)
	n.Truncation = fyne.TextTruncateEllipsis
	if trailing == nil {
		trailing = widget.NewLabel("")
	}
	return NewTappable(container.NewPadded(container.NewBorder(nil, nil, lead, trailing, n)), onTap)
}

// Chips is a row of toggle buttons of which one is selected, such as the
// filters above the library grid. The others are outlined, so they read as
// choices rather than as plain text.
func Chips(options []ChipOption, selected string, onSelect func(string)) *fyne.Container {
	row := HBox()
	for _, o := range options {
		b := widget.NewButtonWithIcon(o.Label, o.Icon, func() { onSelect(o.Value) })
		b.Importance = widget.LowImportance
		if o.Value == selected {
			// Tinted, not filled: the page's main action keeps the solid colour.
			b.Importance = widget.HighImportance
			row.Add(Tonal(b))
			continue
		}
		row.Add(Outlined(b))
	}
	return row
}

// ChipOption is one button of chips; icon may be nil.
type ChipOption struct {
	Value, Label string
	Icon         fyne.Resource
}
