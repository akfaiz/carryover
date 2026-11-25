package kit

import "github.com/egoist/mygo/ui"

// ListButton is a row of a list that runs action when it is clicked or
// picked with the gamepad: as wide as the list, with a fill under the pointer.
// fn draws what the row shows, label names it for assistive technology, and
// key tells it from the other rows (nav.Handle(key) focuses it).
func ListButton(c *ui.Context, nav *Navigator, key any, label string, action func(), fn func()) ui.Element {
	b := ui.ButtonBase(c.Key(key)).Label(label)
	owner, hover := b, c.Theme().SurfaceHover
	b = b.Radius(Radius).AlignSelf(ui.Stretch).Draw(func(p *ui.Painter, r ui.Rect) {
		paintFace(p, r, HoverColor(owner, ui.Color{}, hover), ui.Color{}, 0, Radius)
	}).Children(fn)
	if nav.Item(nav.Handle(key), b, action).Clicked() {
		action()
	}
	return b
}

// Disclosure is a heading that shows and hides what fn builds below it, as
// "Advanced" over the settings few need: an arrow beside label, a fill under
// the pointer, and A on it (the gamepad) opens or closes it as a click does.
// *open says whether it is open; key tells it from the others on the page.
func Disclosure(c *ui.Context, nav *Navigator, key any, label string, open *bool, fn func()) ui.Element {
	k := [2]any{"disclosure", key}
	t := c.Theme()
	return ui.Column(c).Shrink(0).Children(func() {
		p := ui.CollapsibleBase(c.Key(k), open)
		tr := p.Trigger.Label(label).AlignSelf(ui.Start).AlignItems(ui.Center).Gap(S1).
			Padding(S1, S2, S1, 2).Radius(Radius)
		owner := tr
		tr = tr.Draw(func(p *ui.Painter, r ui.Rect) {
			paintFace(p, r, HoverColor(owner, ui.Color{}, t.SurfaceHover), ui.Color{}, 0, Radius)
		})
		icon := IconChevronRight
		if *open {
			icon = IconChevronDown
		}
		tr.Children(func() {
			ui.Icon(c, icon).FontSize(IconSM).TextColor(t.TextMuted)
			ui.Text(c, label).SingleLine()
		})
		nav.Item(nav.Handle(k), tr, func() { *open = !*open })
		// Below the label, lined up with it. Panel is empty while closed.
		p.Panel(fn).Padding(S1, 0, S1, 22).Gap(S2)
	})
}
