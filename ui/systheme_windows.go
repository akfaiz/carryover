//go:build windows

package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
)

// initialSystemDark reports whether Windows is set to dark mode for apps
// (Fyne reads it from the registry when the app starts).
func initialSystemDark(a fyne.App) bool {
	return a.Settings().ThemeVariant() == theme.VariantDark
}

// watchSystemTheme calls onChange with the system's preference when Fyne
// reloads its settings.
func watchSystemTheme(a fyne.App, onChange func(dark bool)) {
	a.Settings().AddListener(func(s fyne.Settings) {
		onChange(s.ThemeVariant() == theme.VariantDark)
	})
}
