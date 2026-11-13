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
type CopyField struct {
	widget.BaseWidget
	content fyne.CanvasObject
}

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
	f := &CopyField{content: RowFill(e, btn)}
	f.ExtendBaseWidget(f)
	return f
}

// CreateRenderer implements fyne.Widget.
func (f *CopyField) CreateRenderer() fyne.WidgetRenderer { return widget.NewSimpleRenderer(f.content) }

// AccordionSection is a one-item accordion whose open state can be read back,
// so a page can keep it open or closed across re-renders.
type AccordionSection struct {
	widget.BaseWidget
	item    *widget.AccordionItem
	content fyne.CanvasObject
}

func NewAccordionSection(title string, body fyne.CanvasObject, open bool) *AccordionSection {
	s := &AccordionSection{item: widget.NewAccordionItem(title, body)}
	s.item.Open = open
	s.content = widget.NewAccordion(s.item)
	s.ExtendBaseWidget(s)
	return s
}

// Open reports whether the section is currently expanded.
func (s *AccordionSection) Open() bool { return s.item.Open }

// CreateRenderer implements fyne.Widget.
func (s *AccordionSection) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(s.content)
}
