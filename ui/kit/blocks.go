package kit

import "github.com/egoist/mygo/ui"

// Callout is a message that stands out from the page: a rounded row tinted by
// kind (a Badge* kind), with an icon, the text and, when actions is not nil,
// buttons on the right. Use it for what the user should notice: a setup that
// is missing, an error, a change waiting to sync.
func Callout(c *ui.Context, kind int, icon *ui.SVG, text string, actions func()) ui.Element {
	fg := kindColor(c, kind)
	return ui.Row(c).Gap(S3).AlignItems(ui.Center).Padding(S2+2, S3).Radius(Radius).
		Background(fg.Alpha(0.10)).Border(1, fg.Alpha(0.30)).Children(func() {
		if icon != nil {
			ui.Icon(c, icon).FontSize(IconMD).TextColor(fg)
		}
		ui.Text(c, text).Wrap().Grow(1).Shrink(1)
		if actions != nil {
			ui.Row(c).Gap(S2).AlignItems(ui.Center).Children(actions)
		}
	})
}

// EmptyState is what a page or list shows while it has nothing: an icon in a
// tinted circle, a title, a line of text and, when actions is not nil, the
// buttons that fill it.
func EmptyState(c *ui.Context, icon *ui.SVG, title, text string, actions func()) ui.Element {
	t := c.Theme()
	return ui.Column(c).AlignItems(ui.Center).Gap(S3).Padding(S8, S6).Children(func() {
		ui.Row(c).Width(56).Height(56).Radius(28).Background(t.Accent.Alpha(0.14)).Center().Children(func() {
			ui.Icon(c, icon).FontSize(IconSM + 8).TextColor(t.Accent)
		})
		if title != "" {
			ui.Text(c, title).FontSize(TextLG).FontWeight(600).TextAlign(ui.Center).Wrap()
		}
		if text != "" {
			ui.Text(c, text).TextColor(t.TextMuted).TextAlign(ui.Center).Wrap().MaxWidth(520)
		}
		if actions != nil {
			ui.Row(c).Gap(S2).Wrap().Justify(ui.Center).Padding(S1, 0, 0, 0).Children(actions)
		}
	})
}

// ArtBackdrop draws artwork (a game's hero image) behind the element it is
// declared in, at its top, height DIPs tall: faint, and fading into the page's
// background at its bottom so text over it stays readable. Nothing shows while
// the image loads or when path is "".
func ArtBackdrop(c *ui.Context, path string, version int64, height float32) {
	if path == "" {
		return
	}
	bm := Images.File(path, version, 960)
	if bm == nil {
		return
	}
	bg := c.Theme().Background
	ui.Image(c, bm).Absolute().Top(0).Left(0).Right(0).Height(height).Fit(ui.Cover).Opacity(0.35).ZIndex(-1)
	ui.Box(c).Absolute().Top(0).Left(0).Right(0).Height(height).ZIndex(-1).
		LinearGradient(ui.LinearGradient{Angle: 180, Stops: []ui.GradientStop{
			{Color: bg.Alpha(0.1), At: 0}, {Color: bg.Alpha(0.6), At: 0.55}, {Color: bg, At: 1},
		}})
}
