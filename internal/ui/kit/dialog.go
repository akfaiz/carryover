package kit

import (
	"path/filepath"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/widget"
)

// Hider is a dialog that can be closed.
type Hider interface{ Hide() }

// ShowCustom shows a dialog without buttons of its own (the body has them),
// at most width wide and always inside the window.
func ShowCustom(win fyne.Window, title string, body fyne.CanvasObject, width float32) Hider {
	d := dialog.NewCustomWithoutButtons(title, body, win)
	d.Resize(fyne.NewSize(min(width, win.Canvas().Size().Width-20), 0))
	d.Show()
	return d
}

// DialogSize is the size of a large dialog (file pickers, lists) in a window.
func DialogSize(w fyne.Window) fyne.Size {
	s := w.Canvas().Size()
	return fyne.NewSize(min(820, s.Width-40), min(600, s.Height-40))
}

// ScrollDialog shows scrolling content in a large dialog without buttons of
// its own; the content has its own.
func ScrollDialog(win fyne.Window, title string, content fyne.CanvasObject) Hider {
	d := dialog.NewCustomWithoutButtons(title, content, win)
	d.Resize(DialogSize(win))
	d.Show()
	return d
}

// PickFile asks for a file, optionally limited to extensions such as ".png",
// and passes its path to done. Cancelling does nothing.
func PickFile(win fyne.Window, title string, exts []string, done func(path string)) {
	d := dialog.NewFileOpen(func(r fyne.URIReadCloser, err error) {
		if err != nil || r == nil {
			return
		}
		p := filepath.FromSlash(r.URI().Path())
		_ = r.Close()
		done(p)
	}, win)
	d.SetTitleText(title)
	if len(exts) > 0 {
		d.SetFilter(storage.NewExtensionFileFilter(exts))
	}
	d.Resize(DialogSize(win))
	d.Show()
}

// ConfirmOptions configures ConfirmDanger. All texts come from the caller.
type ConfirmOptions struct {
	Title, Message string
	// With Check set, an unticked checkbox offers to delete more: Checked
	// replaces Message while it is ticked, and KeptHint is shown while it
	// isn't.
	Check, Checked, KeptHint string
	Action, CancelLabel      string // the red button and Cancel
	OnConfirm                func(checked bool)
}

// ConfirmDanger asks before something is deleted, in a dialog: the message, an
// optional checkbox to delete more, Cancel and a red action button.
func ConfirmDanger(win fyne.Window, o ConfirmOptions) {
	msg := Colored(o.Message, widget.MediumImportance)
	hint := Muted(o.KeptHint)
	if o.KeptHint == "" {
		hint.Hide()
	}
	body := VBox(msg)
	ticked := false
	if o.Check != "" {
		body.Add(NewCheck(o.Check, false, func(on bool) {
			ticked = on
			if on {
				msg.SetText(o.Checked)
				hint.Hide()
			} else {
				msg.SetText(o.Message)
				if o.KeptHint != "" {
					hint.Show()
				}
			}
		}))
		body.Add(hint)
	}
	var d Hider
	cancel := widget.NewButton(o.CancelLabel, func() { d.Hide() })
	del := DangerButton(o.Action, func() {
		d.Hide()
		o.OnConfirm(ticked)
	})
	body.Add(HBox(layout.NewSpacer(), cancel, del))
	d = ShowCustom(win, o.Title, body, 520)
}

// ConfirmDiscard asks whether to leave something with unsaved changes; leave
// runs if the user agrees.
func ConfirmDiscard(win fyne.Window, title, message, keepLabel, discardLabel string, leave func()) {
	var d Hider
	body := VBox(
		Muted(message),
		HBox(layout.NewSpacer(), widget.NewButton(keepLabel, func() { d.Hide() }),
			DangerButton(discardLabel, func() { d.Hide(); leave() })),
	)
	d = ShowCustom(win, title, body, 440)
}
