// Package cloud is the Cloud storage page: the sync location (a remote and a
// folder), the configured remotes, importing an rclone config and the wizard
// that adds a remote.
//
// ViewModel and Wizard hold the state and rules and are tested without the toolkit;
// Page and the wizard's dialog draw them.
package cloud

import (
	"strings"

	"github.com/akfaiz/carryover/internal/app/appdir"
	"github.com/akfaiz/carryover/internal/cloudsync/cloud"
)

// DefaultRoot is the folder on the remote when none is given.
const DefaultRoot = "Carryover"

// Backend is what the page needs of the cloud service.
type Backend interface {
	Remotes() []cloud.Remote
	Providers() []cloud.Provider
	DefaultRcloneConfig() string
	DeleteRemote(name string) error
	ImportConfig(path string) (cloud.ImportResult, error)
}

// Settings is what the page needs of the settings and library sync.
type Settings interface {
	GetSettings() appdir.Settings
	SaveSettings(appdir.Settings) (appdir.Settings, error)
	SyncLibrary() error
}

// Runner does work off the UI thread and then calls done on it.
type Runner func(work func() error, done func(error))

// Deps is everything the ViewModel talks to.
type Deps struct {
	Cloud    Backend
	Settings Settings
	// Current is the settings the app currently runs with (the shared store's).
	Current func() appdir.Settings
	Run     Runner
	// Changed is called, on the UI thread, when the page should be drawn again.
	Changed func()
	// Refresh tells the rest of the app the settings changed.
	Refresh func()
	// Error shows an error to the user; nil is ignored.
	Error func(error)
	// Saved is told the sync location was stored.
	Saved func()
	// SyncFailed is told that the library sync which follows a new location
	// failed.
	SyncFailed func(error)
	// Imported is told what an import found, for the page to report.
	Imported func(cloud.ImportResult)
}

// ViewModel is the Cloud page's state and commands.
type ViewModel struct {
	d           Deps
	remotes     []cloud.Remote
	providers   []cloud.Provider
	defaultConf string
	remote      string // the remote chosen as the sync location (not yet stored)
	root        string
}

// NewViewModel returns a view model working through d.
func NewViewModel(d Deps) *ViewModel { return &ViewModel{d: d, root: DefaultRoot} }

func (vm *ViewModel) Remotes() []cloud.Remote     { return vm.remotes }
func (vm *ViewModel) Providers() []cloud.Provider { return vm.providers }

// DefaultConfig is the path of the user's own rclone config, "" if there is none.
func (vm *ViewModel) DefaultConfig() string { return vm.defaultConf }

// Remote is the chosen remote and Root its folder.
func (vm *ViewModel) Remote() string { return vm.remote }
func (vm *ViewModel) Root() string   { return vm.root }

// SetRemote and SetRoot are the choices made on the page.
func (vm *ViewModel) SetRemote(v string) { vm.remote = v }
func (vm *ViewModel) SetRoot(v string)   { vm.root = v }

// Unsupported reports whether the named remote is of a provider Carryover does
// not include.
func (vm *ViewModel) Unsupported(name string) bool {
	for _, r := range vm.remotes {
		if r.Name == name && !r.Supported {
			return true
		}
	}
	return false
}

// HasRemote reports whether a sync location is stored: the first-run page puts
// the remotes first.
func (vm *ViewModel) HasRemote() bool { return vm.d.Current().Remote != "" }

// IsActive reports whether a remote is the stored sync location.
func (vm *ViewModel) IsActive(name string) bool { return name == vm.d.Current().Remote }

// Open reads everything, when the page is shown.
func (vm *ViewModel) Open() {
	vm.Load(true)
	vm.d.Run(func() error { vm.providers = vm.d.Cloud.Providers(); return nil }, func(error) {})
	var conf string
	vm.d.Run(func() error { conf = vm.d.Cloud.DefaultRcloneConfig(); return nil }, func(error) {
		vm.defaultConf = conf
		vm.d.Changed()
	})
}

// Load reads the remotes and, with target, the stored location.
func (vm *ViewModel) Load(target bool) {
	var remotes []cloud.Remote
	vm.d.Run(func() error { remotes = vm.d.Cloud.Remotes(); return nil }, func(error) {
		vm.remotes = remotes
		if target {
			st := vm.d.Settings.GetSettings()
			vm.remote, vm.root = st.Remote, st.RemoteRoot
			if vm.root == "" {
				vm.root = DefaultRoot
			}
		}
		vm.d.Changed()
	})
}

// SaveTarget stores the chosen remote and folder as the sync location and, when
// there is a remote, syncs the library with it.
func (vm *ViewModel) SaveTarget() {
	st := vm.d.Current()
	st.Remote = vm.remote
	st.RemoteRoot = strings.Trim(strings.TrimSpace(vm.root), "/")
	if st.RemoteRoot == "" {
		st.RemoteRoot = DefaultRoot
	}
	remote := st.Remote
	vm.d.Run(func() error {
		_, err := vm.d.Settings.SaveSettings(st)
		return err
	}, func(err error) {
		if err != nil {
			vm.d.Error(err)
			return
		}
		vm.d.Refresh()
		vm.d.Saved()
		if remote != "" {
			vm.d.Run(vm.d.Settings.SyncLibrary, func(err error) {
				if err != nil {
					vm.d.SyncFailed(err)
				}
			})
		}
	})
}

// UseFirstRemote makes a remote just added the sync location when there is
// none yet, so a first setup needs no extra step.
func (vm *ViewModel) UseFirstRemote(name string) {
	if vm.HasRemote() || name == "" {
		return
	}
	vm.remote = name
	vm.SaveTarget()
}

// RemoteCreated is called when the wizard has made a remote.
func (vm *ViewModel) RemoteCreated(name string) {
	vm.remote = name
	vm.Load(false)
	vm.UseFirstRemote(name)
}

// DeleteRemote removes a remote from the rclone config (its files are not
// touched).
func (vm *ViewModel) DeleteRemote(name string) {
	vm.d.Run(func() error { return vm.d.Cloud.DeleteRemote(name) }, func(err error) {
		vm.d.Error(err)
		vm.Load(false)
		vm.d.Refresh()
	})
}

// ImportConf imports the remotes of an rclone config file. A single new remote
// becomes the sync location if there is none yet.
func (vm *ViewModel) ImportConf(path string) {
	var res cloud.ImportResult
	vm.d.Run(func() (err error) {
		res, err = vm.d.Cloud.ImportConfig(path)
		return err
	}, func(err error) {
		if err != nil {
			vm.d.Error(err)
			return
		}
		vm.d.Imported(res)
		if len(res.Imported) == 1 {
			vm.UseFirstRemote(res.Imported[0])
		}
		vm.Load(false)
	})
}
