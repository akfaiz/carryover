package kit

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
	"github.com/stretchr/testify/assert"
)

func TestPathLabelShortensToFit(t *testing.T) {
	test.NewApp()
	p := NewPathLabel("/a/very/long/folder/structure/to/a/save").Caption("→ ")
	w := test.NewTempWindow(t, p)
	w.Resize(fyne.NewSize(120, 40))
	p.SetPath("/short")
	assert.Contains(t, p.label.Text, "/short")
	assert.Zero(t, p.MinSize().Width)
}

func TestTappableHoverAndRune(t *testing.T) {
	test.NewApp()
	tp := NewTappable(widget.NewLabel("x"), func() {})
	test.NewTempWindow(t, tp)
	tp.MouseIn(&desktop.MouseEvent{})
	tp.MouseMoved(&desktop.MouseEvent{})
	tp.MouseOut()
	tp.TypedRune('a')
}

func TestPanelAndTwoLines(t *testing.T) {
	test.NewApp()
	assert.NotNil(t, Panel(widget.NewLabel("x")))
	l := widget.NewLabel("some long text")
	assert.NotNil(t, TwoLines(l))
	assert.Equal(t, fyne.TextWrapWord, l.Wrapping)
}
