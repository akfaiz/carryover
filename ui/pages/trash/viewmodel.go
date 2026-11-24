// Package trash is the Trash page: deleted games and emulators that can still
// be restored, and cloud save folders no game uses any more.
//
// ViewModel holds the lists, what is loading and what a command does, and is
// tested without the toolkit; Page draws it.
package trash

import "github.com/akfaiz/carryover/services"

// Games is what the page needs of the game library.
type Games interface {
	Trash() []services.TrashGame
	Restore(id string) error
	DeleteForever(id string) error
	DeleteWithSaves(id string) error
}

// Emulators is what the page needs of the emulator list.
type Emulators interface {
	Trash() []services.TrashEmulator
	Restore(id string) error
	DeleteForever(id string) error
	DeleteWithSaves(id string) error
}

// Leftovers is what the page needs to find, restore and delete cloud save
// folders no game uses.
type Leftovers interface {
	Leftovers() ([]services.LeftoverView, error)
	RestoreLeftover(key string) (gameID string, err error)
	DeleteLeftover(key string) error
}

// Runner does work off the UI thread and then calls done on it.
type Runner func(work func() error, done func(error))

// Deps is everything the ViewModel talks to.
type Deps struct {
	Games     Games
	Emulators Emulators
	Leftovers Leftovers
	// HasRemote reports whether a cloud storage is set up.
	HasRemote func() bool
	Run       Runner
	// Changed is called, on the UI thread, when the page should be drawn again.
	Changed func()
	// Count is told how many items the trash holds, after each load.
	Count func(n int)
	// Error shows an error to the user; nil is ignored.
	Error func(error)
}

// LeftoverState is what the leftover cloud saves card can say.
type LeftoverState int

const (
	LeftoverNoRemote LeftoverState = iota // no cloud storage to look in
	LeftoverLoading                       // the cloud was asked; it is slower than the rest
	LeftoverFailed                        // the cloud could not be read
	LeftoverNone                          // nothing left over
	LeftoverFound                         // there are some
)

// ViewModel is the Trash page's state and commands.
type ViewModel struct {
	d Deps

	games  []services.TrashGame
	emus   []services.TrashEmulator
	left   []services.LeftoverView
	loaded bool
	// leftLoaded: the cloud was asked for leftover folders.
	leftLoaded  bool
	leftLoading bool
	leftErr     string
	busy        bool
}

// NewViewModel returns a view model working through d.
func NewViewModel(d Deps) *ViewModel { return &ViewModel{d: d} }

// Loaded reports whether the games and emulators have been read once.
func (vm *ViewModel) Loaded() bool { return vm.loaded }

// Games and Emulators are what can be restored.
func (vm *ViewModel) Games() []services.TrashGame         { return vm.games }
func (vm *ViewModel) Emulators() []services.TrashEmulator { return vm.emus }

// Leftovers are the cloud save folders no game uses.
func (vm *ViewModel) Leftovers() []services.LeftoverView { return vm.left }

// LeftoverState says what the leftover card shows; the text of a failure is
// LeftoverError.
func (vm *ViewModel) LeftoverState() LeftoverState {
	switch {
	case !vm.d.HasRemote():
		return LeftoverNoRemote
	case !vm.leftLoaded:
		return LeftoverLoading
	case vm.leftErr != "":
		return LeftoverFailed
	case len(vm.left) == 0:
		return LeftoverNone
	}
	return LeftoverFound
}

// LeftoverError is why the leftover folders could not be read.
func (vm *ViewModel) LeftoverError() string { return vm.leftErr }

// Busy reports whether a restore or deletion runs: the buttons wait.
func (vm *ViewModel) Busy() bool { return vm.busy }

// Open reads everything, when the page is shown.
func (vm *ViewModel) Open() {
	vm.Reload()
	vm.LoadLeftovers()
}

// LibraryChanged reloads after the shared library changed, unless a command
// runs (it reloads itself when done).
func (vm *ViewModel) LibraryChanged() {
	if vm.busy {
		return
	}
	vm.Reload()
	if !vm.leftLoaded {
		vm.LoadLeftovers() // the cloud may have been set up since the page opened
	}
}

// Reload reads the deleted games and emulators.
func (vm *ViewModel) Reload() {
	var games []services.TrashGame
	var emus []services.TrashEmulator
	vm.d.Run(func() error {
		games, emus = vm.d.Games.Trash(), vm.d.Emulators.Trash()
		return nil
	}, func(error) {
		vm.games, vm.emus, vm.loaded = games, emus, true
		vm.d.Count(len(games) + len(emus))
		vm.d.Changed()
	})
}

// LoadLeftovers asks the cloud for save folders no game uses.
func (vm *ViewModel) LoadLeftovers() {
	if !vm.d.HasRemote() || vm.leftLoading {
		return
	}
	vm.leftLoading = true
	var list []services.LeftoverView
	vm.d.Run(func() (err error) {
		list, err = vm.d.Leftovers.Leftovers()
		return err
	}, func(err error) {
		vm.left, vm.leftLoaded, vm.leftLoading, vm.leftErr = list, true, false, ""
		if err != nil {
			vm.leftErr = err.Error()
		}
		vm.d.Changed()
	})
}

// do runs a deletion or restore, then reloads. done runs when it worked.
func (vm *ViewModel) do(work func() error, done func()) {
	vm.busy = true
	vm.d.Run(work, func(err error) {
		vm.busy = false
		vm.d.Error(err)
		if err == nil && done != nil {
			done()
		}
		vm.Reload()
		vm.LoadLeftovers()
	})
}

// RestoreGame takes a game out of the trash; done runs when it worked.
func (vm *ViewModel) RestoreGame(id string, done func()) {
	vm.do(func() error { return vm.d.Games.Restore(id) }, done)
}

// DeleteGame removes a game for good, with its cloud saves and backups if
// saves is set.
func (vm *ViewModel) DeleteGame(id string, saves bool) {
	vm.do(func() error {
		if saves {
			return vm.d.Games.DeleteWithSaves(id)
		}
		return vm.d.Games.DeleteForever(id)
	}, nil)
}

// RestoreEmulator takes an emulator out of the trash; done runs when it worked.
func (vm *ViewModel) RestoreEmulator(id string, done func()) {
	vm.do(func() error { return vm.d.Emulators.Restore(id) }, done)
}

// DeleteEmulator removes an emulator for good, with the cloud saves made with
// it if saves is set.
func (vm *ViewModel) DeleteEmulator(id string, saves bool) {
	vm.do(func() error {
		if saves {
			return vm.d.Emulators.DeleteWithSaves(id)
		}
		return vm.d.Emulators.DeleteForever(id)
	}, nil)
}

// DeleteLeftover deletes a cloud save folder no game uses.
func (vm *ViewModel) DeleteLeftover(key string) {
	vm.do(func() error { return vm.d.Leftovers.DeleteLeftover(key) }, nil)
}

// RestoreLeftover adds a leftover cloud save folder back to the library as a
// game; done gets the new game's id when it worked. The page isn't reloaded:
// the caller goes on to the game's editor.
func (vm *ViewModel) RestoreLeftover(key string, done func(gameID string)) {
	vm.busy = true
	var id string
	vm.d.Run(func() (err error) {
		id, err = vm.d.Leftovers.RestoreLeftover(key)
		return err
	}, func(err error) {
		vm.busy = false
		if err != nil {
			vm.d.Error(err)
			return
		}
		done(id)
	})
}
