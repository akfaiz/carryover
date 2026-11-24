package env

import "github.com/egoist/mygo/ui"

// Page is one screen of the main window. MyGo redraws the window every frame,
// so a page builds its controls in View each time it is called, from its
// ViewModel and the little UI state it keeps (handles, scroll positions).
type Page interface {
	View(c *ui.Context)
}

// PadFocuser pages say where a gamepad starts on them (nil: the top left
// control).
type PadFocuser interface {
	DefaultFocus() *ui.Handle
}

// Scrollable pages say which scroll container the gamepad's right stick
// scrolls.
type Scrollable interface {
	Scroll() *ui.ScrollState
}
