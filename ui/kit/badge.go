package kit

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
)

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
