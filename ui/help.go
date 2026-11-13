package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/lang"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
)

// help is a one-line explanation with a "More" button that unfolds the rest.
// Whether it is unfolded is remembered for the session (by its summary), so
// pages that redraw often don't fold it again.
func (u *UI) help(summary string, details ...string) fyne.CanvasObject {
	if len(details) == 0 {
		return muted(summary)
	}
	if u.helpOpen == nil {
		u.helpOpen = map[string]bool{}
	}
	body := vbox()
	for _, d := range details {
		body.Add(muted(d))
	}
	toggle := lowButton("", nil)
	set := func(open bool) {
		u.helpOpen[summary] = open
		if open {
			toggle.SetText(lang.L("Less"))
			body.Show()
		} else {
			toggle.SetText(lang.L("More"))
			body.Hide()
		}
	}
	toggle.OnTapped = func() {
		set(!u.helpOpen[summary])
		// Draw the page again, so what follows moves down (or up): Fyne doesn't
		// reliably lay out the parents of something that changed height.
		if r, ok := u.page.(redrawable); ok {
			r.render()
		}
	}
	set(u.helpOpen[summary])
	return vbox(container.NewBorder(nil, nil, nil, toggle, muted(summary)), body)
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
	msg := colored(o.message, widget.MediumImportance)
	hint := muted(o.keptHint)
	if o.keptHint == "" {
		hint.Hide()
	}
	body := vbox(msg)
	ticked := false
	if o.check != "" {
		body.Add(newCheck(o.check, false, func(on bool) {
			ticked = on
			if on {
				msg.SetText(o.checked)
				hint.Hide()
			} else {
				msg.SetText(o.message)
				if o.keptHint != "" {
					hint.Show()
				}
			}
		}))
		body.Add(hint)
	}
	action := o.action
	if action == "" {
		action = lang.L("Delete")
	}
	var d dialogHider
	cancel := widget.NewButton(lang.L("Cancel"), func() { d.Hide() })
	del := dangerButton(action, func() {
		d.Hide()
		o.onConfirm(ticked)
	})
	body.Add(hbox(layout.NewSpacer(), cancel, del))
	d = showCustom(u, o.title, body, 520)
}

// confirmDiscard asks whether to leave a form with unsaved changes.
func confirmDiscard(u *UI, leave func()) {
	var d dialogHider
	body := vbox(
		muted(lang.L("Your changes on this page haven't been saved.")),
		hbox(layout.NewSpacer(), widget.NewButton(lang.L("Keep editing"), func() { d.Hide() }),
			dangerButton(lang.L("Discard changes"), func() { d.Hide(); leave() })),
	)
	d = showCustom(u, lang.L("Discard changes?"), body, 440)
}

// dialogHider is a dialog that can be closed.
type dialogHider interface{ Hide() }

// showCustom shows a dialog without buttons of its own (the body has them),
// at most width wide.
func showCustom(u *UI, title string, body fyne.CanvasObject, width float32) dialogHider {
	d := dialog.NewCustomWithoutButtons(title, body, u.win)
	d.Resize(fyne.NewSize(min(width, u.win.Canvas().Size().Width-20), 0))
	d.Show()
	return d
}
