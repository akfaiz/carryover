//go:build windows

package kit

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
)

// InitialSystemDark reports whether Windows is set to dark mode for apps
// (Fyne reads it from the registry when the app starts).
func InitialSystemDark(a fyne.App) bool {
	return a.Settings().ThemeVariant() == theme.VariantDark
}

// WatchSystemTheme calls onChange with the system's preference when Fyne
// reloads its settings.
func WatchSystemTheme(a fyne.App, onChange func(dark bool)) {
	a.Settings().AddListener(func(s fyne.Settings) {
		onChange(s.ThemeVariant() == theme.VariantDark)
	})
}
