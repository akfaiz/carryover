package kit

import (
	"fmt"

	"github.com/egoist/mygo/ui"
)

// menuMinWidth keeps a menu of short labels from being a sliver.
const menuMinWidth = 200

// MenuItem is one action of a MenuButton. Danger draws it red; Icon may be nil.
type MenuItem struct {
	Key              any
	Label            string
	Icon             *ui.SVG
	Danger, Disabled bool
	Action           func()
}

// MenuButton is an icon button, such as the "⋯" of a page or a row, that drops
// down a list of actions: pressing one closes the menu, then runs it. It looks
// and works like SelectOf's list for the mouse, the keyboard and a gamepad (A
// opens it on its first action, the D-pad moves, A runs and B closes; opened
// with the mouse, only the action under the pointer is lit). label names the
// button, for assistive technology, tests and its tooltip; key tells it from
// the other menus of the page, and is also its BtnHandle key.
func MenuButton(c *ui.Context, nav *Navigator, key any, kind BtnKind, icon *ui.SVG, label string, items []MenuItem) ui.Element {
	d := nav.dropdown([2]any{"menu", key})
	toggle := func() { d.toggle(nav) }
	button := IconBtn(c, nav, key, kind, icon, label, toggle)
	closeMenu := func() {
		d.open = false
		BtnHandle(nav, key).Focus(c) // the pad goes on from where it was
	}
	showDropdown(c, nav, fmt.Sprint("menu:", key), button, d, closeMenu, func(panel ui.Element) {
		// Its end lines up with the button's: such a button sits at the end of
		// its row.
		panel.MinWidth(menuMinWidth).AttachTo(button, ui.AnchorBottomEnd, ui.AnchorTopEnd)
	}, func() {
		for _, it := range items {
			menuRow(c, nav, key, it, func() {
				closeMenu()
				it.Action()
			})
		}
	})
	return button
}

// menuRow is one action of a menu: its icon and label.
func menuRow(c *ui.Context, nav *Navigator, menu any, it MenuItem, run func()) {
	t := c.Theme()
	k := [3]any{"menu-item", menu, it.Key}
	h := nav.Handle(k)
	fg, hover := t.Text, t.SurfaceHover
	if it.Danger {
		fg, hover = t.Danger, t.Danger.Alpha(0.15)
	}
	row := dropdownRow(c, h, k, it.Label, hover, func() {
		if it.Icon != nil {
			ui.Icon(c, it.Icon).FontSize(IconSM)
		}
		ui.Text(c, it.Label).SingleLine().Grow(1)
	}).TextColor(fg)
	if it.Disabled {
		row.Disabled(true).Opacity(0.5)
		return
	}
	if nav.Item(h, row, run).Clicked() {
		run()
	}
}
