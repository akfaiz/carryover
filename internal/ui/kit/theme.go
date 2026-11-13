package kit

import (
	"image/color"
	"sync/atomic"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
)

// Palette (kept from the previous web UI). The variables hold the active
// palette; setPalette swaps them, so widgets built afterwards pick it up.
var (
	ColBg     color.NRGBA
	ColBg2    color.NRGBA
	ColBg3    color.NRGBA
	ColLine   color.NRGBA
	ColText   color.NRGBA
	ColMuted  color.NRGBA
	ColAccent color.NRGBA
	ColOK     color.NRGBA
	ColWarn   color.NRGBA
	ColErr    color.NRGBA

	ColHover color.NRGBA
	LightOn  bool
)

// Theme names stored in settings; themeSystem follows the system's light/dark
// preference (light when the system states none).
const (
	ThemeSystem = ""
	ThemeDark   = "dark"
	ThemeLight  = "light"
)

// SystemDark is the system's light/dark preference, kept up to date by
// watchSystemTheme.
var SystemDark atomic.Bool

func init() { SetPalette(ThemeDark) }

// PaletteFor returns the palette ("dark" or "light") for a stored theme setting.
func PaletteFor(setting string) string {
	switch setting {
	case ThemeDark, ThemeLight:
		return setting
	}
	if SystemDark.Load() {
		return ThemeDark
	}
	return ThemeLight
}

// SetPalette activates the "dark" or "light" palette.
func SetPalette(name string) {
	LightOn = name == ThemeLight
	if LightOn {
		ColBg = color.NRGBA{0xf5, 0xf6, 0xf9, 0xff}
		ColBg2 = color.NRGBA{0xe9, 0xeb, 0xf1, 0xff}
		ColBg3 = color.NRGBA{0xdc, 0xdf, 0xe8, 0xff}
		ColLine = color.NRGBA{0xc9, 0xcd, 0xd9, 0xff}
		ColText = color.NRGBA{0x1b, 0x1e, 0x28, 0xff}
		ColMuted = color.NRGBA{0x5f, 0x65, 0x78, 0xff}
		ColAccent = color.NRGBA{0x2f, 0x62, 0xe0, 0xff}
		ColOK = color.NRGBA{0x12, 0x8a, 0x5a, 0xff}
		ColWarn = color.NRGBA{0xb3, 0x6b, 0x00, 0xff}
		ColErr = color.NRGBA{0xd0, 0x30, 0x30, 0xff}
		ColHover = color.NRGBA{0x00, 0x00, 0x00, 0x12}
		return
	}
	ColBg = color.NRGBA{0x0e, 0x0f, 0x14, 0xff}
	ColBg2 = color.NRGBA{0x15, 0x17, 0x1f, 0xff}
	ColBg3 = color.NRGBA{0x1d, 0x20, 0x29, 0xff}
	ColLine = color.NRGBA{0x2a, 0x2e, 0x3a, 0xff}
	ColText = color.NRGBA{0xe7, 0xe9, 0xee, 0xff}
	ColMuted = color.NRGBA{0x8b, 0x90, 0xa0, 0xff}
	ColAccent = color.NRGBA{0x5b, 0x8c, 0xff, 0xff}
	ColOK = color.NRGBA{0x3c, 0xcf, 0x91, 0xff}
	ColWarn = color.NRGBA{0xf5, 0xb8, 0x4b, 0xff}
	ColErr = color.NRGBA{0xff, 0x6b, 0x6b, 0xff}
	ColHover = color.NRGBA{0xff, 0xff, 0xff, 0x12}
}

// AppTheme is Carryover's theme: Fyne's default theme with the active palette.
type AppTheme struct{}

var _ fyne.Theme = AppTheme{}

func (AppTheme) Color(n fyne.ThemeColorName, _ fyne.ThemeVariant) color.Color {
	switch n {
	case theme.ColorNameBackground:
		return ColBg
	case theme.ColorNameButton:
		return ColBg3
	case theme.ColorNameDisabledButton:
		return ColBg2
	case theme.ColorNameInputBackground:
		return ColBg
	case theme.ColorNameInputBorder, theme.ColorNameSeparator:
		return ColLine
	case theme.ColorNameMenuBackground, theme.ColorNameOverlayBackground, theme.ColorNameHeaderBackground:
		return ColBg2
	case theme.ColorNameForeground:
		return ColText
	case theme.ColorNamePlaceHolder, theme.ColorNameDisabled:
		return ColMuted
	case theme.ColorNamePrimary, theme.ColorNameHyperlink, theme.ColorNameFocus:
		return ColAccent
	case theme.ColorNameSuccess:
		return ColOK
	case theme.ColorNameWarning:
		return ColWarn
	case theme.ColorNameError:
		return ColErr
	case theme.ColorNameSelection:
		return color.NRGBA{ColAccent.R, ColAccent.G, ColAccent.B, 0x55}
	case theme.ColorNameHover:
		return ColHover
	}
	if LightOn {
		return theme.DefaultTheme().Color(n, theme.VariantLight)
	}
	return theme.DefaultTheme().Color(n, theme.VariantDark)
}

func (AppTheme) Font(s fyne.TextStyle) fyne.Resource { return theme.DefaultTheme().Font(s) }

func (AppTheme) Icon(n fyne.ThemeIconName) fyne.Resource { return theme.DefaultTheme().Icon(n) }

func (AppTheme) Size(n fyne.ThemeSizeName) float32 {
	switch n {
	case theme.SizeNameInputRadius, theme.SizeNameSelectionRadius:
		return 6
	}
	return theme.DefaultTheme().Size(n)
}
