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
	bg, fg := ui.Color{}, t.TextMuted
	switch {
	case active:
		bg, fg = t.SurfaceHover, t.Text
	case b.Pressed():
		bg, fg = t.SurfacePressed, t.Text
	case b.Hovered():
		bg, fg = t.SurfaceHover.Alpha(0.6), t.Text
	}
	iconColor := fg
	if active {
		iconColor = t.Accent
	}
	if len(tone) > 0 {
		fg, iconColor = tone[0], tone[0]
	}
	b = b.Background(bg).TextColor(fg).Radius(Radius).Padding(10, S3).Gap(10).AlignSelf(ui.Stretch).Justify(ui.Start).Children(func() {
		ui.Icon(c, icon).FontSize(IconMD).TextColor(iconColor)
		ui.Text(c, label).SingleLine().FontWeight(600).Grow(1)
		if count > 0 {
			Badge(c, strconv.Itoa(count), BadgeNone)
		}
	})
	if nav.Item(h, b, action).Clicked() {
		action()
	}
	return b
}
