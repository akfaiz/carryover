//go:build windows

package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver"
	"golang.org/x/sys/windows"
)

var setWindowPos = windows.NewLazySystemDLL("user32.dll").NewProc("SetWindowPos")

// setTopmost keeps the wrapper window above other windows (Fyne has no
// always-on-top option).
func setTopmost(w fyne.Window) {
	nw, ok := w.(driver.NativeWindow)
	if !ok {
		return
	}
	nw.RunNative(func(ctx any) {
		wc, ok := ctx.(driver.WindowsWindowContext)
		if !ok || wc.HWND == 0 {
			return
		}
		const (
			hwndTopmost   = ^uintptr(0) // HWND_TOPMOST (-1)
			swpNoSize     = 0x0001
			swpNoMove     = 0x0002
			swpNoActivate = 0x0010
		)
		_, _, _ = setWindowPos.Call(wc.HWND, hwndTopmost, 0, 0, 0, 0, swpNoSize|swpNoMove|swpNoActivate)
	})
}
