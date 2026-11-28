package emulators

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/akfaiz/carryover/internal/library"
	"github.com/akfaiz/carryover/internal/saves/pathvar"
	"github.com/akfaiz/carryover/services"
)

type fakeBackend struct {
	list  []services.EmulatorView
	err   error
	calls []string
}

func (f *fakeBackend) List() []services.EmulatorView { return f.list }
func (f *fakeBackend) Unset(id string) error         { f.calls = append(f.calls, "unset "+id); return f.err }
func (f *fakeBackend) Delete(id string) error        { f.calls = append(f.calls, "delete "+id); return f.err }
func (f *fakeBackend) DeleteWithSaves(id string) error {
	f.calls = append(f.calls, "delete+saves "+id)
	return f.err
}

type harness struct {
	*fakeBackend

	vm      *ViewModel
	changed int
	errs    []error
}

func newHarness() *harness {
	h := &harness{fakeBackend: &fakeBackend{}}
	h.vm = NewViewModel(Deps{
		Emulators: h.fakeBackend,
		Tokens:    func() []pathvar.Token { return []pathvar.Token{{Token: "{home}"}} },
		Run:       func(work func() error, done func(error)) { done(work()) },
		Changed:   func() { h.changed++ },
		Error: func(err error) {
			if err != nil {
				h.errs = append(h.errs, err)
			}
		},
	})
	return h
}

func TestReloadReadsTheListAndTheTokens(t *testing.T) {
	h := newHarness()
	h.list = []services.EmulatorView{{ID: "e1", Name: "Eden"}}
	assert.False(t, h.vm.Loaded())

	h.vm.Reload()

	assert.True(t, h.vm.Loaded())
	assert.Len(t, h.vm.List(), 1)
	assert.Len(t, h.vm.Tokens(), 1)
	assert.Equal(t, 1, h.changed)
}

// A form open over the list is not drawn again when the library changes: that
// would drop what was typed into it.
func TestLibraryChangedLeavesAnOpenFormAlone(t *testing.T) {
	h := newHarness()
	h.vm.SetEditing(true)
	h.list = []services.EmulatorView{{ID: "e1"}}
	h.vm.LibraryChanged()
	assert.Empty(t, h.vm.List())
	assert.Zero(t, h.changed)

	h.vm.SetEditing(false)
	h.vm.LibraryChanged()
	assert.Len(t, h.vm.List(), 1)
}

func TestUnsetAndDeleteThenReload(t *testing.T) {
	h := newHarness()
	h.vm.Unset("e1")
	h.vm.Delete("e2", false)
	h.vm.Delete("e3", true)

	assert.Equal(t, []string{"unset e1", "delete e2", "delete+saves e3"}, h.calls)
	assert.Equal(t, 3, h.changed, "the list is read again after each")
}

func TestAFailedCommandShowsTheErrorAndStillReloads(t *testing.T) {
	h := newHarness()
	h.err = errors.New("offline")
	h.vm.Delete("e1", true)

	assert.Equal(t, []error{h.err}, h.errs)
	assert.Equal(t, 1, h.changed, "what is really there is shown")
}

func TestSetupOfCarriesThisDevicesSetup(t *testing.T) {
	ev := services.EmulatorView{
		ID: "e1", Name: "Eden", Preset: "eden",
		Device: library.EmulatorDevice{
			Exe: "/apps/eden", Args: "-f {rom}", WorkDir: "/apps", Env: map[string]string{"K": "v"},
			WaitProcess: "eden.exe", DataDir: "/data",
		},
	}
	assert.Equal(t, services.EmulatorSetup{
		ID: "e1", Name: "Eden", Exe: "/apps/eden", Args: "-f {rom}", WorkDir: "/apps",
		Env: map[string]string{"K": "v"}, WaitProcess: "eden.exe", Preset: "eden", DataDir: "/data",
	}, SetupOf(ev))
}
