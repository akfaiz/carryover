package kit

import (
	"strconv"

	"github.com/egoist/mygo/ui"
)

// NavItem is one entry of the sidebar: an icon and a label, left-aligned and
// as wide as the sidebar, filled when it is the page that is open. count > 0
// shows a small number on the right. h is the item's Navigator handle; tone,
// when given, colors the icon and label (a warning, say).
func NavItem(c *ui.Context, nav *Navigator, h *ui.Handle, key any, icon *ui.SVG, label string, active bool, count int, action func(), tone ...ui.Color) ui.Element {
	t := c.Theme()
	k := [2]any{"nav", key}
	b := ui.ButtonBase(c.Key(k)).Label(label)
	// The sidebar is in the same view as the page: reading the hover here, as the
	// view builds, would build the page anew whenever the pointer crossed an
	// entry. So the look of the pointer's being over it is worked out apart, and
	// painted as the entry paints.
	pressed := b.Pressed()
	look := func(hovered bool) (bg, fg, iconColor ui.Color) {
		bg, fg = ui.Color{}, t.TextMuted
		switch {
		case active:
			bg, fg = t.SurfaceHover, t.Text
		case pressed:
			bg, fg = t.SurfacePressed, t.Text
		case hovered:
			bg, fg = t.SurfaceHover.Alpha(0.6), t.Text
		}
		iconColor = fg
		if active {
			iconColor = t.Accent
		}
		if len(tone) > 0 {
			fg, iconColor = tone[0], tone[0]
		}
		return bg, fg, iconColor
	}
	bg0, fg0, ic0 := look(false)
	bg1, fg1, ic1 := look(true)
	owner := b
	b = b.TextColor(fg0).Radius(Radius).Padding(10, S3).Gap(10).AlignSelf(ui.Stretch).Justify(ui.Start).Draw(func(p *ui.Painter, r ui.Rect) {
		paintFace(p, r, HoverColor(owner, bg0, bg1), ui.Color{}, 0, Radius)
	}).Children(func() {
		HoverIcon(c, owner, icon, IconMD, ic0, ic1)
		HoverText(c, owner, label, 0, 600, fg0, fg1).Grow(1)
		if count > 0 {
			Badge(c, strconv.Itoa(count), BadgeNone)
		}
	})
	if nav.Item(h, b, action).Clicked() {
		action()
	}
	return b
}
