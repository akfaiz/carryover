package kit

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

// PageGap is the space between cards and page sections stacked on a page.
// Fyne's own VBox leaves only theme.Padding() (4px), which looks cramped next
// to the page margin.
const PageGap float32 = 16

// gapVBox stacks objects vertically at full width with gap between them.
type gapVBox struct{ gap float32 }

func (g gapVBox) Layout(objs []fyne.CanvasObject, size fyne.Size) {
	y := float32(0)
	for _, o := range objs {
		if !o.Visible() {
			continue
		}
		h := o.MinSize().Height
		o.Move(fyne.NewPos(0, y))
		o.Resize(fyne.NewSize(size.Width, h))
		y += h + g.gap
	}
}

func (g gapVBox) MinSize(objs []fyne.CanvasObject) fyne.Size {
	var w, h float32
	for _, o := range objs {
		if !o.Visible() {
			continue
		}
		m := o.MinSize()
		w = max(w, m.Width)
		h += m.Height + g.gap
	}
	return fyne.NewSize(w, max(h-g.gap, 0))
}

// Stack is a vertical stack of cards or page sections, pageGap apart.
func Stack(objs ...fyne.CanvasObject) *fyne.Container {
	return container.New(gapVBox{PageGap}, objs...)
}

// FullHeight makes o at least InputHeight tall (an input on its own in a
// vertical box, which gives it no more than its own height).
func FullHeight(o fyne.CanvasObject) fyne.CanvasObject {
	return container.New(inputHeightLayout{}, o)
}

type inputHeightLayout struct{}

func (inputHeightLayout) Layout(objs []fyne.CanvasObject, size fyne.Size) {
	for _, o := range objs {
		o.Move(fyne.NewPos(0, 0))
		o.Resize(size)
	}
}

func (inputHeightLayout) MinSize(objs []fyne.CanvasObject) fyne.Size {
	var s fyne.Size
	for _, o := range objs {
		if o.Visible() {
			s = s.Max(o.MinSize().Max(fyne.NewSize(0, InputHeight())))
		}
	}
	return s
}

func HBox(objs ...fyne.CanvasObject) *fyne.Container { return container.NewHBox(objs...) }

func VBox(objs ...fyne.CanvasObject) *fyne.Container { return container.NewVBox(objs...) }

// RowFill puts main in the middle (stretched) and fixed objects at its right.
func RowFill(main fyne.CanvasObject, right ...fyne.CanvasObject) fyne.CanvasObject {
	if len(right) == 0 {
		// No empty box on the right: the border layout would still leave a
		// gap for it, making main narrower than the inputs under it.
		return FullHeight(main)
	}
	return container.NewBorder(nil, nil, nil, HBox(right...), main)
}

// TwoLines lets l wrap onto a second line, ending in "…" past that, and
// always takes the height of two lines so whatever follows lines up.
func TwoLines(l *widget.Label) fyne.CanvasObject {
	probe := widget.NewLabelWithStyle("A\nA", l.Alignment, l.TextStyle)
	probe.SizeName = l.SizeName
	l.Wrapping = fyne.TextWrapWord
	l.Truncation = fyne.TextTruncateEllipsis
	return container.New(&minSizeLayout{size: fyne.NewSize(0, probe.MinSize().Height)}, l)
}

// GridWrap lays out cells of size in rows that wrap, gap apart. Fyne's own
// GridWrap leaves only theme.Padding() (4px), too tight for cover art.
func GridWrap(size fyne.Size, gap float32, objs ...fyne.CanvasObject) *fyne.Container {
	return container.New(&gridWrapGap{cell: size, gap: gap, rows: 1}, objs...)
}

type gridWrapGap struct {
	cell fyne.Size
	gap  float32
	rows int // from the last layout, for MinSize
}

func (g *gridWrapGap) Layout(objs []fyne.CanvasObject, size fyne.Size) {
	cols := max(1, int((size.Width+g.gap)/(g.cell.Width+g.gap)))
	g.rows = 0
	i := 0
	for _, o := range objs {
		if !o.Visible() {
			continue
		}
		col, row := i%cols, i/cols
		if col == 0 {
			g.rows++
		}
		o.Move(fyne.NewPos(float32(col)*(g.cell.Width+g.gap), float32(row)*(g.cell.Height+g.gap)))
		o.Resize(g.cell)
		i++
	}
}

