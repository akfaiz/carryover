package kit

import "github.com/egoist/mygo/ui"

// Hover without a rebuild. Reading Element.Hovered while the view builds makes
// the pointer crossing that element build the whole window's view anew: every
// element of the page and the sidebar, however few changed, and for a page of
// hundreds of tiles that is milliseconds a frame. Read in a Draw callback it
// only paints the last frame's elements again. So a control whose look follows
// its hover paints that look itself: its face (fill and outline) in its own
// Draw, and what it holds that changes color (text, icons) with HoverText and
// HoverIcon below, which keep the real element for layout and accessibility
// and paint over it.

// paintFace paints the fill and the outline (width DIPs, inside the edge) of a
// control, in place of the Background and Border it would have been given. The
// element keeps a Border of width and no color, so the room stays the same.
func paintFace(p *ui.Painter, r ui.Rect, bg, border ui.Color, width, radius float32) {
	if bg.A != 0 {
		p.Fill(r, bg, radius)
	}
	if width > 0 && border.A != 0 {
		p.Stroke(r, border, radius, width)
	}
}

// HoverColor is hover while the pointer is over owner, normal otherwise. Call it
// only as the element paints.
func HoverColor(owner ui.Element, normal, hover ui.Color) ui.Color {
	if owner.Hovered() {
		return hover
	}
	return normal
}

// HoverText is a line of text in the color normal, or hover while the pointer
// is over owner (a button it is in): an ui.Text(...).SingleLine() with the
// weight (0 for the theme's) and size (0 for the theme's), which the caller
// can go on styling (Grow, Shrink). The text is painted as the element paints,
// so the hover is not read as the view builds. Equal colors need no painting,
// and give the plain element.
func HoverText(c *ui.Context, owner ui.Element, s string, size float32, weight int, normal, hover ui.Color) ui.Element {
	el := ui.Text(c, s).SingleLine()
	if size > 0 {
		el = el.FontSize(size)
	}
	if weight > 0 {
		el = el.FontWeight(weight)
	}
	if normal == hover {
		return el.TextColor(normal)
	}
	return el.TextColor(ui.Color{}).Draw(func(p *ui.Painter, r ui.Rect) {
		span := ui.Span{Text: s, Size: size, Weight: weight, Color: HoverColor(owner, normal, hover)}
		span.Text = fitText(p, span, r.W)
		p.RichText(r.X, r.Y, 0, span)
	})
}

// fitText cuts s with an ellipsis until it takes no more than width DIPs, as
// ui.Text's SingleLine does.
func fitText(p *ui.Painter, span ui.Span, width float32) string {
	if w, _ := p.MeasureText(0, span); w <= width {
		return span.Text
	}
	runes := []rune(span.Text)
	for n := len(runes) - 1; n > 0; n-- {
		span.Text = string(runes[:n]) + "…"
		if w, _ := p.MeasureText(0, span); w <= width {
			return span.Text
		}
	}
	return "…"
}

// HoverIcon is an icon of size DIPs in the color normal, or hover while the
// pointer is over owner (see HoverText).
func HoverIcon(c *ui.Context, owner ui.Element, icon *ui.SVG, size float32, normal, hover ui.Color) ui.Element {
	el := ui.Icon(c, icon).FontSize(size)
	if normal == hover {
		return el.TextColor(normal)
	}
	return el.TextColor(ui.Color{}).Draw(func(p *ui.Painter, r ui.Rect) {
		p.Icon(icon, r, HoverColor(owner, normal, hover))
	})
}
