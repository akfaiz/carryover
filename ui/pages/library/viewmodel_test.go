package library

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/akfaiz/carryover/services"
	"github.com/akfaiz/carryover/ui/store"
)

type fakeSource struct {
	games     []services.GameView
	loaded    bool
	hasRemote bool
	syncAll   store.SyncAllState
}

func (f *fakeSource) Games() []services.GameView  { return f.games }
func (f *fakeSource) Loaded() bool                { return f.loaded }
func (f *fakeSource) HasRemote() bool             { return f.hasRemote }
func (f *fakeSource) SyncAll() store.SyncAllState { return f.syncAll }

type fakePrefs map[string]string

func (p fakePrefs) StringWithFallback(key, fallback string) string {
	if v, ok := p[key]; ok {
		return v
	}
	return fallback
}
func (p fakePrefs) SetString(key, value string) { p[key] = value }

// game is a set-up game, synced at the given time ("" = never).
func game(name, syncedAt string) services.GameView {
	return services.GameView{ID: name, Name: name, Ready: true, Overall: services.GameStatus{LastSyncAt: syncedAt}}
}

func newVM(games ...services.GameView) (*ViewModel, *fakeSource, fakePrefs) {
	src := &fakeSource{games: games, loaded: true, hasRemote: true}
	prefs := fakePrefs{}
	return NewViewModel(src, prefs), src, prefs
}

func names(games []services.GameView) []string {
	var out []string
	for _, g := range games {
		out = append(out, g.Name)
	}
	return out
}

func TestVisibleSortsByNameIgnoringCase(t *testing.T) {
	vm, _, _ := newVM(game("hades", ""), game("Celeste", ""), game("Abzu", ""))
	assert.Equal(t, []string{"Abzu", "Celeste", "hades"}, names(vm.Visible()))
}

func TestVisibleSortsByLastSyncNewestFirstNeverLast(t *testing.T) {
	vm, _, _ := newVM(
		game("Never", ""),
		game("Old", "2026-01-01T10:00:00Z"),
		game("New", "2026-03-01T10:00:00Z"),
	)
	vm.SetSort(SortRecent)
	assert.Equal(t, []string{"New", "Old", "Never"}, names(vm.Visible()))
}

func TestVisibleSearchesTheNameIgnoringCaseAndPadding(t *testing.T) {
	vm, _, _ := newVM(game("Hades", ""), game("Celeste", ""), game("Hades II", ""))
	vm.SetQuery("  hAD ")
	assert.Equal(t, []string{"Hades", "Hades II"}, names(vm.Visible()))
	assert.Equal(t, "hAD", vm.Query(), "trimmed for matching and for the empty message")
}

func TestFilters(t *testing.T) {
	notReady := services.GameView{ID: "n", Name: "Not set up"}
	conflict := game("Conflict", "")
	conflict.Overall.Conflict = true
	running := game("Running", "")
	running.Overall.Running = true
	running.Overall.Conflict = true // a running game is never asked about
	vm, _, _ := newVM(game("Fine", ""), notReady, conflict, running)

	assert.Len(t, vm.Visible(), 4)

	vm.SetFilter(FilterReady)
	assert.Equal(t, []string{"Conflict", "Fine", "Running"}, names(vm.Visible()), "ready: set up on this device")

	vm.SetFilter(FilterAttention)
	assert.Equal(t, []string{"Conflict", "Not set up"}, names(vm.Visible()))

	assert.Equal(t, Counts{All: 4, Ready: 3, Attention: 2}, vm.Counts())
}

func TestFilterAndSortAreKeptInPrefs(t *testing.T) {
	vm, _, prefs := newVM()
	assert.Equal(t, FilterAll, vm.Filter(), "defaults")
	assert.Equal(t, SortName, vm.Sort())

	vm.SetFilter(FilterReady)
	vm.SetSort(SortRecent)
	assert.Equal(t, "ready", prefs["library.filter"])
	assert.Equal(t, "recent", prefs["library.sort"])

	again := NewViewModel(&fakeSource{}, prefs)
	assert.Equal(t, FilterReady, again.Filter(), "a new page picks them up")
	assert.Equal(t, SortRecent, again.Sort())
}

func TestShowFilters(t *testing.T) {
	vm, src, _ := newVM()
	assert.False(t, vm.ShowFilters(), "no games")

	src.games = []services.GameView{game("A", ""), game("B", "")}
	assert.False(t, vm.ShowFilters(), "everything ready and fine: nothing to filter for")

	src.games = append(src.games, services.GameView{Name: "C"})
	assert.True(t, vm.ShowFilters(), "a game that isn't set up here")

	src.games = []services.GameView{game("A", ""), game("B", "")}
	vm.SetFilter(FilterAttention)
	assert.True(t, vm.ShowFilters(), "a filter that is on can be turned off")
}

func TestEmptyStates(t *testing.T) {
	vm, src, _ := newVM()
	assert.Equal(t, EmptyNoGames, vm.Empty(), "an empty library")

	src.loaded = false
	assert.Equal(t, EmptyNone, vm.Empty(), "still loading: say nothing yet")
	src.loaded = true

	src.games = []services.GameView{game("Hades", "")}
	assert.Equal(t, EmptyNone, vm.Empty(), "there is a game to show")

	vm.SetQuery("zelda")
	assert.Equal(t, EmptyNoMatch, vm.Empty(), "the search matches nothing")
	vm.SetQuery("")

	vm.SetFilter(FilterAttention)
	assert.Equal(t, EmptyNoAttention, vm.Empty(), "every game is set up and synced")

	src.games = []services.GameView{{Name: "Not set up"}}
	vm.SetFilter(FilterReady)
	assert.Equal(t, EmptyNoReady, vm.Empty(), "no game can be played here")

	vm.SetQuery("zelda")
	assert.Equal(t, EmptyNoMatch, vm.Empty(), "a search explains itself first")
}

func TestSyncButton(t *testing.T) {
	vm, src, _ := newVM(game("A", ""))

	state, enabled := vm.SyncButton()
	assert.Equal(t, SyncIdle, state)
	assert.True(t, enabled)
	assert.True(t, vm.CanSyncNow())

	src.hasRemote = false
	_, enabled = vm.SyncButton()
	assert.False(t, enabled, "no cloud storage to sync with")
	assert.False(t, vm.CanSyncNow())
	src.hasRemote = true

	src.syncAll = store.SyncAllState{Running: true}
	state, enabled = vm.SyncButton()
	assert.Equal(t, SyncRunning, state, "the button cancels while it runs")
	assert.True(t, enabled)
	assert.False(t, vm.CanSyncNow(), "F5 doesn't start a second one")

	src.syncAll.Cancelling = true
	state, enabled = vm.SyncButton()
	assert.Equal(t, SyncCancelling, state)
	assert.False(t, enabled, "once asked to stop there is nothing more to press")
}

func TestProgress(t *testing.T) {
	vm, src, _ := newVM()
	_, ok := vm.Progress()
	assert.False(t, ok, "nothing runs")

	src.syncAll = store.SyncAllState{Running: true}
	_, ok = vm.Progress()
	assert.False(t, ok, "running, but the size isn't known yet")

	src.syncAll.Progress = services.SyncAllProgress{Done: 1, Total: 3, Name: "Hades"}
	p, ok := vm.Progress()
	require.True(t, ok)
	assert.Equal(t, SyncProgress{Done: 1, Total: 3, Name: "Hades"}, p)

	src.syncAll.Running = false
	_, ok = vm.Progress()
	assert.False(t, ok, "a finished run shows nothing, whatever it left behind")
}
