//go:build darwin && cgo

package kit

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Cocoa
#import <Cocoa/Cocoa.h>
#include <stdlib.h>

// pickProgram shows the system's Open panel, where an app bundle is chosen
// like a file. It returns the chosen path (to be freed), or NULL when
// cancelled. *ok is 0 when it can't run here (not on the main thread).
static char *pickProgram(const char *title, const char *dir, int *ok) {
	if (![NSThread isMainThread]) {
		*ok = 0;
		return NULL;
	}
	*ok = 1;
	@autoreleasepool {
		NSWindow *key = [NSApp keyWindow];
		NSOpenPanel *p = [NSOpenPanel openPanel];
		p.canChooseFiles = YES;
		p.canChooseDirectories = NO;
		p.allowsMultipleSelection = NO;
		p.treatsFilePackagesAsDirectories = NO;
		p.message = [NSString stringWithUTF8String:title];
		p.directoryURL = [NSURL fileURLWithPath:[NSString stringWithUTF8String:dir] isDirectory:YES];
		NSModalResponse res = [p runModal];
		[key makeKeyAndOrderFront:nil];
		if (res != NSModalResponseOK || p.URL == nil) {
			return NULL;
		}
		return strdup(p.URL.path.UTF8String);
	}
}
*/
import "C"

import (
	"path/filepath"
	"strings"
	"unsafe"

	"fyne.io/fyne/v2"
)

// PickProgram asks for a program and passes its path to done; cancelling
// does nothing. On macOS it uses the system's Open panel, starting at dir
// (from ExistingDir) or else in /Applications, because Fyne's file dialog
// shows an app bundle (X.app) as a folder that can't be chosen. If the panel
// can't be shown it falls back to PickFile.
func PickProgram(win fyne.Window, title, dir string, done func(path string)) {
	start := dir
	if strings.HasSuffix(strings.ToLower(start), ".app") {
		start = filepath.Dir(start) // the folder that holds the bundle, not its inside
	}
	if start == "" {
		start = "/Applications"
	}
	ct, cd := C.CString(title), C.CString(start)
	defer C.free(unsafe.Pointer(ct))
	defer C.free(unsafe.Pointer(cd))
	var ok C.int
	res := C.pickProgram(ct, cd, &ok)
	if ok == 0 {
		PickFile(win, title, nil, dir, done)
		return
	}
	if res == nil {
		return
	}
	defer C.free(unsafe.Pointer(res))
	done(C.GoString(res))
}
