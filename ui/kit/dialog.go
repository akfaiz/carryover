package kit

import (
	"os"
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
// at most width wide and always inside the window. A gamepad's B (or Escape)
// closes it.
func ShowCustom(win fyne.Window, title string, body fyne.CanvasObject, width float32) Hider {
	body, ring := ringedBody(win, body)
	d := dialog.NewCustomWithoutButtons(title, body, win)
	d.Resize(fyne.NewSize(min(width, win.Canvas().Size().Width-20), 0))
	d.Show()
	OnBack(win, d.Hide)
	ring()
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
	content, ring := ringedBody(win, content)
	d := dialog.NewCustomWithoutButtons(title, content, win)
	d.Resize(DialogSize(win))
	d.Show()
	OnBack(win, d.Hide)
	ring()
	return d
}

// ShowFilePicker shows a Fyne file or folder dialog at the large dialog size.
// A gamepad's B cancels it, and with a gamepad connected the focus starts in
// the list of files: A opens a folder or picks a file, the D-pad leaves the
// list at its edges for Open and Cancel.
func ShowFilePicker(win fyne.Window, d *dialog.FileDialog) {
	d.Resize(DialogSize(win))
	d.Show()
	OnBack(win, d.Dismiss) // Cancel: the caller hears "nothing chosen"
	if top := win.Canvas().Overlays().Top(); top != nil {
		FocusSafe(win, fileView(top))
	}
}

// fileView is the biggest list or grid in a file dialog: its files (the
// other list holds the places).
func fileView(root fyne.CanvasObject) fyne.Focusable {
	var best fyne.Focusable
	var area float32
	for _, t := range walkNav(root).targets {
		switch t.obj.(type) {
		case *widget.List, *widget.GridWrap:
			if a := t.rect.W * t.rect.H; a > area {
				best, area = t.obj, a
			}
		}
	}
	return best
}

// ExistingDir is the folder a file or folder dialog should open at for an
// absolute path: the path itself if it is a folder, otherwise the nearest
// folder above it that exists. It is "" for an empty or relative path, or when
// only the drive or file system root is left (the dialog's own default is
// better then). It reads the disk: call it off the UI goroutine.
func ExistingDir(path string) string {
	if path == "" || !filepath.IsAbs(path) {
		return ""
	}
	for p := filepath.Clean(path); ; {
		parent := filepath.Dir(p)
		if parent == p {
			return ""
		}
		if fi, err := os.Stat(p); err == nil && fi.IsDir() {
			return p
		}
		p = parent
	}
}

// StartIn makes a file or folder dialog open at dir (from ExistingDir); ""
// keeps the dialog's default.
func StartIn(d *dialog.FileDialog, dir string) {
	if dir == "" {
		return
	}
	if l, err := storage.ListerForURI(storage.NewFileURI(dir)); err == nil {
		d.SetLocation(l)
	}
}

// PickFile asks for a file, optionally limited to extensions such as ".png",
// and passes its path to done. The dialog opens at dir (from ExistingDir; ""
// for its default). Cancelling does nothing.
func PickFile(win fyne.Window, title string, exts []string, dir string, done func(path string)) {
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
	StartIn(d, dir)
	ShowFilePicker(win, d)
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
	FocusSafe(win, cancel)
}

// ConfirmDiscard asks whether to leave something with unsaved changes; leave
// runs if the user agrees.
func ConfirmDiscard(win fyne.Window, title, message, keepLabel, discardLabel string, leave func()) {
	var d Hider
	keep := widget.NewButton(keepLabel, func() { d.Hide() })
	body := VBox(
		Muted(message),
		HBox(layout.NewSpacer(), keep,
			DangerButton(discardLabel, func() { d.Hide(); leave() })),
	)
	d = ShowCustom(win, title, body, 440)
	FocusSafe(win, keep)
}
