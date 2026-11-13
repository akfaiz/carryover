package kit

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// box is a rectangle with a fixed minimum size, to see where layouts put it.
func box(w, h float32) *canvas.Rectangle {
	r := canvas.NewRectangle(ColBg)
	r.SetMinSize(fyne.NewSize(w, h))
	return r
}

func TestStackSpacesObjectsByThePageGap(t *testing.T) {
	test.NewApp()
	a, b, hidden, c := box(50, 20), box(80, 30), box(10, 99), box(20, 10)
	hidden.Hide()
	s := Stack(a, b, hidden, c)
	assert.Equal(t, fyne.NewSize(80, 20+30+10+2*PageGap), s.MinSize(), "hidden objects take no space")
	s.Resize(fyne.NewSize(200, 300))
	assert.Equal(t, fyne.NewPos(0, 0), a.Position())
	assert.Equal(t, float32(20+PageGap), b.Position().Y)
	assert.Equal(t, float32(200), b.Size().Width, "full width")
	assert.Equal(t, float32(20+PageGap+30+PageGap), c.Position().Y)
	assert.Equal(t, fyne.NewSize(0, 0), Stack().MinSize())
}

func TestFixedWidthAndSizeAndMaxWidth(t *testing.T) {
	test.NewApp()
	assert.Equal(t, float32(300), FixedWidth(300, box(10, 10)).MinSize().Width)
	assert.Equal(t, float32(500), FixedWidth(300, box(500, 10)).MinSize().Width, "never smaller than the content")

	fs := FixedSize(fyne.NewSize(120, 40), box(500, 500))
	assert.Equal(t, fyne.NewSize(120, 40), fs.MinSize(), "ignores the content's size")

	inner := box(10, 10)
	m := MaxWidth(200, inner)
	m.Resize(fyne.NewSize(800, 50))
	assert.Equal(t, float32(200), inner.Size().Width, "capped")
	m.Resize(fyne.NewSize(100, 50))
	assert.Equal(t, float32(100), inner.Size().Width, "narrower windows win")
	assert.Equal(t, fyne.NewSize(10, 10), m.MinSize())
}

func TestColsGoSideBySideWhenWideAndStackWhenNarrow(t *testing.T) {
	test.NewApp()
	a, b := box(100, 40), box(100, 70)
	cols := NewCols(200, []float32{3, 1}, a, b)

	cols.Resize(fyne.NewSize(1000, 100))
	assert.Equal(t, fyne.NewPos(0, 0), a.Position())
	assert.Greater(t, b.Position().X, a.Size().Width, "side by side")
	assert.InDelta(t, 3*b.Size().Width, a.Size().Width, 1, "the 3:1 ratio")
	assert.Equal(t, float32(70), cols.MinSize().Height, "the taller column")

	cols.Resize(fyne.NewSize(300, 100))
	assert.Equal(t, float32(0), b.Position().X, "stacked")
	assert.Equal(t, 40+PageGap, b.Position().Y)
	assert.Equal(t, float32(300), b.Size().Width)
	assert.Equal(t, 40+PageGap+70, cols.MinSize().Height)

	// Columns without ratios share the width equally; an empty container is fine.
	eq := NewCols(100, nil, box(10, 10), box(10, 10))
	eq.Resize(fyne.NewSize(500, 50))
	assert.Equal(t, eq.Objects[0].Size().Width, eq.Objects[1].Size().Width)
	NewCols(100, nil).Resize(fyne.NewSize(10, 10))
}

func TestLabeledAndFillRows(t *testing.T) {
	test.NewApp()
	val := box(10, 10)
	row := LabeledRow(120, widget.NewLabel("Name"), val, widget.NewButton("x", nil))
	row.Resize(fyne.NewSize(600, 40))
	assert.Greater(t, val.Position().X, float32(119), "the value starts after the label column")
	assert.Len(t, all[*widget.Button](row), 1)
	assert.Empty(t, all[*widget.Button](LabeledRow(120, widget.NewLabel("Name"), val)), "no right-hand objects")

	main := box(10, 10)
	fill := RowFill(main, widget.NewButton("a", nil), widget.NewButton("b", nil))
	fill.Resize(fyne.NewSize(500, 40))
	assert.Greater(t, main.Size().Width, float32(300), "the main object takes the rest")
}

func TestMustURL(t *testing.T) {
	assert.Equal(t, "example.com", MustURL("https://example.com/x").Host)
	assert.Panics(t, func() { MustURL("://bad") })
}

func TestPaddedPagesAndScrollKeepTheirPosition(t *testing.T) {
	test.NewApp()
	content := box(100, 2000)
	s := ScrollPage(content)
	s.Resize(fyne.NewSize(400, 300))
	assert.Equal(t, content.MinSize().Width+56, s.Content.MinSize().Width, "28px margin on every side")

	s.Offset = fyne.NewPos(0, 123)
	ReplaceScroll(s, box(100, 3000))
	assert.Equal(t, float32(123), s.Offset.Y, "the scroll position is kept")
	assert.Equal(t, float32(3000+56), s.Content.MinSize().Height)
	require.NotNil(t, PagePadded(box(1, 1)))
}
