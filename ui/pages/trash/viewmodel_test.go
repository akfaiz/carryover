package trash

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/akfaiz/carryover/services"
)

// fake is every service the page talks to, recording what was asked of it.
type fake struct {
	games     []services.TrashGame
	emus      []services.TrashEmulator
	left      []services.LeftoverView
	leftErr   error
	workErr   error // returned by every restore and delete
	restoreID string

	calls []string
}

func (f *fake) rec(s string) { f.calls = append(f.calls, s) }

type fakeGames struct{ *fake }

func (f fakeGames) Trash() []services.TrashGame { return f.games }
func (f fakeGames) Restore(id string) error     { f.rec("restore game " + id); return f.workErr }
func (f fakeGames) DeleteForever(id string) error {
	f.rec("delete game " + id)
	return f.workErr
}
func (f fakeGames) DeleteWithSaves(id string) error {
	f.rec("delete game+saves " + id)
	return f.workErr
}

type fakeEmus struct{ *fake }

func (f fakeEmus) Trash() []services.TrashEmulator { return f.emus }
func (f fakeEmus) Restore(id string) error         { f.rec("restore emulator " + id); return f.workErr }
func (f fakeEmus) DeleteForever(id string) error {
	f.rec("delete emulator " + id)
	return f.workErr
}
func (f fakeEmus) DeleteWithSaves(id string) error {
	f.rec("delete emulator+saves " + id)
	return f.workErr
}

type fakeLeft struct{ *fake }

func (f fakeLeft) Leftovers() ([]services.LeftoverView, error) { return f.left, f.leftErr }
func (f fakeLeft) RestoreLeftover(key string) (string, error) {
	f.rec("restore leftover " + key)
	return f.restoreID, f.workErr
}
func (f fakeLeft) DeleteLeftover(key string) error { f.rec("delete leftover " + key); return f.workErr }

// harness is a view model whose work runs at once, with what it told the page.
type harness struct {
	*fake
	vm      *ViewModel
	remote  bool
	changed int
	counts  []int
	errs    []error
}

func newHarness() *harness {
	h := &harness{fake: &fake{}, remote: true}
	h.vm = NewViewModel(Deps{
		Games: fakeGames{h.fake}, Emulators: fakeEmus{h.fake}, Leftovers: fakeLeft{h.fake},
		HasRemote: func() bool { return h.remote },
		Run:       func(work func() error, done func(error)) { done(work()) },
		Changed:   func() { h.changed++ },
		Count:     func(n int) { h.counts = append(h.counts, n) },
		Error: func(err error) {
			if err != nil {
				h.errs = append(h.errs, err)
			}
		},
	})
	return h
}

func TestOpenLoadsEverythingAndCountsTheTrash(t *testing.T) {
	h := newHarness()
	h.games = []services.TrashGame{{ID: "g1", Name: "Hades"}}
	h.emus = []services.TrashEmulator{{ID: "e1", Name: "Eden"}, {ID: "e2", Name: "Ryujinx"}}
	h.left = []services.LeftoverView{{Key: "k1", GameName: "Old"}}
	assert.False(t, h.vm.Loaded(), "nothing is read before the page opens")

	h.vm.Open()

	assert.True(t, h.vm.Loaded())
	assert.Len(t, h.vm.Games(), 1)
	assert.Len(t, h.vm.Emulators(), 2)
	assert.Equal(t, []int{3}, h.counts, "the sidebar counts games and emulators")
	assert.Equal(t, LeftoverFound, h.vm.LeftoverState())
	assert.Len(t, h.vm.Leftovers(), 1)
	assert.Positive(t, h.changed, "the page is drawn")
}

func TestLeftoverStates(t *testing.T) {
	h := newHarness()

	h.remote = false
	h.vm.Open()
	assert.Equal(t, LeftoverNoRemote, h.vm.LeftoverState(), "no cloud storage")

	h.remote = true
	assert.Equal(t, LeftoverLoading, h.vm.LeftoverState(), "not asked yet")
	h.vm.LoadLeftovers()
	assert.Equal(t, LeftoverNone, h.vm.LeftoverState())

	h.leftErr = errors.New("offline")
	h.vm.leftLoaded = false
	h.vm.LoadLeftovers()
	assert.Equal(t, LeftoverFailed, h.vm.LeftoverState())
	assert.Equal(t, "offline", h.vm.LeftoverError())

	h.leftErr = nil
	h.left = []services.LeftoverView{{Key: "k"}}
	h.vm.LoadLeftovers()
	assert.Equal(t, LeftoverFound, h.vm.LeftoverState())
	assert.Empty(t, h.vm.LeftoverError(), "a later success forgets the error")
}

