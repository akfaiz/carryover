package kit

import "github.com/egoist/mygo/ui"

// ChoiceTile is a square button for one way to play a game: an icon over its
// name, with a tinted face for the highlighted one. The highlight is not an
// outline: an accent ring means focus only, and next to a second one it looks
// like the focus never left the highlighted tile. key tells it from the others
// on the page.
func ChoiceTile(c *ui.Context, nav *Navigator, key any, name string, icon *ui.SVG, highlighted bool, tapped func(), opts ...ItemOption) ui.Element {
	t := c.Theme()
	k := [2]any{"tile", key}
	b := ui.ButtonBase(c.Key(k))
	face, hoverFace := t.Surface, t.SurfaceHover
	if highlighted {
		face, hoverFace = t.Accent.Alpha(0.18), t.Accent.Alpha(0.28)
	}
	owner := b
	b = b.Label(name).Width(ChoiceTileSize).Height(ChoiceTileSize).Radius(10).Border(1, ui.Color{}).
		Draw(func(p *ui.Painter, r ui.Rect) {
			paintFace(p, r, HoverColor(owner, face, hoverFace), HoverColor(owner, t.Border, t.TextMuted), 1, 10)
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
