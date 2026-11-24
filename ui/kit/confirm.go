package kit

import "github.com/egoist/mygo/ui"

// ConfirmOptions is the text of a yes/no dialog. Texts come translated.
type ConfirmOptions struct {
	Title, Body string
	Cancel, OK  string // button labels
	// Danger makes OK a warning-looking action and the dialog start on Cancel
	// (it always does: a dialog starts on its safe button).
	Danger bool
	// With Check set, a checkbox (unticked) offers to do more: Checked
	// replaces Body while it is ticked, and KeptHint is shown while it isn't.
	// OnConfirm gets whether it was ticked.
	Check, Checked, KeptHint string
	OnConfirm                func(checked bool)
}

// Confirm is a yes/no dialog a window owns: Show it from any callback on the UI
// goroutine, and call View in the window's view every frame.
type Confirm struct {
	open bool
	opts ConfirmOptions
	ok   func()
	h    [3]ui.Handle

	ticked bool
}

// Show opens the dialog; ok runs when the user chooses OK.
func (d *Confirm) Show(o ConfirmOptions, ok func()) {
	d.open, d.opts, d.ok, d.ticked = true, o, ok, false
}

// Open reports whether the dialog is showing.
func (d *Confirm) Open() bool { return d.open }

// Close closes the dialog without answering.
func (d *Confirm) Close() { d.open = false }

// View declares the dialog, if it is open, as a scope of nav: B or Escape
// cancels, and it starts on Cancel.
func (d *Confirm) View(c *ui.Context, nav *Navigator) {
	nav.Overlay("confirm", d.open, d.Close, func() {
		ui.Modal(c, &d.open, func() {
			t := c.Theme()
			ui.Column(c).Gap(12).Padding(20).Width(420).Children(func() {
				ui.Text(c, d.opts.Title).FontSize(16).Bold()
				body := d.opts.Body
				if d.ticked && d.opts.Checked != "" {
					body = d.opts.Checked
				}
				ui.Text(c, body).Wrap().TextColor(t.TextMuted)
				if d.opts.Check != "" {
					nav.Item(&d.h[2], ui.Checkbox(c.Key("confirm-check"), &d.ticked, d.opts.Check), func() { d.ticked = !d.ticked })
					if !d.ticked && d.opts.KeptHint != "" {
						ui.Text(c, d.opts.KeptHint).Wrap().FontSize(12).TextColor(t.TextMuted)
					}
				}
				ui.Row(c).Gap(8).Justify(ui.End).Children(func() {
					cancel := func() { d.open = false }
					if nav.Item(&d.h[0], ui.Button(c.Key("confirm-cancel"), d.opts.Cancel), cancel, FocusSafe).Clicked() {
						cancel()
					}
					ok := func() {
						d.open = false
						if d.ok != nil {
							d.ok()
						}
						if d.opts.OnConfirm != nil {
							d.opts.OnConfirm(d.ticked)
						}
					}
					var btn ui.Element
					if d.opts.Danger {
						btn = ui.Button(c.Key("confirm-ok"), d.opts.OK).TextColor(t.Danger)
					} else {
						btn = ui.PrimaryButton(c.Key("confirm-ok"), d.opts.OK)
					}
					if nav.Item(&d.h[1], btn, ok).Clicked() {
						ok()
					}
				})
			})
		})
	})
}