// The cloud is slow: a second ask while one is out waits for it.
func TestLoadLeftoversDoesNotOverlap(t *testing.T) {
	h := newHarness()
	var pending []func(error)
	asks := 0
	h.vm.d.Run = func(work func() error, done func(error)) {
		asks++
		pending = append(pending, func(e error) { done(e) })
	}

	h.vm.LoadLeftovers()
	h.vm.LoadLeftovers()
	assert.Equal(t, 1, asks)

	pending[0](nil)
	h.vm.LoadLeftovers()
	assert.Equal(t, 2, asks, "once it is back, it can ask again")
}

func TestLibraryChangedReloadsOnlyWhenIdle(t *testing.T) {
	h := newHarness()
	h.vm.Open()
	h.games = []services.TrashGame{{ID: "g1"}}

	h.vm.busy = true
	h.vm.LibraryChanged()
	assert.Empty(t, h.vm.Games(), "a running command reloads itself when it is done")

	h.vm.busy = false
	h.vm.LibraryChanged()
	assert.Len(t, h.vm.Games(), 1)
}

func TestLibraryChangedLooksForLeftoversOnceTheCloudIsSetUp(t *testing.T) {
	h := newHarness()
	h.remote = false
	h.vm.Open()
	require.Equal(t, LeftoverNoRemote, h.vm.LeftoverState())

	h.remote = true
	h.left = []services.LeftoverView{{Key: "k"}}
	h.vm.LibraryChanged()
	assert.Equal(t, LeftoverFound, h.vm.LeftoverState())
}

func TestRestoreRunsThenReloadsAndReports(t *testing.T) {
	h := newHarness()
	h.games = []services.TrashGame{{ID: "g1"}}
	h.vm.Open()
	h.games = nil // restored: gone from the trash

	restored := 0
	h.vm.RestoreGame("g1", func() { restored++ })

	assert.Equal(t, []string{"restore game g1"}, h.calls)
	assert.Equal(t, 1, restored)
	assert.Empty(t, h.vm.Games(), "reloaded")
	assert.False(t, h.vm.Busy())
}

func TestFailedRestoreShowsTheErrorAndDoesNotReport(t *testing.T) {
	h := newHarness()
	h.workErr = errors.New("disk full")
	restored := 0
	h.vm.RestoreEmulator("e1", func() { restored++ })

	assert.Zero(t, restored)
	assert.Equal(t, []error{h.workErr}, h.errs)
	assert.False(t, h.vm.Busy(), "the buttons come back")
}

func TestDeleteWithAndWithoutCloudSaves(t *testing.T) {
	h := newHarness()
	h.vm.DeleteGame("g1", false)
	h.vm.DeleteGame("g2", true)
	h.vm.DeleteEmulator("e1", false)
	h.vm.DeleteEmulator("e2", true)
	h.vm.DeleteLeftover("k1")

	assert.Equal(t, []string{
		"delete game g1", "delete game+saves g2",
		"delete emulator e1", "delete emulator+saves e2",
		"delete leftover k1",
	}, h.calls)
}

func TestBusyWhileACommandRuns(t *testing.T) {
	h := newHarness()
	var finish func(error)
	h.vm.d.Run = func(work func() error, done func(error)) {
		if finish == nil { // the command; its reload runs at once
			finish = func(e error) { done(e) }
			return
		}
		done(work())
	}
	h.vm.DeleteGame("g1", false)
	assert.True(t, h.vm.Busy())
	finish(nil)
	assert.False(t, h.vm.Busy())
}

func TestRestoreLeftoverReportsTheNewGameWithoutReloading(t *testing.T) {
	h := newHarness()
	h.restoreID = "new-game"
	h.vm.Open()
	loads := h.changed

	var got string
	h.vm.RestoreLeftover("k1", func(id string) { got = id })

	assert.Equal(t, "new-game", got)
	assert.Equal(t, loads, h.changed, "the caller goes on to the editor, no redraw")
	assert.False(t, h.vm.Busy())
}

func TestRestoreLeftoverFailure(t *testing.T) {
	h := newHarness()
	h.workErr = errors.New("gone")
	called := false
	h.vm.RestoreLeftover("k1", func(string) { called = true })

	assert.False(t, called)
	assert.Equal(t, []error{h.workErr}, h.errs)
	assert.False(t, h.vm.Busy())
}
