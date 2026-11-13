package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/lang"

	"github.com/akfaiz/carryover/internal/ui/kit"
)

// help is a one-line explanation with a "More" button that unfolds the rest.
// Whether it is unfolded is remembered for the session (by its summary), so
// pages that redraw often don't fold it again.
func (u *UI) help(summary string, details ...string) fyne.CanvasObject {
	if u.helpOpen == nil {
		u.helpOpen = map[string]bool{}
	}
	return kit.Collapsible(summary, details, lang.L("More"), lang.L("Less"), u.helpOpen, func() {
		// Draw the page again, so what follows moves down (or up): Fyne doesn't
		// reliably lay out the parents of something that changed height.
		if r, ok := u.page.(redrawable); ok {
			r.render()
		}
	})
}

// redrawable pages can be drawn again from their state.
type redrawable interface {
	render()
}

// confirmOpts configures confirmDanger.
type confirmOpts struct {
	title, message string
	// With check set, a checkbox (unticked) offers to delete more; checked
	// replaces message while it is ticked, and keptHint is shown while it isn't.
	check, checked, keptHint string
	action                   string // the danger button's label ("Delete" if empty)
	onConfirm                func(checked bool)
}

// confirmDanger asks before something is deleted, in a dialog: the message, an
// optional checkbox to delete more, Cancel and a red action button.
func confirmDanger(u *UI, o confirmOpts) {
	action := o.action
	if action == "" {
		action = lang.L("Delete")
	}
	kit.ConfirmDanger(u.win, kit.ConfirmOptions{
		Title: o.title, Message: o.message, Check: o.check, Checked: o.checked, KeptHint: o.keptHint,
		Action: action, CancelLabel: lang.L("Cancel"), OnConfirm: o.onConfirm,
	})
}

// confirmDiscard asks whether to leave a form with unsaved changes.
func confirmDiscard(u *UI, leave func()) {
	kit.ConfirmDiscard(u.win, lang.L("Discard changes?"), lang.L("Your changes on this page haven't been saved."),
		lang.L("Keep editing"), lang.L("Discard changes"), leave)
}

// dialogHider is a dialog that can be closed.
type dialogHider = kit.Hider

// showCustom shows a dialog without buttons of its own (the body has them),
// at most width wide.
func showCustom(u *UI, title string, body fyne.CanvasObject, width float32) dialogHider {
	return kit.ShowCustom(u.win, title, body, width)
}
