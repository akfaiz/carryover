// Package env is what a page or a shared widget sees of the application: the
// window, the services, the shared state and a few actions (go to another page,
// show a toast). Pages depend on this interface, never on the window code that
// implements it, so they can be built and tested with a stand-in.
//
// Everything here is used on the Fyne UI goroutine only.
package env

import (
	"fyne.io/fyne/v2"

	"github.com/akfaiz/carryover/ui/envcore"
	"github.com/akfaiz/carryover/ui/kit"
)

// Env is the application as a page sees it.
type Env interface {
	// Window is the window the page is shown in; dialogs open on it.
	Window() fyne.Window
	App() fyne.App
	Navigator() *kit.Navigator

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
	OpenFolder(path string)

	// AddGame opens the "add a game" dialog.
	AddGame()
	// SyncAll starts "Sync all", or cancels it while it runs.
	SyncAll()
	// ApplyTheme switches to the palette of a stored theme setting.
	ApplyTheme(setting string)
	ToggleFullScreen()
	// FocusForPad puts the gamepad's focus where the open page starts.
	FocusForPad()
	// RefreshSidebar draws the sidebar again (its Trash count, Exit button).
	RefreshSidebar()
	ShowNotices()

	// HelpOpen holds which help texts are unfolded this session, by summary.
	HelpOpen() map[string]bool
	// Redraw draws the open page again from its state, if it supports that.
	Redraw()
}
