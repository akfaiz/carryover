//go:build !(darwin && cgo)

package kit

import "fyne.io/fyne/v2"

// PickProgram asks for a program and passes its path to done; cancelling
// does nothing. It opens at dir (from ExistingDir; "" for the default).
// Outside macOS it is Fyne's file dialog (PickFile).
func PickProgram(win fyne.Window, title, dir string, done func(path string)) {
	PickFile(win, title, nil, dir, done)
}
