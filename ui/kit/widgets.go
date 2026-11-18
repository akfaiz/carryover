package kit

import (
	"image/color"
	"net/url"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
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

// Card is a rounded panel with an optional heading.
func Card(title string, objs ...fyne.CanvasObject) fyne.CanvasObject {
	bg := canvas.NewRectangle(ColBg2)
	bg.CornerRadius = 10
	bg.StrokeColor = ColLine
	bg.StrokeWidth = 1
	items := objs
	if title != "" {
		items = append([]fyne.CanvasObject{Heading(title)}, objs...)
	}
	return container.NewStack(bg, container.New(layout.NewCustomPaddedLayout(14, 14, 16, 16), container.NewVBox(items...)))
}

// Panel is a group inside a card (one emulator of a game, say): darker than
// the card, so what belongs together reads as one block.
func Panel(objs ...fyne.CanvasObject) fyne.CanvasObject {
	bg := canvas.NewRectangle(ColBg)
	bg.CornerRadius = 8
	bg.StrokeColor = ColLine
	bg.StrokeWidth = 1
	return container.NewStack(bg, container.New(layout.NewCustomPaddedLayout(10, 12, 12, 12), container.NewVBox(objs...)))
}

// CardHead is a card heading with right-aligned actions.
func CardHead(title string, actions ...fyne.CanvasObject) fyne.CanvasObject {
	objs := append([]fyne.CanvasObject{Heading(title), layout.NewSpacer()}, actions...)
	return container.NewHBox(objs...)
}

func Heading(t string) *widget.Label {
	l := widget.NewLabelWithStyle(t, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	l.SizeName = theme.SizeNameSubHeadingText
	return l
}

func Title(t string) *widget.Label {
	l := widget.NewLabelWithStyle(t, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	l.SizeName = theme.SizeNameHeadingText
	return l
}

// Muted is small grey wrapping text.
func Muted(t string) *widget.Label {
	l := widget.NewLabel(t)
	l.Importance = widget.LowImportance
	l.Wrapping = fyne.TextWrapWord
	l.SizeName = theme.SizeNameCaptionText
	return l
}

func Colored(t string, imp widget.Importance) *widget.Label {
	l := widget.NewLabel(t)
	l.Importance = imp
	l.Wrapping = fyne.TextWrapWord
	return l
}

func Mono(t string) *widget.Label {
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

// Field is a caption above an input. A one-line input gets InputHeight, so
// entries, drop-downs and buttons line up: a monospace entry is otherwise
// shorter than one in the regular font, and both are shorter than a button.
func Field(label string, input fyne.CanvasObject) fyne.CanvasObject {
	return container.NewVBox(fieldLabel(label), FullHeight(input))
}

// InputHeight is the height of a one-line input or button: a line of text or
// an inline icon, whichever is taller, and the inner padding above and below.
func InputHeight() float32 {
	line := fyne.MeasureText("Mg", theme.TextSize(), fyne.TextStyle{}).Height
	return max(line, theme.IconInlineSize()) + 2*theme.InnerPadding()
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

// Badge kinds.
const (
	BadgeNone = iota
	BadgeOK
	BadgeWarn
	BadgeErr
	BadgeInfo
)

func Badge(text string, kind int) fyne.CanvasObject {
	fg := ColMuted
	switch kind {
	case BadgeOK:
		fg = ColOK
	case BadgeWarn:
		fg = ColWarn
	case BadgeErr:
		fg = ColErr
	case BadgeInfo:
		fg = ColAccent
	}
	bg := canvas.NewRectangle(color.NRGBA{fg.R, fg.G, fg.B, 0x26})
	bg.CornerRadius = 9
	t := canvas.NewText(text, fg)
	t.TextSize = theme.CaptionTextSize()
	return container.NewStack(bg, container.New(layout.NewCustomPaddedLayout(2, 2, 8, 8), t))
}

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

// ProgressBar is a determinate or indeterminate progress indicator. It must
// be a widget: a struct that merely embeds *fyne.Container is laid out but
// never painted.
type ProgressBar struct {
	widget.BaseWidget
	content fyne.CanvasObject
	bar     *widget.ProgressBar
	inf     *widget.ProgressBarInfinite
	info    *widget.Label
}

func NewProgress() *ProgressBar {
	p := &ProgressBar{bar: widget.NewProgressBar(), inf: widget.NewProgressBarInfinite(), info: Muted("")}
	p.bar.TextFormatter = func() string { return "" }
	p.content = container.NewVBox(container.NewStack(p.bar, p.inf), p.info)
	p.ExtendBaseWidget(p)
	return p
}

// CreateRenderer implements fyne.Widget.
func (p *ProgressBar) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(p.content)
}

// Set updates the bar; total 0 means indeterminate.
func (p *ProgressBar) Set(msg string, bytes, total int64) {
	if total > 0 {
		p.inf.Hide()
		p.inf.Stop()
		p.bar.Show()
		p.bar.SetValue(pct(bytes, total))
		if msg != "" {
			msg += " · "
		}
		msg += FormatBytes(bytes) + " / " + FormatBytes(total)
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

// Choice maps Select labels to values.
type Choice struct {
	Labels []string
	Values []string
}

func (c *Choice) Add(value, label string) {
	c.Values = append(c.Values, value)
	c.Labels = append(c.Labels, label)
}

func (c *Choice) Label(v string) string {
	for i, x := range c.Values {
		if x == v {
			return c.Labels[i]
		}
	}
	return ""
}

func (c *Choice) Value(l string) string {
	for i, x := range c.Labels {
		if x == l {
			return c.Values[i]
		}
	}
	return ""
}

// SelectOf builds a Select for c with value v preselected.
func SelectOf(c *Choice, v string, onChange func(string)) *widget.Select {
	s := widget.NewSelect(c.Labels, nil)
	s.SetSelected(c.Label(v))
	s.OnChanged = func(l string) {
		if onChange != nil {
			onChange(c.Value(l))
		}
	}
	return s
}

// Outlined draws an input's border over o. A Select is filled with the input
// background, which the theme makes the page colour, so without it a Select
// is bare text next to a bordered Entry.
func Outlined(o fyne.CanvasObject) fyne.CanvasObject {
	border := canvas.NewRectangle(color.Transparent)
	border.StrokeColor = ColLine
	border.StrokeWidth = theme.Size(theme.SizeNameInputBorder)
	border.CornerRadius = theme.Size(theme.SizeNameInputRadius)
	return container.NewStack(o, border)
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

// PathLabel shows a file path on one line in the monospace font. When it
// doesn't fit, folders are left out of its middle
// ("C:\Users\…\save\0100152000022000"), keeping the start and as much of the
// end as fits: that is where paths differ.
type PathLabel struct {
	widget.BaseWidget
	path, prefix string
	label        *widget.Label
}

func NewPathLabel(path string) *PathLabel {
	p := &PathLabel{path: path, label: widget.NewLabelWithStyle(path, fyne.TextAlignLeading, fyne.TextStyle{Monospace: true})}
	p.label.Truncation = fyne.TextTruncateEllipsis // when not even the last folder fits
	p.ExtendBaseWidget(p)
	return p
}

// Caption makes it small and grey with prefix ("→ ") in front, for a line
// under a field.
func (p *PathLabel) Caption(prefix string) *PathLabel {
	p.prefix = prefix
	p.label.SizeName = theme.SizeNameCaptionText
	p.label.Importance = widget.LowImportance
	return p
}

// SetPath shows another path.
func (p *PathLabel) SetPath(path string) {
	p.path = path
	p.Refresh()
}

// CreateRenderer implements fyne.Widget.
func (p *PathLabel) CreateRenderer() fyne.WidgetRenderer { return &pathRenderer{p} }

type pathRenderer struct{ p *PathLabel }

func (r *pathRenderer) Layout(size fyne.Size) {
	p, l := r.p, r.p.label
	room := size.Width - 2*theme.InnerPadding()
	textSize := theme.TextSize()
	if l.SizeName != "" {
		textSize = theme.Size(l.SizeName)
	}
	text := p.prefix + ShortenPath(p.path, func(s string) bool {
		return fyne.MeasureText(p.prefix+s, textSize, l.TextStyle).Width <= room
	})
	if text != l.Text {
		l.SetText(text)
	}
	l.Resize(size)
}

func (r *pathRenderer) MinSize() fyne.Size {
	return fyne.NewSize(0, r.p.label.MinSize().Height) // shortens to any width
}

func (r *pathRenderer) Refresh() {
	r.Layout(r.p.Size()) // the path may have changed
	r.p.label.Refresh()
}
func (r *pathRenderer) Objects() []fyne.CanvasObject { return []fyne.CanvasObject{r.p.label} }
func (r *pathRenderer) Destroy()                     {}

// ShortenPath returns path, or path with folders after its first part
// replaced by "…", as few as needed for fits to accept it; the shortest form
// ("C:\…\last") when nothing fits.
func ShortenPath(path string, fits func(string) bool) string {
	if fits(path) {
		return path
	}
	sep := "/"
	if strings.Count(path, `\`) > strings.Count(path, "/") {
		sep = `\`
	}
	parts := strings.Split(path, sep)
	if len(parts) < 3 {
		return path
	}
	short := ""
	for i := 2; i < len(parts); i++ {
		short = parts[0] + sep + "…" + sep + strings.Join(parts[i:], sep)
		if fits(short) {
			break
		}
	}
	return short
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

// Tappable makes any object clickable. It is highlighted while the mouse is
// over it, so it reads as clickable like a button does. It takes the focus
// too (Tab, a gamepad's D-pad), shown as a ring, and Space or Enter press it.
type Tappable struct {
	widget.BaseWidget
	content fyne.CanvasObject
	onTap   func()
	hover   *canvas.Rectangle
	ring    *canvas.Rectangle
}

func NewTappable(content fyne.CanvasObject, onTap func()) *Tappable {
	t := &Tappable{content: content, onTap: onTap, hover: canvas.NewRectangle(color.Transparent), ring: canvas.NewRectangle(color.Transparent)}
	t.hover.CornerRadius = 8
	t.ring.CornerRadius = 8
	t.ring.StrokeWidth = 2
	t.ExtendBaseWidget(t)
	return t
}

func (t *Tappable) Tapped(*fyne.PointEvent) {
	if t.onTap != nil {
		t.onTap()
	}
}

func (t *Tappable) Cursor() desktop.Cursor { return desktop.PointerCursor }

// MouseIn implements desktop.Hoverable.
func (t *Tappable) MouseIn(*desktop.MouseEvent) {
	t.hover.FillColor = ColHover
	t.hover.Refresh()
}

// MouseMoved implements desktop.Hoverable.
func (t *Tappable) MouseMoved(*desktop.MouseEvent) {}

// MouseOut implements desktop.Hoverable.
func (t *Tappable) MouseOut() {
	t.hover.FillColor = color.Transparent
	t.hover.Refresh()
}

// FocusGained implements fyne.Focusable.
func (t *Tappable) FocusGained() {
	t.ring.StrokeColor = ColAccent
	t.ring.FillColor = ColHover
	t.ring.Refresh()
}

// FocusLost implements fyne.Focusable.
func (t *Tappable) FocusLost() {
	t.ring.StrokeColor = color.Transparent
	t.ring.FillColor = color.Transparent
	t.ring.Refresh()
}

// TypedKey implements fyne.Focusable: Space and Enter press it; the arrow keys
// and Escape move on, like a gamepad's D-pad and B, in a window with a
// Navigator.
func (t *Tappable) TypedKey(ev *fyne.KeyEvent) {
	switch ev.Name {
	case fyne.KeySpace, fyne.KeyReturn, fyne.KeyEnter:
		t.Tapped(nil)
	default:
		if n := navigatorOf(t); n != nil {
			n.HandleKey(ev)
		}
	}
}

// TypedRune implements fyne.Focusable.
func (t *Tappable) TypedRune(rune) {}

func (t *Tappable) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(container.NewStack(t.hover, t.ring, t.content))
}

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

func MustURL(s string) *url.URL {
	u, err := url.Parse(s)
	if err != nil {
		panic(err)
	}
	return u
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

// WrapTitle is a heading that wraps instead of growing the layout.
func WrapTitle(t string) *widget.Label {
	l := Title(t)
	l.Wrapping = fyne.TextWrapWord
	return l
}

// ListRow is a clickable list entry: a truncated name with grey detail text.
func ListRow(name, detail string, onTap func()) fyne.CanvasObject {
	n := widget.NewLabelWithStyle(name, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	n.Truncation = fyne.TextTruncateEllipsis
	return NewTappable(container.NewBorder(nil, nil, nil, Small(detail), n), onTap)
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

// Small is a non-wrapping grey caption, for use inside rows.
func Small(t string) *widget.Label {
	l := widget.NewLabel(t)
	l.Importance = widget.LowImportance
	l.SizeName = theme.SizeNameCaptionText
	return l
}

// BusyLine is a thin indeterminate bar with a caption, shown in the card whose
// action is running.
func BusyLine(text string) fyne.CanvasObject {
	bar := widget.NewProgressBarInfinite()
	return VBox(Small(text), bar)
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
