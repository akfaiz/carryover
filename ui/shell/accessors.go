package shell

import (
	"github.com/egoist/mygo/ui"

	"github.com/akfaiz/carryover/ui/env"
	"github.com/akfaiz/carryover/ui/kit"
	"github.com/akfaiz/carryover/ui/prompts"
)

// The accessors below exist for tests in other packages; the application does
// not use them.

// Route is the open page's route.
func (u *Shell) Route() string { return u.route }

// Page is the open page.
func (u *Shell) Page() env.Page { return u.page }

// ViewFunc is the window's view, to drive with a ui.Tester.
func (u *Shell) ViewFunc() func(*ui.Context) { return u.view }

// History is the routes Back returns to, oldest first.
func (u *Shell) History() []string { return u.history }

// Confirming is the open yes/no dialog, if any.
func (u *Shell) Confirming() *kit.Confirm { return &u.confirm }

// Prompts is the queue of session prompts.
func (u *Shell) Prompts() *prompts.Queue { return u.prompts }
