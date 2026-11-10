package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/lang"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// monoEntry is a single-line monospace entry. onChange may be nil.
func monoEntry(placeholder, text string, onChange func(string)) *widget.Entry {
	e := widget.NewEntry()
	e.TextStyle.Monospace = true
	e.SetPlaceHolder(placeholder)
	e.SetText(text)
	e.OnChanged = onChange
	return e
}

// monoMultiLine is a monospace entry showing rows lines. onChange may be nil.
func monoMultiLine(placeholder, text string, rows int, onChange func(string)) *widget.Entry {
	e := widget.NewMultiLineEntry()
	e.TextStyle.Monospace = true
	e.SetPlaceHolder(placeholder)
	e.SetMinRowsVisible(rows)
	e.SetText(text)
	e.OnChanged = onChange
	return e
}

// copyField shows text in a read-only (but selectable) monospace entry with a
// Copy button.
type copyField struct {
	widget.BaseWidget
	content fyne.CanvasObject
}

func newCopyField(u *UI, text string) *copyField {
	e := monoEntry("", text, nil)
	e.OnChanged = func(string) { e.SetText(text) } // read-only but selectable
	btn := widget.NewButtonWithIcon(lang.L("Copy"), theme.ContentCopyIcon(), func() {
		u.app.Clipboard().SetContent(text)
		u.toast(lang.L("Copied"), toastOK)
	})
	f := &copyField{content: rowFill(e, btn)}
	f.ExtendBaseWidget(f)
	return f
}

// CreateRenderer implements fyne.Widget.
func (f *copyField) CreateRenderer() fyne.WidgetRenderer { return widget.NewSimpleRenderer(f.content) }

// accordionSection is a one-item accordion whose open state can be read back,
// so a page can keep it open or closed across re-renders.
type accordionSection struct {
	widget.BaseWidget
	item    *widget.AccordionItem
	content fyne.CanvasObject
}

func newAccordionSection(title string, body fyne.CanvasObject, open bool) *accordionSection {
	s := &accordionSection{item: widget.NewAccordionItem(title, body)}
	s.item.Open = open
	s.content = widget.NewAccordion(s.item)
	s.ExtendBaseWidget(s)
	return s
}

// Open reports whether the section is currently expanded.
func (s *accordionSection) Open() bool { return s.item.Open }

// CreateRenderer implements fyne.Widget.
func (s *accordionSection) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(s.content)
}
