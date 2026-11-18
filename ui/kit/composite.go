package kit

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/widget"
)

// Composite is the base of a widget that only draws ready-made content, such
// as a form built from other widgets. A struct that merely embeds a
// *fyne.Container is laid out but never painted, so it has to be a widget:
// embed Composite and call Init once the content is built.
//
//	type planView struct{ kit.Composite }
//
//	v := &planView{}
//	v.Init(v, content)
type Composite struct {
	widget.BaseWidget
	// Content is what the widget draws. It is read once, when the widget is
	// first drawn: change what it shows by changing the objects inside it.
	Content fyne.CanvasObject
}

// Init sets the content and registers self (the struct that embeds the
// Composite) as the widget. Call it once, before the widget is used.
func (c *Composite) Init(self fyne.Widget, content fyne.CanvasObject) {
	c.Content = content
	c.ExtendBaseWidget(self)
}

// CreateRenderer implements fyne.Widget.
func (c *Composite) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(c.Content)
}