func (g *gridWrapGap) MinSize([]fyne.CanvasObject) fyne.Size {
	rows := float32(max(g.rows, 1))
	return fyne.NewSize(g.cell.Width, rows*g.cell.Height+(rows-1)*g.gap)
}

// FixedWidth constrains an object's minimum width.
func FixedWidth(w float32, o fyne.CanvasObject) fyne.CanvasObject {
	return container.New(&minSizeLayout{size: fyne.NewSize(w, 0)}, o)
}

// minSizeLayout stretches its children and enforces a minimum size.
type minSizeLayout struct{ size fyne.Size }

func (l *minSizeLayout) Layout(objs []fyne.CanvasObject, size fyne.Size) {
	for _, o := range objs {
		o.Move(fyne.NewPos(0, 0))
		o.Resize(size)
	}
}

func (l *minSizeLayout) MinSize(objs []fyne.CanvasObject) fyne.Size {
	s := l.size
	for _, o := range objs {
		s = s.Max(o.MinSize())
	}
	return s
}

// FixedSize forces exactly size (ignores children's min size).
func FixedSize(size fyne.Size, o fyne.CanvasObject) fyne.CanvasObject {
	return container.New(&exactLayout{size: size}, o)
}

type exactLayout struct{ size fyne.Size }

func (l *exactLayout) Layout(objs []fyne.CanvasObject, size fyne.Size) {
	for _, o := range objs {
		o.Move(fyne.NewPos(0, 0))
		o.Resize(size)
	}
}

func (l *exactLayout) MinSize([]fyne.CanvasObject) fyne.Size { return l.size }

// columns lays out objects side by side when wide enough, stacked otherwise.
type responsiveCols struct {
	minCol float32
	ratios []float32
	width  float32 // last laid-out width
}

func NewCols(minCol float32, ratios []float32, objs ...fyne.CanvasObject) *fyne.Container {
	return container.New(&responsiveCols{minCol: minCol, ratios: ratios}, objs...)
}

func (r *responsiveCols) sideBySide(width float32, n int) bool {
	return width >= r.minCol*float32(n)
}

func (r *responsiveCols) Layout(objs []fyne.CanvasObject, size fyne.Size) {
	pad := PageGap
	n := len(objs)
	if n == 0 {
		return
	}
	r.width = size.Width
	if !r.sideBySide(size.Width, n) {
		y := float32(0)
		for _, o := range objs {
			h := o.MinSize().Height
			o.Move(fyne.NewPos(0, y))
			o.Resize(fyne.NewSize(size.Width, h))
			y += h + pad
		}
		return
	}
	var total float32
	for i := range objs {
		total += r.ratio(i)
	}
	avail := size.Width - pad*float32(n-1)
	x := float32(0)
	for i, o := range objs {
		w := avail * r.ratio(i) / total
		o.Move(fyne.NewPos(x, 0))
		o.Resize(fyne.NewSize(w, o.MinSize().Height))
		x += w + pad
	}
}

func (r *responsiveCols) ratio(i int) float32 {
	if i < len(r.ratios) {
		return r.ratios[i]
	}
	return 1
}

func (r *responsiveCols) MinSize(objs []fyne.CanvasObject) fyne.Size {
	var w, h float32
	pad := PageGap
	if r.width > 0 && r.sideBySide(r.width, len(objs)) {
		for _, o := range objs {
			m := o.MinSize()
			w = max(w, m.Width)
			h = max(h, m.Height)
		}
		return fyne.NewSize(w, h)
	}
	for i, o := range objs {
		m := o.MinSize()
		w = max(w, m.Width)
		h += m.Height
		if i > 0 {
			h += pad
		}
	}
	return fyne.NewSize(w, h)
}

// MaxWidth left-aligns o and caps its width.
func MaxWidth(w float32, o fyne.CanvasObject) fyne.CanvasObject {
	return container.New(&maxWidthLayout{w: w}, o)
}

type maxWidthLayout struct{ w float32 }

func (l *maxWidthLayout) Layout(objs []fyne.CanvasObject, size fyne.Size) {
	for _, o := range objs {
		o.Move(fyne.NewPos(0, 0))
		o.Resize(fyne.NewSize(min(size.Width, l.w), size.Height))
	}
}

func (l *maxWidthLayout) MinSize(objs []fyne.CanvasObject) fyne.Size {
	var s fyne.Size
	for _, o := range objs {
		s = s.Max(o.MinSize())
	}
	return s
}
