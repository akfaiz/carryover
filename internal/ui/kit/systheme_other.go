//go:build !windows

package kit

import (
	"time"

	"fyne.io/fyne/v2"
	portal "github.com/rymdport/portal/settings"
	"github.com/rymdport/portal/settings/appearance"
)

// The desktop's light/dark preference comes from the XDG desktop portal. Fyne
// reads it too, but without a portal it falls back to dark; Carryover falls back
// to light, so it starts light unless the system asks for dark.

// InitialSystemDark asks the portal for the color scheme, waiting at most half
// a second: the lookup hangs on some desktops.
func InitialSystemDark(fyne.App) bool {
	res := make(chan bool, 1)
	go func() {
		scheme, err := appearance.GetColorScheme()
		res <- err == nil && scheme == appearance.Dark
	}()
	select {
	case dark := <-res:
		return dark
	case <-time.After(500 * time.Millisecond):
		return false
	}
}

// WatchSystemTheme calls onChange (on a background goroutine) with the
// desktop's color scheme, then again whenever it changes. The first call covers
// a portal that answered too late for initialSystemDark.
func WatchSystemTheme(_ fyne.App, onChange func(dark bool)) {
	go func() {
		if scheme, err := appearance.GetColorScheme(); err == nil {
			onChange(scheme == appearance.Dark)
		}
		// Without a portal this returns an error at once; Fyne already logs it.
		_ = portal.OnSignalSettingChanged(func(c portal.Changed) {
			if c.Namespace != appearance.Namespace || c.Key != "color-scheme" {
				return
			}
			scheme, err := appearance.ValueToColorScheme(c.Value)
			onChange(err == nil && scheme == appearance.Dark)
		})
	}()
}
