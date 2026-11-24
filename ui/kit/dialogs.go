package kit

import (
	"slices"
	"strconv"

	"github.com/egoist/mygo/ui"
)

// Dialog is a modal dialog a window shows over its page. Body declares its
// content each frame and registers its controls with the Navigator it is given
// (so the gamepad reaches them); Back is what B, Escape or a click outside do:
// closing it, unless it says otherwise (a prompt gives the safe answer).
type Dialog struct {
	Title string
	Width float32 // 0: 480
	Body  func(c *ui.Context, nav *Navigator)
	Back  func()
	// Wide gives the dialog the window's width, up to 900: lists and forms.
	Wide bool

	host *Dialogs
}

// Dialog widths: the default, a Wide one's, and the room kept between a
// dialog's content and each side of a window too narrow for it (MyGo's modal
// keeps 20 DIPs from the window's edge and pads its panel by 20 more).
const (
	dialogWidth     float32 = 480
	dialogWideWidth float32 = 860
	dialogMargin    float32 = 40
)

// width is how wide the dialog is in a window windowW wide: what it asks for,
// at most the window less a margin on each side.
func (d *Dialog) width(windowW float32) float32 {
	w := d.Width
	if w == 0 {
		w = dialogWidth
	}
	if d.Wide {
		w = dialogWideWidth
	}
	return max(min(w, windowW-2*dialogMargin), 0)
}

// ContentWidth is the width the dialog's Body has in this window, for content
// laid out by hand to fit it (ChoiceGrid).
func (d *Dialog) ContentWidth(c *ui.Context) float32 {
	w, _ := c.Size()
	return max(d.width(w)-2*S6, ChoiceTileSize)
}

// Close takes the dialog down.
func (d *Dialog) Close() {
	if d.host != nil {
		d.host.Close(d)
	}
}

// Dialogs is the stack of dialogs a window shows; the last is on top and the
// only one the gamepad reaches. Use it on the UI goroutine.
type Dialogs struct {
	stack []*Dialog
	seq   int
	ids   map[*Dialog]string
}

// Show puts a dialog on top.
func (h *Dialogs) Show(d *Dialog) *Dialog {
	if h.ids == nil {
		h.ids = map[*Dialog]string{}
	}
	h.seq++
	d.host = h
	h.ids[d] = "dialog-" + strconv.Itoa(h.seq)
	h.stack = append(h.stack, d)
	Redraw()
	return d
}

// Close removes a dialog (and nothing else) from the stack.
func (h *Dialogs) Close(d *Dialog) {
	if i := slices.Index(h.stack, d); i >= 0 {
		h.stack = slices.Delete(h.stack, i, i+1)
		delete(h.ids, d)
		d.host = nil
		Redraw()
	}
}

// Open reports whether any dialog shows.
func (h *Dialogs) Open() bool { return len(h.stack) > 0 }

// Top is the dialog on top, or nil.
func (h *Dialogs) Top() *Dialog {
	if len(h.stack) == 0 {
		return nil
	}
	return h.stack[len(h.stack)-1]
}

// View declares every dialog, in order, as a scope of nav.
func (h *Dialogs) View(c *ui.Context, nav *Navigator) {
	for _, d := range slices.Clone(h.stack) {
		back := d.Back
		if back == nil {
			back = d.Close
		}
		nav.Overlay(h.ids[d], true, back, func() {
			open := true
			m := ui.Modal(c, &open, func() {
				ww, _ := c.Size()
				w := d.width(ww) // never wider than the window
				ui.Column(c).Width(w).MaxWidth(w).Padding(S6).Gap(S3).Children(func() {
					if d.Title != "" {
						Heading(c, d.Title).Wrap()
					}
					d.Body(c, nav)
				})
			})
			if m.Dismissed() {
				back()
			}
		})
	}
}
