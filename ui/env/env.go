// Package env is what a page or a shared widget sees of the application: the
// window, the services, the shared state and a few actions (go to another page,
// show a toast). Pages depend on this interface, never on the window code that
// implements it, so they can be built and tested with a stand-in.
//
// Everything here is used on the MyGo UI goroutine only.
package env

import (
	"github.com/egoist/mygo"

	"github.com/akfaiz/carryover/ui/envcore"
	"github.com/akfaiz/carryover/ui/kit"
)

// Env is the application as a page sees it.
type Env interface {
	// Window is the window the page is shown in.
	Window() *mygo.Window
	Navigator() *kit.Navigator
	// Dialogs is the stack of dialogs the window shows over the page.
	Dialogs() *kit.Dialogs

	// Services are the services and the shared state (Store), the part a
	// ViewModel's backend may hold.
	envcore.Services

	// Navigate opens a page and keeps the current one in the history; Replace
	// opens it in place of the current one; Back returns to the previous one.
	// Leaving a form with unsaved changes asks first.
	Navigate(path string)
	Replace(path string)
	Back()

	Toast(text string, kind kit.ToastKind)
	ToastErr(err error) // nil is ignored
	// Confirm asks a yes/no question in a dialog; ok runs on yes.
	Confirm(c kit.ConfirmOptions, ok func())
	OpenFolder(path string)
	// OpenURL opens an address in the system browser (or the app registered for it).
	OpenURL(url string) error
	// Prefs keeps the user's small choices.
	Prefs() Prefs

	// AddGame opens the "add a game" dialog.
	AddGame()
	// SyncAll starts "Sync all", or cancels it while it runs.
	SyncAll()
	// ApplyTheme switches to the palette of a stored theme setting.
	ApplyTheme(setting string)
	ToggleFullScreen()
	// FocusForPad puts the gamepad's focus where the open page starts.
	FocusForPad()
	ShowNotices()
	// HelpOpen holds which help texts are unfolded this session, by summary.
	HelpOpen() map[string]bool
}
