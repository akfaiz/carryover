package ui

import (
	"image/color"
	"net/url"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/lang"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/akfaiz/carryover/services"
)

// card is a rounded panel with an optional heading.
func card(title string, objs ...fyne.CanvasObject) fyne.CanvasObject {
	bg := canvas.NewRectangle(colBg2)
	bg.CornerRadius = 10
	bg.StrokeColor = colLine
	bg.StrokeWidth = 1
	items := objs
	if title != "" {
		items = append([]fyne.CanvasObject{heading(title)}, objs...)
	}
	return container.NewStack(bg, container.New(layout.NewCustomPaddedLayout(12, 12, 14, 14), container.NewVBox(items...)))
}

// cardHead is a card heading with right-aligned actions.
func cardHead(title string, actions ...fyne.CanvasObject) fyne.CanvasObject {
	objs := append([]fyne.CanvasObject{heading(title), layout.NewSpacer()}, actions...)
	return container.NewHBox(objs...)
}

func heading(t string) *widget.Label {
	l := widget.NewLabelWithStyle(t, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	l.SizeName = theme.SizeNameSubHeadingText
	return l
}

func title(t string) *widget.Label {
	l := widget.NewLabelWithStyle(t, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	l.SizeName = theme.SizeNameHeadingText
	return l
}

// muted is small grey wrapping text.
func muted(t string) *widget.Label {
	l := widget.NewLabel(t)
	l.Importance = widget.LowImportance
	l.Wrapping = fyne.TextWrapWord
	l.SizeName = theme.SizeNameCaptionText
	return l
}

func colored(t string, imp widget.Importance) *widget.Label {
	l := widget.NewLabel(t)
	l.Importance = imp
	l.Wrapping = fyne.TextWrapWord
	return l
}

func mono(t string) *widget.Label {
	l := widget.NewLabelWithStyle(t, fyne.TextAlignLeading, fyne.TextStyle{Monospace: true})
	l.Wrapping = fyne.TextWrapBreak
	return l
}

func fieldLabel(t string) *widget.Label {
	l := widget.NewLabel(t)
	l.Importance = widget.LowImportance
	l.Wrapping = fyne.TextWrapWord
	return l
}

// field is a caption above an input.
func field(label string, input fyne.CanvasObject) fyne.CanvasObject {
	return container.NewVBox(fieldLabel(label), input)
}

// badge kinds.
const (
	badgeNone = iota
	badgeOK
	badgeWarn
	badgeErr
	badgeInfo
)

func badge(text string, kind int) fyne.CanvasObject {
	fg := colMuted
	switch kind {
	case badgeOK:
		fg = colOK
	case badgeWarn:
		fg = colWarn
	case badgeErr:
		fg = colErr
	case badgeInfo:
		fg = colAccent
	}
	bg := canvas.NewRectangle(color.NRGBA{fg.R, fg.G, fg.B, 0x26})
	bg.CornerRadius = 9
	t := canvas.NewText(text, fg)
	t.TextSize = theme.CaptionTextSize()
	return container.NewStack(bg, container.New(layout.NewCustomPaddedLayout(2, 2, 8, 8), t))
}

// statusBadge is the sync state badge of a game.
func statusBadge(g services.GameView) fyne.CanvasObject {
	switch {
	case g.Status.Running:
		return badge(lang.L("running"), badgeInfo)
	case !g.Ready:
		return badge(lang.L("needs setup"), badgeWarn)
	case g.Status.Conflict:
		return badge(lang.L("conflict"), badgeErr)
	case g.Status.LastError != "":
		return badge(lang.L("sync error"), badgeErr)
	case g.Status.LastSyncAt != "":
		return badge(lang.L("synced"), badgeOK)
	}
	return badge(lang.L("not synced"), badgeNone)
}

func primary(label string, f func()) *widget.Button {
	b := widget.NewButton(label, f)
	b.Importance = widget.HighImportance
	return b
}

func lowButton(label string, f func()) *widget.Button {
	b := widget.NewButton(label, f)
	b.Importance = widget.LowImportance
	return b
}

func dangerButton(label string, f func()) *widget.Button {
	b := widget.NewButton(label, f)
	b.Importance = widget.DangerImportance
	return b
}

func setEnabled(w fyne.Disableable, on bool) {
	if on {
		w.Enable()
	} else {
		w.Disable()
	}
}

func hbox(objs ...fyne.CanvasObject) *fyne.Container { return container.NewHBox(objs...) }

func vbox(objs ...fyne.CanvasObject) *fyne.Container { return container.NewVBox(objs...) }

// rowFill puts main in the middle (stretched) and fixed objects at its right.
func rowFill(main fyne.CanvasObject, right ...fyne.CanvasObject) fyne.CanvasObject {
	return container.NewBorder(nil, nil, nil, hbox(right...), main)
}

// progressBar is a determinate or indeterminate progress indicator. It must
// be a widget: a struct that merely embeds *fyne.Container is laid out but
// never painted.
type progressBar struct {
	widget.BaseWidget
	content fyne.CanvasObject
	bar     *widget.ProgressBar
	inf     *widget.ProgressBarInfinite
	info    *widget.Label
}

func newProgress() *progressBar {
	p := &progressBar{bar: widget.NewProgressBar(), inf: widget.NewProgressBarInfinite(), info: muted("")}
	p.bar.TextFormatter = func() string { return "" }
	p.content = container.NewVBox(container.NewStack(p.bar, p.inf), p.info)
	p.ExtendBaseWidget(p)
	return p
}

// CreateRenderer implements fyne.Widget.
func (p *progressBar) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(p.content)
}

// set updates the bar; total 0 means indeterminate.
func (p *progressBar) set(msg string, bytes, total int64) {
	if total > 0 {
		p.inf.Hide()
		p.inf.Stop()
		p.bar.Show()
		p.bar.SetValue(pct(bytes, total))
		if msg != "" {
			msg += " · "
		}
		msg += formatBytes(bytes) + " / " + formatBytes(total)
	} else {
		p.bar.Hide()
		p.inf.Show()
		p.inf.Start()
	}
	p.info.SetText(msg)
	if msg == "" {
		p.info.Hide()
	} else {
		p.info.Show()
	}
}

// choice maps Select labels to values.
type choice struct {
	labels []string
	values []string
}

func (c *choice) add(value, label string) {
	c.values = append(c.values, value)
	c.labels = append(c.labels, label)
}

func (c *choice) label(v string) string {
	for i, x := range c.values {
		if x == v {
			return c.labels[i]
		}
	}
	return ""
}

func (c *choice) value(l string) string {
	for i, x := range c.labels {
		if x == l {
			return c.values[i]
		}
	}
	return ""
}

// selectOf builds a Select for c with value v preselected.
func selectOf(c *choice, v string, onChange func(string)) *widget.Select {
	s := widget.NewSelect(c.labels, nil)
	s.SetSelected(c.label(v))
	s.OnChanged = func(l string) { onChange(c.value(l)) }
	return s
}

// fixedWidth constrains an object's minimum width.
func fixedWidth(w float32, o fyne.CanvasObject) fyne.CanvasObject {
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

// fixedSize forces exactly size (ignores children's min size).
func fixedSize(size fyne.Size, o fyne.CanvasObject) fyne.CanvasObject {
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

// tappable makes any object clickable.
type tappable struct {
	widget.BaseWidget
	content fyne.CanvasObject
	onTap   func()
}

func newTappable(content fyne.CanvasObject, onTap func()) *tappable {
	t := &tappable{content: content, onTap: onTap}
	t.ExtendBaseWidget(t)
	return t
}

func (t *tappable) Tapped(*fyne.PointEvent) {
	if t.onTap != nil {
		t.onTap()
	}
}

func (t *tappable) Cursor() desktop.Cursor { return desktop.PointerCursor }

func (t *tappable) CreateRenderer() fyne.WidgetRenderer { return widget.NewSimpleRenderer(t.content) }

// columns lays out objects side by side when wide enough, stacked otherwise.
type responsiveCols struct {
	minCol float32
	ratios []float32
	width  float32 // last laid-out width
}

func newCols(minCol float32, ratios []float32, objs ...fyne.CanvasObject) *fyne.Container {
	return container.New(&responsiveCols{minCol: minCol, ratios: ratios}, objs...)
}

func (r *responsiveCols) sideBySide(width float32, n int) bool {
	return width >= r.minCol*float32(n)
}

func (r *responsiveCols) Layout(objs []fyne.CanvasObject, size fyne.Size) {
	pad := theme.Padding() * 3
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
	pad := theme.Padding() * 3
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

func mustURL(s string) *url.URL {
	u, err := url.Parse(s)
	if err != nil {
		panic(err)
	}
	return u
}

// maxWidth left-aligns o and caps its width.
func maxWidth(w float32, o fyne.CanvasObject) fyne.CanvasObject {
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

// wrapTitle is a heading that wraps instead of growing the layout.
func wrapTitle(t string) *widget.Label {
	l := title(t)
	l.Wrapping = fyne.TextWrapWord
	return l
}

// listRow is a clickable list entry: a truncated name with grey detail text.
func listRow(name, detail string, onTap func()) fyne.CanvasObject {
	n := widget.NewLabelWithStyle(name, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	n.Truncation = fyne.TextTruncateEllipsis
	return newTappable(container.NewBorder(nil, nil, nil, small(detail), n), onTap)
}

// labeledRow is a fixed-width label, a stretching value and optional objects
// at the right.
func labeledRow(labelWidth float32, label, value fyne.CanvasObject, right ...fyne.CanvasObject) fyne.CanvasObject {
	var r fyne.CanvasObject
	if len(right) > 0 {
		r = hbox(right...)
	}
	return container.NewBorder(nil, nil, fixedWidth(labelWidth, label), r, value)
}

// mediaRow is a clickable list entry with a leading image, a truncated name
// and an optional trailing object (such as a badge).
func mediaRow(lead fyne.CanvasObject, name string, trailing fyne.CanvasObject, onTap func()) fyne.CanvasObject {
	n := widget.NewLabel(name)
	n.Truncation = fyne.TextTruncateEllipsis
	if trailing == nil {
		trailing = widget.NewLabel("")
	}
	return newTappable(container.NewPadded(container.NewBorder(nil, nil, lead, trailing, n)), onTap)
}

// small is a non-wrapping grey caption, for use inside rows.
func small(t string) *widget.Label {
	l := widget.NewLabel(t)
	l.Importance = widget.LowImportance
	l.SizeName = theme.SizeNameCaptionText
	return l
}
