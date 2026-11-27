package kit

import "github.com/egoist/mygo/ui"

// dropdownMaxHeight is how tall a dropdown's list grows before it scrolls.
const dropdownMaxHeight = 320

// dropdownState is what a select or a menu button keeps between frames.
type dropdownState struct {
	open bool
	// byPointer says the pointer opened the list. It then starts with nothing
	// focused, so only the row under the pointer is lit; opened with a key or
	// the gamepad, it starts with a row focused.
	byPointer bool
	// reveal asks a select's list to scroll to its current choice, which the
	// focus does when the list starts with one.
	reveal bool
}

// toggle opens or closes the list, for its anchor's click or A.
func (d *dropdownState) toggle(nav *Navigator) {
	d.open = !d.open
	d.byPointer = nav.byPointer
	d.reveal = d.open && d.byPointer
}

// showDropdown draws the list a select or a menu button shows below its anchor
// while it is open: a bordered panel of rows, which scrolls when there are
// many. It is a scope of the Navigator named id: the page behind is out of
// reach, and B (like Escape or a press outside) runs closeList. place adjusts
// the panel, such as its width or where it attaches; rows builds the rows (see
// dropdownRow).
func showDropdown(c *ui.Context, nav *Navigator, id string, anchor ui.Element, d *dropdownState, closeList func(), place func(panel ui.Element), rows func()) {
	t := c.Theme()
	nav.Overlay(id, d.open, closeList, func() {
		nav.building.unfocused = d.byPointer
		ui.PopoverBase(c, anchor, &d.open, func(panel ui.Element) {
			panel.Margin(S1, 0, 0, 0).Radius(Radius+2).
				Background(t.Surface).Border(1, t.Border).Shadow(0, 6, 20, 0, ui.RGBA(0, 0, 0, 0.3))
			place(panel)
			ui.Scroll(c).MaxHeight(dropdownMaxHeight).Children(func() {
				// The padding is inside the scroll, which clips what it holds:
				// the focus ring drawn outside a row needs the room.
				ui.Column(c).Gap(2).Padding(S1).Children(rows)
			})
		})
	})
}

// dropdownRow is the shape of one row of a dropdown: content on a face that
// fills with hover under the pointer or the gamepad's cursor. A nil key part is
// not allowed: key tells the row from every other control of the window.
func dropdownRow(c *ui.Context, h *ui.Handle, key any, label string, hover ui.Color, content func()) ui.Element {
	row := ui.ButtonBase(c.Key(key)).Label(label)
	focused, owner := h.Focused(c), row
	return row.Radius(RadiusSM).Padding(S2, S3).Gap(S2).AlignSelf(ui.Stretch).Justify(ui.Start).Draw(func(p *ui.Painter, r ui.Rect) {
		if focused || owner.Hovered() {
			paintFace(p, r, hover, ui.Color{}, 0, RadiusSM)
		}
	}).Children(content)
}
