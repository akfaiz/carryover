package kit

import (
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

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

func (r *pathRenderer) Destroy() {}

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
