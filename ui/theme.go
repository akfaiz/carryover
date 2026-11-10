package ui

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
)

// Palette (kept from the previous web UI).
var (
	colBg     = color.NRGBA{0x0e, 0x0f, 0x14, 0xff}
	colBg2    = color.NRGBA{0x15, 0x17, 0x1f, 0xff}
	colBg3    = color.NRGBA{0x1d, 0x20, 0x29, 0xff}
	colLine   = color.NRGBA{0x2a, 0x2e, 0x3a, 0xff}
	colText   = color.NRGBA{0xe7, 0xe9, 0xee, 0xff}
	colMuted  = color.NRGBA{0x8b, 0x90, 0xa0, 0xff}
	colAccent = color.NRGBA{0x5b, 0x8c, 0xff, 0xff}
	colOK     = color.NRGBA{0x3c, 0xcf, 0x91, 0xff}
	colWarn   = color.NRGBA{0xf5, 0xb8, 0x4b, 0xff}
	colErr    = color.NRGBA{0xff, 0x6b, 0x6b, 0xff}
)

// darkTheme is PlayRelay's theme: Fyne's dark theme with the app palette.
type darkTheme struct{}

var _ fyne.Theme = darkTheme{}

func (darkTheme) Color(n fyne.ThemeColorName, _ fyne.ThemeVariant) color.Color {
	switch n {
	case theme.ColorNameBackground:
		return colBg
	case theme.ColorNameButton:
		return colBg3
	case theme.ColorNameDisabledButton:
		return colBg2
	case theme.ColorNameInputBackground:
		return colBg
	case theme.ColorNameInputBorder, theme.ColorNameSeparator:
		return colLine
	case theme.ColorNameMenuBackground, theme.ColorNameOverlayBackground, theme.ColorNameHeaderBackground:
		return colBg2
	case theme.ColorNameForeground:
		return colText
	case theme.ColorNamePlaceHolder:
		return colMuted
	case theme.ColorNamePrimary, theme.ColorNameHyperlink, theme.ColorNameFocus:
		return colAccent
	case theme.ColorNameSuccess:
		return colOK
	case theme.ColorNameWarning:
		return colWarn
	case theme.ColorNameError:
		return colErr
	case theme.ColorNameSelection:
		return color.NRGBA{0x5b, 0x8c, 0xff, 0x55}
	case theme.ColorNameHover:
		return color.NRGBA{0xff, 0xff, 0xff, 0x12}
	}
	return theme.DefaultTheme().Color(n, theme.VariantDark)
}

func (darkTheme) Font(s fyne.TextStyle) fyne.Resource { return theme.DefaultTheme().Font(s) }

func (darkTheme) Icon(n fyne.ThemeIconName) fyne.Resource { return theme.DefaultTheme().Icon(n) }

func (darkTheme) Size(n fyne.ThemeSizeName) float32 {
	switch n {
	case theme.SizeNameInputRadius, theme.SizeNameSelectionRadius:
		return 6
	}
	return theme.DefaultTheme().Size(n)
}
