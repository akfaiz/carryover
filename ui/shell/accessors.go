package shell

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/widget"

	"github.com/akfaiz/carryover/ui/env"
	"github.com/akfaiz/carryover/ui/kit"
	"github.com/akfaiz/carryover/ui/prompts"
	"github.com/akfaiz/carryover/ui/store"
)

// What the application's own tests look at.

// Route is the open page's route.
func (u *Shell) Route() string { return u.route }

// Page is the open page.
func (u *Shell) Page() env.Page { return u.page }

// NavButtons are the sidebar's buttons by route.
func (u *Shell) NavButtons() map[string]*widget.Button { return u.navBtns }

// Sidebar is the sidebar's content.
func (u *Shell) Sidebar() *fyne.Container { return u.sidebar }

// Activity is the sidebar line for a running "Sync all" or game.
func (u *Shell) Activity() *fyne.Container { return u.activity }

// Toasts is the window's toaster.
func (u *Shell) Toasts() *kit.Toaster { return u.toasts }

// Prompts is the queue of session prompts.
func (u *Shell) Prompts() *prompts.Queue { return u.prompts }

// Busy reports whether a play session or a sync is running.
func (u *Shell) Busy() bool { return u.busy() }

// ConfirmClose closes the main window, after asking while something runs.
func (u *Shell) ConfirmClose() { u.confirmClose() }

// ApplyFullScreen opens the window full screen if that was last chosen.
func (u *Shell) ApplyFullScreen() { u.applyFullScreen() }

// Notify tells the open page that shared state changed.
func (u *Shell) Notify(t store.Topic) { u.notify(t) }
