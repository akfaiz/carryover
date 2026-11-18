// Package settings is the Settings page. Every setting saves as soon as it
// changes (text fields shortly after the last keystroke), except the
// SteamGridDB key, which has a Save button of its own: it is a secret, and half
// a pasted key is worthless.
//
// ViewModel turns what the form shows into settings and saves them one at a
// time; it is tested without Fyne. Page builds the form.
package settings

import (
	"errors"
	"strconv"
	"strings"

	"github.com/akfaiz/carryover/internal/app/appdir"
)

// DefaultRetention is the backup count offered when cloud backups are turned on.
const DefaultRetention = 5

// ErrRetention says the number of backups to keep is not a number from 1 to 100.
// The page words it in the user's language.
var ErrRetention = errors.New("backups to keep must be a number from 1 to 100")

// ParseRetention parses the number of cloud backups to keep (1-100).
func ParseRetention(s string) (int, error) {
	n, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil || n < 1 || n > 100 {
		return 0, ErrRetention
	}
	return n, nil
}

// Form is what the form shows right now. The SteamGridDB key is not in it: it
// is the saved one until its own button is pressed (ViewModel.SaveKey).
type Form struct {
	DeviceName   string
	BackupsOn    bool
	Retention    string // the text of the field; only read while BackupsOn
	ShareLibrary bool
	UploadCovers bool
	Theme        string // a kit.Theme* setting (the system's is "")
	ThemeChosen  bool   // Theme is a choice; otherwise the stored theme stays
	Language     string // a language code, "" for the system's
}

// Runner does a save off the UI thread and then calls done on it.
type Runner func(work func() (appdir.Settings, error), done func(appdir.Settings, error))

// Deps is everything the ViewModel talks to.
type Deps struct {
	Save func(appdir.Settings) (appdir.Settings, error)
	Run  Runner
	// Saved is called after a save, on the UI thread, before the next one: the
	// page shows its "Saved" note.
	Saved func()
	// Refresh tells the rest of the app the settings changed.
	Refresh func()
	// ThemeChanged applies a new theme. The page is built again afterwards, so
	// the ViewModel does not save anything more after it.
	ThemeChanged func(theme string)
	// Error shows an error to the user.
	Error func(error)
}

// ViewModel is the Settings page's state.
type ViewModel struct {
	d  Deps
	st appdir.Settings // as last saved
	// keyToSave is the SteamGridDB key the next save writes: the saved one until
	// SaveKey is called.
	keyToSave string
	// Saves are one at a time: a change made while one is running is saved
	// right after it, from whatever the form shows then.
	saving, again bool
}

// NewViewModel returns a view model for the settings as loaded.
func NewViewModel(d Deps, st appdir.Settings) *ViewModel {
	return &ViewModel{d: d, st: st, keyToSave: st.SteamGridDBKey}
}

// Settings are the settings as last saved.
func (vm *ViewModel) Settings() appdir.Settings { return vm.st }

// InitialRetention is the count the field starts with: the stored one, or the
// default while backups are off (off is stored as 0).
func (vm *ViewModel) InitialRetention() int {
	if vm.st.BackupRetention <= 0 {
		return DefaultRetention
	}
	return vm.st.BackupRetention
}

// KeyDirty reports whether the typed SteamGridDB key differs from the saved one
// (so its Save button can be pressed).
func (vm *ViewModel) KeyDirty(typed string) bool {
	return strings.TrimSpace(typed) != vm.st.SteamGridDBKey
}

// Collect returns the settings the form shows. ok is false when a value can't be
// saved yet; err says why when the user should hear about it.
func (vm *ViewModel) Collect(f Form) (next appdir.Settings, ok bool, err error) {
	next = vm.st
	n := 0
	if f.BackupsOn {
		if n, err = ParseRetention(f.Retention); err != nil {
			return vm.st, false, err
		}
	}
	if dev := strings.TrimSpace(f.DeviceName); dev != "" {
		next.DeviceName = dev
	}
	next.BackupRetention = n
	next.SyncLibrary = f.ShareLibrary
	next.CoversLocalOnly = !f.UploadCovers
	if f.ThemeChosen {
		next.Theme = f.Theme
	}
	next.Language = f.Language
	next.SteamGridDBKey = vm.keyToSave
	return next, true, nil
}

// Commit saves what read() returns, now, or right after the save that is
// running. Nothing is saved when it equals what is stored.
func (vm *ViewModel) Commit(read func() Form) {
	if vm.saving {
		vm.again = true
		return
	}
	next, ok, err := vm.Collect(read())
	if err != nil {
		vm.d.Error(err)
	}
	if !ok || next == vm.st {
		return
	}
	vm.saving = true
	vm.d.Run(func() (appdir.Settings, error) { return vm.d.Save(next) }, func(saved appdir.Settings, err error) {
		vm.saving = false
		if err != nil {
			vm.d.Error(err)
			return
		}
		themeChanged := saved.Theme != vm.st.Theme
		vm.st = saved
		vm.keyToSave = saved.SteamGridDBKey
		vm.d.Refresh()
		vm.d.Saved()
		if themeChanged {
			vm.d.ThemeChanged(saved.Theme)
			return // the page is built again
		}
		if vm.again {
			vm.again = false
			vm.Commit(read)
		}
	})
}

// SaveKey saves the typed SteamGridDB key (and whatever else changed).
func (vm *ViewModel) SaveKey(typed string, read func() Form) {
	vm.keyToSave = strings.TrimSpace(typed)
	vm.Commit(read)
}
