package components

import (
	"fyne.io/fyne/v2"

	"github.com/akfaiz/carryover/ui/i18n"

	"github.com/akfaiz/carryover/ui/env"
	"github.com/akfaiz/carryover/ui/kit"
)

// help is a one-line explanation with a "More" button that unfolds the rest.
// Whether it is unfolded is remembered for the session (by its summary), so
// pages that redraw often don't fold it again.
func Help(u env.Env, summary string, details ...string) fyne.CanvasObject {
	// Draw the page again, so what follows moves down (or up): Fyne doesn't
	// reliably lay out the parents of something that changed height.
	return kit.Collapsible(summary, details, i18n.L("More"), i18n.L("Less"), u.HelpOpen(), u.Redraw)
}

// ConfirmOpts configures ConfirmDanger.
type ConfirmOpts struct {
	Title, Message string
	// With Check set, a checkbox (unticked) offers to delete more; Checked
	// replaces Message while it is ticked, and KeptHint is shown while it isn't.
	Check, Checked, KeptHint string
	Action                   string // the danger button's label ("Delete" if empty)
	OnConfirm                func(Checked bool)
}

// ConfirmDanger asks before something is deleted, in a dialog: the message, an
// optional checkbox to delete more, Cancel and a red action button.
func ConfirmDanger(u env.Env, o ConfirmOpts) {
	action := o.Action
	if action == "" {
		action = i18n.L("Delete")
	}
	kit.ConfirmDanger(u.Window(), kit.ConfirmOptions{
		Title: o.Title, Message: o.Message, Check: o.Check, Checked: o.Checked, KeptHint: o.KeptHint,
		Action: action, CancelLabel: i18n.L("Cancel"), OnConfirm: o.OnConfirm,
	})
}

// ConfirmDiscard asks whether to leave a form with unsaved changes.
func ConfirmDiscard(u env.Env, leave func()) {
	kit.ConfirmDiscard(u.Window(), i18n.L("Discard changes?"), i18n.L("Your changes on this page haven't been saved."),
		i18n.L("Keep editing"), i18n.L("Discard changes"), leave)
}

// DialogHider is a dialog that can be closed.
type DialogHider = kit.Hider

// ShowCustom shows a dialog without buttons of its own (the body has them),
// at most width wide.
func ShowCustom(u env.Env, title string, body fyne.CanvasObject, width float32) DialogHider {
	return kit.ShowCustom(u.Window(), title, body, width)
}

// CopyField is a read-only text with a Copy button that tells it copied.
func CopyField(u env.Env, text string) fyne.CanvasObject {
	return kit.NewCopyField(i18n.L("Copy"), text, u.App().Clipboard(), func() { u.Toast(i18n.L("Copied"), kit.ToastOK) })
}
