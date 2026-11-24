package kit

import (
	"image/color"

	"github.com/egoist/mygo/ui"
)

// The reusable pieces of the MyGo pages: text styles, badges, cards. They
// are small building blocks; each draws into the Context it is given and
// returns the element so the caller can chain more styling.

// Muted is secondary text.
func Muted(c *ui.Context, t string) ui.Element {
	return ui.Text(c, t).Wrap().TextColor(c.Theme().TextMuted)
}

// Small is caption-sized text.
func Small(c *ui.Context, t string) ui.Element { return ui.Text(c, t).FontSize(12) }

// Heading is a section heading.
func Heading(c *ui.Context, t string) ui.Element { return ui.Text(c, t).FontSize(15).Bold() }

// Title is a page title.
func Title(c *ui.Context, t string) ui.Element { return ui.Text(c, t).FontSize(20).Bold() }

// Mono is monospaced text (paths, errors, commands), selectable.
func Mono(c *ui.Context, t string) ui.Element {
	return ui.Text(c, t).Font("monospace").FontSize(12).Wrap().Selectable()
}

// Colored is text in one of the theme's colors.
func Colored(c *ui.Context, t string, col ui.Color) ui.Element {
	return ui.Text(c, t).Wrap().TextColor(col)
}

// Danger is error text.
func Danger(c *ui.Context, t string) ui.Element { return Colored(c, t, c.Theme().Danger) }

// kindColor is the color a badge kind is drawn in.
func kindColor(c *ui.Context, kind int) ui.Color {
	switch kind {
	case BadgeOK:
		return col(ColOK)
	case BadgeWarn:
		return col(ColWarn)
	case BadgeErr:
		return col(ColErr)
	case BadgeInfo:
		return col(ColAccent)
	}
	return c.Theme().TextMuted
}

// Badge is a small rounded label, tinted by its kind.
func Badge(c *ui.Context, text string, kind int) ui.Element {
	fg := kindColor(c, kind)
	return ui.Row(c).Padding(2, 8).Radius(9).Background(fg.Alpha(0.15)).Children(func() {
		ui.Text(c, text).FontSize(11).TextColor(fg)
	})
}

// Card is a rounded panel with an optional heading; fn draws its content.
func Card(c *ui.Context, title string, fn func()) ui.Element {
	t := c.Theme()
	return ui.Column(c).Padding(14, 16).Gap(8).Radius(10).Background(t.Surface).Border(1, t.Border).Children(func() {
		if title != "" {
			Heading(c, title)
		}
		fn()
	})
}

// Panel is a group inside a card (one emulator of a game, say): darker than the
// card, so what belongs together reads as one block.
func Panel(c *ui.Context, fn func()) ui.Element {
	t := c.Theme()
	return ui.Column(c).Padding(10, 12).Gap(6).Radius(8).Background(t.Background).Border(1, t.Border).Children(fn)
}

// CardHead is a card heading with right-aligned actions.
func CardHead(c *ui.Context, title string, actions func()) ui.Element {
	return ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
		Heading(c, title).Grow(1)
		if actions != nil {
			actions()
		}
	})
}

// Labeled is a form row: a label over (or beside) the control fn draws.
func Labeled(c *ui.Context, label string, fn func()) ui.Element {
	return ui.Column(c).Gap(4).Children(func() {
		ui.Text(c, label).FontSize(12).TextColor(c.Theme().TextMuted)
		fn()
	})
}

// Tint returns col with the given alpha, as a color.NRGBA for code that still
// holds one (the palette).
func Tint(c color.NRGBA, alpha uint8) color.NRGBA { return color.NRGBA{c.R, c.G, c.B, alpha} }

// PageHead is the line at the top of a page: a back button, the title and the
// page's actions. back is nil on a page with nowhere to go back to.
func PageHead(c *ui.Context, nav *Navigator, backH *ui.Handle, back func(), text string, actions func()) ui.Element {
	return ui.Row(c).Gap(10).Padding(16, 20).AlignItems(ui.Center).Children(func() {
		if back != nil {
			if nav.Item(backH, ui.Button(c.Key("back"), "←"), back).Clicked() {
				back()
			}
		}
		Title(c, text).Grow(1)
		if actions != nil {
			actions()
		}
	})
}

// ProgressLine is a progress bar with a line of text under it: msg, then the
// transferred bytes of the total (an unknown total shows activity only).
func ProgressLine(c *ui.Context, msg string, bytes, total int64) {
	ui.Column(c).Gap(4).Children(func() {
		if total > 0 {
			ui.Progress(c, pct(bytes, total))
			if msg != "" {
				msg += " · "
			}
			msg += FormatBytes(bytes) + " / " + FormatBytes(total)
		} else {
			ui.Progress(c, -1)
		}
		if msg != "" {
			Small(c, msg).TextColor(c.Theme().TextMuted)
		}
	})
}

// BusyLine is a spinner with a line of text: something is working.
func BusyLine(c *ui.Context, text string) ui.Element {
	return ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
		ui.Spinner(c)
		Muted(c, text)
	})
}
