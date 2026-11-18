//go:build !(darwin && cgo)

package kit

import "fyne.io/fyne/v2"

// PickProgram asks for a program and passes its path to done; cancelling
// does nothing. Outside macOS it is Fyne's file dialog (PickFile).
func PickProgram(win fyne.Window, title string, done func(path string)) {
	PickFile(win, title, nil, done)
}
