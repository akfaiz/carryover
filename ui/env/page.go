package env

import (
	"fyne.io/fyne/v2"

	"github.com/akfaiz/carryover/ui/store"
)

// Page is one screen of the main window.
type Page interface {
	View() fyne.CanvasObject
}

// Guarded pages hold a form: leaving them while Dirty asks first.
type Guarded interface {
	Dirty() bool
}

// Searchable pages focus their search field on Ctrl+F.
type Searchable interface {
	FocusSearch()
}

// Syncable pages start a sync on F5.
type Syncable interface {
	SyncNow()
}

// Stateful pages are told when shared state changes.
type Stateful interface {
	StateChanged(store.Topic)
}

// PadFocuser pages say where a gamepad starts on them (nil: the top left
// control).
type PadFocuser interface {
	DefaultFocus() fyne.Focusable
}

// Redrawable pages can be drawn again from their state.
type Redrawable interface {
	Redraw()
}
