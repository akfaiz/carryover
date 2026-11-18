// Package emulators is the Emulators page: it lists the emulators and edits
// them, where each is installed on this device and how it is started. A game
// then picks its emulators.
//
// ViewModel holds the list and the commands and is tested without Fyne; Page
// draws it and hosts the shared emulator form.
package emulators

import (
	"github.com/akfaiz/carryover/internal/saves/pathvar"
	"github.com/akfaiz/carryover/services"
)

// Backend is what the page needs of the emulator service.
type Backend interface {
	List() []services.EmulatorView
	Unset(id string) error
	Delete(id string) error
	DeleteWithSaves(id string) error
}

// Runner does work off the UI thread and then calls done on it.
type Runner func(work func() error, done func(error))

// Deps is everything the ViewModel talks to.
type Deps struct {
	Emulators Backend
	// Tokens are the path variables the data folder field offers.
	Tokens func() []pathvar.Token
	Run    Runner
	// Changed is called, on the UI thread, when the page should be drawn again.
	Changed func()
	// Error shows an error to the user; nil is ignored.
	Error func(error)
}

// ViewModel is the Emulators page's state and commands.
type ViewModel struct {
	d       Deps
	list    []services.EmulatorView
	tokens  []pathvar.Token
	loaded  bool
	editing bool // a form is open instead of the list
}

// NewViewModel returns a view model working through d.
func NewViewModel(d Deps) *ViewModel { return &ViewModel{d: d} }

// Loaded reports whether the list has been read once.
func (vm *ViewModel) Loaded() bool { return vm.loaded }

// List is the emulators.
func (vm *ViewModel) List() []services.EmulatorView { return vm.list }

// Tokens are the path variables for the data folder field.
func (vm *ViewModel) Tokens() []pathvar.Token { return vm.tokens }

// Editing reports whether a form is open.
func (vm *ViewModel) Editing() bool { return vm.editing }

// SetEditing records that a form was opened or closed.
func (vm *ViewModel) SetEditing(on bool) { vm.editing = on }

// Reload reads the list.
func (vm *ViewModel) Reload() {
	var list []services.EmulatorView
	var tokens []pathvar.Token
	vm.d.Run(func() error {
		list, tokens = vm.d.Emulators.List(), vm.d.Tokens()
		return nil
	}, func(error) {
		vm.list, vm.tokens, vm.loaded = list, tokens, true
		vm.d.Changed()
	})
}

// LibraryChanged reloads after the shared library changed, unless a form is
// open: the list under it isn't shown, and a redraw would drop the form.
func (vm *ViewModel) LibraryChanged() {
	if !vm.editing {
		vm.Reload()
	}
}

func (vm *ViewModel) then(work func() error) {
	vm.d.Run(work, func(err error) {
		vm.d.Error(err)
		vm.Reload()
	})
}

// Unset clears where an emulator is installed on this device, keeping it
// everywhere else.
func (vm *ViewModel) Unset(id string) {
	vm.then(func() error { return vm.d.Emulators.Unset(id) })
}

// Delete moves an emulator to the trash on all devices, or deletes it for good
// with its cloud saves when saves is set.
func (vm *ViewModel) Delete(id string, saves bool) {
	vm.then(func() error {
		if saves {
			return vm.d.Emulators.DeleteWithSaves(id)
		}
		return vm.d.Emulators.Delete(id)
	})
}

// SetupOf is the form's content for an emulator being edited.
func SetupOf(ev services.EmulatorView) services.EmulatorSetup {
	return services.EmulatorSetup{
		ID: ev.ID, Name: ev.Name, Exe: ev.Device.Exe, Args: ev.Device.Args, WorkDir: ev.Device.WorkDir,
		Env: ev.Device.Env, WaitProcess: ev.Device.WaitProcess, Preset: ev.Preset, DataDir: ev.Device.DataDir,
	}
}
