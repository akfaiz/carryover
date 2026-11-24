package env

import "github.com/akfaiz/carryover/ui/store"

// The optional contracts a page implements, whatever the GUI toolkit.

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

// Redrawable pages can be drawn again from their state.
type Redrawable interface {
	Redraw()
}

// Prefs is where the GUI keeps the user's small choices (the library's filter
// and sort, full screen).
type Prefs interface {
	StringWithFallback(key, fallback string) string
	SetString(key, value string)
	BoolWithFallback(key string, fallback bool) bool
	SetBool(key string, value bool)
}
