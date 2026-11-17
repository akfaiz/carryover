//go:build darwin && cgo

package kit

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Cocoa
#import <Cocoa/Cocoa.h>

static void setFloating(uintptr_t w) {
	NSWindow *win = (NSWindow *)w;
	[win setLevel:NSFloatingWindowLevel];
}
*/
import "C"

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver"
)

// SetTopmost keeps the wrapper window above other windows (Fyne has no
// always-on-top option): it raises the NSWindow to the floating level.
func SetTopmost(w fyne.Window) {
	nw, ok := w.(driver.NativeWindow)
	if !ok {
		return
	}
	nw.RunNative(func(ctx any) {
		mc, ok := ctx.(driver.MacWindowContext)
		if !ok || mc.NSWindow == 0 {
			return
		}
		C.setFloating(C.uintptr_t(mc.NSWindow))
	})
}
