package kit

import "github.com/egoist/mygo/ui"

// ChoiceTile is a square button for one way to play a game: an icon over its
// name, with a ring around the highlighted one. key tells it from the others on
// the page.
func ChoiceTile(c *ui.Context, nav *Navigator, key any, name string, icon *ui.SVG, highlighted bool, tapped func(), opts ...ItemOption) ui.Element {
	t := c.Theme()
	k := [2]any{"tile", key}
	b := ui.ButtonBase(c.Key(k))
	border, hoverBorder, width := t.Border, t.TextMuted, float32(1)
	if highlighted {
		border, hoverBorder, width = t.Accent, t.Accent, 2
	}
	owner := b
	b = b.Label(name).Width(ChoiceTileSize).Height(ChoiceTileSize).Radius(10).Border(width, ui.Color{}).
		Draw(func(p *ui.Painter, r ui.Rect) {
			paintFace(p, r, HoverColor(owner, t.Surface, t.SurfaceHover), HoverColor(owner, border, hoverBorder), width, 10)
		}).Children(func() {
		ui.Column(c).Fill().Center().Gap(8).Padding(8).Children(func() {
			ui.Icon(c, icon).FontSize(30).TextColor(t.Text)
			ui.Text(c, name).Wrap().TextAlign(ui.Center).FontSize(13)
		})
	})
	if nav.Item(nav.Handle(k), b, tapped, opts...).Clicked() {
		tapped()
	}
	return b
}
