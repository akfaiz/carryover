// Package library is the library page: the grid of games with its search,
// filters, sort order and "Sync all".
//
// The page is split in two. ViewModel holds everything that can be decided
// without a widget (which games show, in which order, which empty state, what
// the Sync all button does) and is tested without the toolkit. Page is the view: it
// reads the ViewModel and draws it in every frame.
package library

import (
	"slices"
	"strings"

	"github.com/akfaiz/carryover/services"
	"github.com/akfaiz/carryover/ui/store"
)

// Filter narrows the library to some of its games.
type Filter string

const (
	FilterAll       Filter = "all"
	FilterReady     Filter = "ready" // the games that can be played on this device
	FilterAttention Filter = "attention"
)

// Sort is the order of the games.
type Sort string

const (
	SortName   Sort = "name"
	SortRecent Sort = "recent" // last synced first, never synced last
)

// Preference keys the choices are kept under.
const (
	prefFilter = "library.filter"
	prefSort   = "library.sort"
)

// Prefs is where the chosen filter and sort order are kept: the GUI's
// preferences (env.Prefs), or a fake in tests.
type Prefs interface {
	StringWithFallback(key, fallback string) string
	SetString(key, value string)
}

// Source is the shared state the library shows.
type Source interface {
	Games() []services.GameView
	Loaded() bool    // the first read of the library has finished
	HasRemote() bool // a cloud storage is set up
	SyncAll() store.SyncAllState
}

// StoreSource reads the library's state from the application's store.
func StoreSource(s *store.Store) Source { return storeSource{s} }

type storeSource struct{ s *store.Store }

func (a storeSource) Games() []services.GameView  { return a.s.Games }
func (a storeSource) Loaded() bool                { return a.s.Loaded }
func (a storeSource) HasRemote() bool             { return a.s.Settings.Remote != "" }
func (a storeSource) SyncAll() store.SyncAllState { return a.s.SyncAll }

// EmptyState is why the grid has no games, when it has none.
type EmptyState int

const (
	EmptyNone        EmptyState = iota // there are games to show, or the library is still loading
	EmptyNoGames                       // the library itself is empty
	EmptyNoMatch                       // the search matches nothing
	EmptyNoReady                       // no game can be played on this device
	EmptyNoAttention                   // nothing needs attention
)

// Counts are the numbers on the filter chips.
type Counts struct{ All, Ready, Attention int }

// SyncState is what the Sync all button does now.
type SyncState int

const (
	SyncIdle       SyncState = iota // starts a Sync all
	SyncRunning                     // cancels it
	SyncCancelling                  // waiting for it to stop
)

// SyncProgress is how far a Sync all has come.
type SyncProgress struct {
	Done, Total int
	Name        string // the game being synced
}

// ViewModel is the library page's state and rules.
type ViewModel struct {
	src   Source
	prefs Prefs
	query string
}

// NewViewModel returns a view model reading src and keeping its choices in
// prefs.
func NewViewModel(src Source, prefs Prefs) *ViewModel {
	return &ViewModel{src: src, prefs: prefs}
}

// Query is the search text, without the spaces around it.
func (vm *ViewModel) Query() string { return strings.TrimSpace(vm.query) }

// SetQuery sets the search text as typed.
func (vm *ViewModel) SetQuery(q string) { vm.query = q }

// Filter is the chosen filter, as last stored.
func (vm *ViewModel) Filter() Filter {
	return Filter(vm.prefs.StringWithFallback(prefFilter, string(FilterAll)))
}

// SetFilter chooses and stores a filter.
func (vm *ViewModel) SetFilter(f Filter) { vm.prefs.SetString(prefFilter, string(f)) }

// Sort is the chosen order, as last stored.
func (vm *ViewModel) Sort() Sort {
	return Sort(vm.prefs.StringWithFallback(prefSort, string(SortName)))
}

// SetSort chooses and stores an order.
func (vm *ViewModel) SetSort(s Sort) { vm.prefs.SetString(prefSort, string(s)) }

// Visible lists the games the search and the filter let through, in the chosen
// order.
func (vm *ViewModel) Visible() []services.GameView {
	q := strings.ToLower(vm.Query())
	filter := vm.Filter()
	var out []services.GameView
	for _, g := range vm.src.Games() {
		if !strings.Contains(strings.ToLower(g.Name), q) ||
			(filter == FilterAttention && !g.NeedsAttention()) || (filter == FilterReady && !g.Ready) {
			continue
		}
		out = append(out, g)
	}
	if vm.Sort() == SortRecent {
		// The times are RFC 3339, so they sort as text.
		slices.SortStableFunc(out, func(a, b services.GameView) int {
			return strings.Compare(b.Overall.LastSyncAt, a.Overall.LastSyncAt)
		})
	} else {
		slices.SortStableFunc(out, func(a, b services.GameView) int {
			return strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name))
		})
	}
	return out
}

// Counts returns the numbers for the filter chips.
func (vm *ViewModel) Counts() Counts {
	games := vm.src.Games()
	c := Counts{All: len(games)}
	for _, g := range games {
		if g.NeedsAttention() {
			c.Attention++
		}
		if g.Ready {
			c.Ready++
		}
	}
	return c
}

// HasGames reports whether the library has any game at all.
func (vm *ViewModel) HasGames() bool { return len(vm.src.Games()) > 0 }

// ShowFilters reports whether the filter row is shown: only when there is
// something to filter for, or a filter is on (so it can be turned off again).
func (vm *ViewModel) ShowFilters() bool {
	if !vm.HasGames() {
		return false
	}
	c := vm.Counts()
	return c.Attention > 0 || c.Ready < c.All || vm.Filter() != FilterAll
}

// Empty says why the grid shows no game, or EmptyNone when it shows some (or
// the library has not loaded yet).
func (vm *ViewModel) Empty() EmptyState {
	loaded := vm.src.Loaded()
	switch {
	case loaded && !vm.HasGames():
		return EmptyNoGames
	case len(vm.Visible()) > 0:
		return EmptyNone
	case vm.Query() != "":
		return EmptyNoMatch
	case loaded && vm.Filter() == FilterReady:
		return EmptyNoReady
	case loaded:
		return EmptyNoAttention
	}
	return EmptyNone
}

// SyncButton is what the Sync all button does, and whether it can be pressed.
func (vm *ViewModel) SyncButton() (state SyncState, enabled bool) {
	sa := vm.src.SyncAll()
	switch {
	case sa.Cancelling:
		return SyncCancelling, false
	case sa.Running:
		return SyncRunning, true
	}
	return SyncIdle, vm.src.HasRemote()
}

// CanSyncNow reports whether F5 (or the controller's Y) may start a Sync all.
func (vm *ViewModel) CanSyncNow() bool {
	return !vm.src.SyncAll().Running && vm.src.HasRemote()
}

// Progress is how far a running Sync all has come; ok is false when none runs
// or its size is not known yet.
func (vm *ViewModel) Progress() (p SyncProgress, ok bool) {
	sa := vm.src.SyncAll()
	if !sa.Running || sa.Progress.Total == 0 {
		return SyncProgress{}, false
	}
	return SyncProgress{Done: sa.Progress.Done, Total: sa.Progress.Total, Name: sa.Progress.Name}, true
}
