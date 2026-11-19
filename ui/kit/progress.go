package kit

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

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

// BusyLine is a thin indeterminate bar with a caption, shown in the card whose
// action is running.
func BusyLine(text string) fyne.CanvasObject {
	bar := widget.NewProgressBarInfinite()
	return VBox(Small(text), bar)
}
