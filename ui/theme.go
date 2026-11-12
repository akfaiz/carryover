package ui

import (
	"image/color"
	"sync/atomic"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
)

// Palette (kept from the previous web UI). The variables hold the active
// palette; setPalette swaps them, so widgets built afterwards pick it up.
var (
	colBg     color.NRGBA
	colBg2    color.NRGBA
	colBg3    color.NRGBA
	colLine   color.NRGBA
	colText   color.NRGBA
	colMuted  color.NRGBA
	colAccent color.NRGBA
	colOK     color.NRGBA
	colWarn   color.NRGBA
	colErr    color.NRGBA

	colHover color.NRGBA
	lightOn  bool
)

// Theme names stored in settings; themeSystem follows the system's light/dark
// preference (light when the system states none).
const (
	themeSystem = ""
	themeDark   = "dark"
	themeLight  = "light"
)

// systemDark is the system's light/dark preference, kept up to date by
// watchSystemTheme.
var systemDark atomic.Bool

func init() { setPalette(themeDark) }

// paletteFor returns the palette ("dark" or "light") for a stored theme setting.
func paletteFor(setting string) string {
	switch setting {
	case themeDark, themeLight:
		return setting
	}
	if systemDark.Load() {
		return themeDark
	}
	return themeLight
}

// setPalette activates the "dark" or "light" palette.
func setPalette(name string) {
	lightOn = name == themeLight
	if lightOn {
		colBg = color.NRGBA{0xf5, 0xf6, 0xf9, 0xff}
		colBg2 = color.NRGBA{0xe9, 0xeb, 0xf1, 0xff}
		colBg3 = color.NRGBA{0xdc, 0xdf, 0xe8, 0xff}
		colLine = color.NRGBA{0xc9, 0xcd, 0xd9, 0xff}
		colText = color.NRGBA{0x1b, 0x1e, 0x28, 0xff}
		colMuted = color.NRGBA{0x5f, 0x65, 0x78, 0xff}
		colAccent = color.NRGBA{0x2f, 0x62, 0xe0, 0xff}
		colOK = color.NRGBA{0x12, 0x8a, 0x5a, 0xff}
		colWarn = color.NRGBA{0xb3, 0x6b, 0x00, 0xff}
		colErr = color.NRGBA{0xd0, 0x30, 0x30, 0xff}
		colHover = color.NRGBA{0x00, 0x00, 0x00, 0x12}
		return
	}
	colBg = color.NRGBA{0x0e, 0x0f, 0x14, 0xff}
	colBg2 = color.NRGBA{0x15, 0x17, 0x1f, 0xff}
	colBg3 = color.NRGBA{0x1d, 0x20, 0x29, 0xff}
	colLine = color.NRGBA{0x2a, 0x2e, 0x3a, 0xff}
	colText = color.NRGBA{0xe7, 0xe9, 0xee, 0xff}
	colMuted = color.NRGBA{0x8b, 0x90, 0xa0, 0xff}
	colAccent = color.NRGBA{0x5b, 0x8c, 0xff, 0xff}
	colOK = color.NRGBA{0x3c, 0xcf, 0x91, 0xff}
	colWarn = color.NRGBA{0xf5, 0xb8, 0x4b, 0xff}
	colErr = color.NRGBA{0xff, 0x6b, 0x6b, 0xff}
	colHover = color.NRGBA{0xff, 0xff, 0xff, 0x12}
}

// appTheme is Carryover's theme: Fyne's default theme with the active palette.
type appTheme struct{}

var _ fyne.Theme = appTheme{}

func (appTheme) Color(n fyne.ThemeColorName, _ fyne.ThemeVariant) color.Color {
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
	case theme.ColorNamePlaceHolder, theme.ColorNameDisabled:
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
		return color.NRGBA{colAccent.R, colAccent.G, colAccent.B, 0x55}
	case theme.ColorNameHover:
		return colHover
	}
	if lightOn {
		return theme.DefaultTheme().Color(n, theme.VariantLight)
	}
	return theme.DefaultTheme().Color(n, theme.VariantDark)
}

func (appTheme) Font(s fyne.TextStyle) fyne.Resource { return theme.DefaultTheme().Font(s) }

func (appTheme) Icon(n fyne.ThemeIconName) fyne.Resource { return theme.DefaultTheme().Icon(n) }

func (appTheme) Size(n fyne.ThemeSizeName) float32 {
	switch n {
	case theme.SizeNameInputRadius, theme.SizeNameSelectionRadius:
		return 6
	}
	return theme.DefaultTheme().Size(n)
}
