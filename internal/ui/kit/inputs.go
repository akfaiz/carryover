package kit

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// MonoEntry is a single-line monospace entry. onChange may be nil.
func MonoEntry(placeholder, text string, onChange func(string)) *widget.Entry {
	e := widget.NewEntry()
	e.TextStyle.Monospace = true
	e.SetPlaceHolder(placeholder)
	e.SetText(text)
	e.OnChanged = onChange
	return e
}

// MonoMultiLine is a monospace entry showing rows lines. onChange may be nil.
func MonoMultiLine(placeholder, text string, rows int, onChange func(string)) *widget.Entry {
	e := widget.NewMultiLineEntry()
	e.TextStyle.Monospace = true
	e.SetPlaceHolder(placeholder)
	e.SetMinRowsVisible(rows)
	e.SetText(text)
	e.OnChanged = onChange
	return e
}

// CopyField shows text in a read-only (but selectable) monospace entry with a
// copy button. The button copies the text and then calls onCopy, if set (to
// tell the user); copyLabel is its label.
type CopyField struct{ Composite }

// NewCopyField creates a CopyField. clipboard receives the text to copy.
func NewCopyField(copyLabel, text string, clipboard fyne.Clipboard, onCopy func()) *CopyField {
	e := MonoEntry("", text, nil)
	e.OnChanged = func(string) { e.SetText(text) } // read-only but selectable
	btn := widget.NewButtonWithIcon(copyLabel, theme.ContentCopyIcon(), func() {
		clipboard.SetContent(text)
		if onCopy != nil {
			onCopy()
		}
	})
	f := &CopyField{}
	f.Init(f, RowFill(e, btn))
	return f
}

// AccordionSection is a one-item accordion whose open state can be read back,
// so a page can keep it open or closed across re-renders.
type AccordionSection struct {
	Composite
	item *widget.AccordionItem
}

func NewAccordionSection(title string, body fyne.CanvasObject, open bool) *AccordionSection {
	s := &AccordionSection{item: widget.NewAccordionItem(title, body)}
	s.item.Open = open
	s.Init(s, widget.NewAccordion(s.item))
	return s
}

// Open reports whether the section is currently expanded.
func (s *AccordionSection) Open() bool { return s.item.Open }
