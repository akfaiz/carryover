// Package envcore is the part of the application a page's ViewModel and
// backend see that does not depend on the GUI toolkit: the services and the
// shared state. env.Env embeds it and adds the window and the actions of the
// Fyne GUI.
//
// Everything here is used on the UI goroutine only.
package envcore

import (
	"github.com/akfaiz/carryover/services"
	"github.com/akfaiz/carryover/ui/store"
)

// Services are the application's services and its shared state.
type Services interface {
	// Store is the state every page shares.
	Store() *store.Store
	Lib() *services.LibraryService
	Sync() *services.SyncService
	Cloud() *services.CloudService
	Steam() *services.SteamService
	Emus() *services.EmulatorService
	Sess() *services.SessionService
}
