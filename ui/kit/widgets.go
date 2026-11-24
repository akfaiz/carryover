package kit

import "github.com/egoist/mygo/ui"

// The reusable pieces of the MyGo pages: text styles, badges, cards. They
// are small building blocks; each draws into the Context it is given and
// returns the element so the caller can chain more styling.

// Muted is secondary text.
func Muted(c *ui.Context, t string) ui.Element {
	return ui.Text(c, t).Wrap().TextColor(c.Theme().TextMuted)
}

// Small is caption-sized text.
func Small(c *ui.Context, t string) ui.Element {
	return ui.Text(c, t).FontSize(TextSM).TextColor(c.Theme().TextMuted)
}

// Heading is a section heading.
func Heading(c *ui.Context, t string) ui.Element { return ui.Text(c, t).FontSize(TextCard).Bold() }

// Title is a page title.
func Title(c *ui.Context, t string) ui.Element { return ui.Text(c, t).FontSize(TextTitle).Bold() }

// HeroTitle is the big name at the top of a game's page.
func HeroTitle(c *ui.Context, t string) ui.Element { return ui.Text(c, t).FontSize(TextHero).Bold() }

// ItemName is the name of one row in a list: a device, a remote, a game in the
// trash.
func ItemName(c *ui.Context, t string) ui.Element {
	return ui.Text(c, t).FontSize(TextBase).FontWeight(600)
}

// Mono is monospaced text (paths, errors, commands), selectable.
func Mono(c *ui.Context, t string) ui.Element {
	return ui.Text(c, t).Font("monospace").FontSize(TextSM).Wrap().Selectable()
}

// Colored is text in one of the theme's colors.
func Colored(c *ui.Context, t string, col ui.Color) ui.Element {
	return ui.Text(c, t).Wrap().TextColor(col)
}

// IconLine is a line of text with an icon before it, both in col: a status such
// as "In Steam on this device" with a tick.
func IconLine(c *ui.Context, icon *ui.SVG, t string, col ui.Color) ui.Element {
	return ui.Row(c).Gap(S2).AlignItems(ui.Center).Children(func() {
		ui.Icon(c, icon).FontSize(IconMD).TextColor(col)
		ui.Text(c, t).TextColor(col)
	})
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
	return ui.Row(c).Padding(2, S2).Radius(RadiusLG).Background(fg.Alpha(0.15)).Children(func() {
		ui.Text(c, text).FontSize(TextXS).FontWeight(600).TextColor(fg)
	})
}

// Card is a rounded panel with an optional heading; fn draws its content.
func Card(c *ui.Context, title string, fn func()) ui.Element {
	t := c.Theme()
	return ui.Column(c).Padding(S4, 20).Gap(S3).Radius(RadiusLG).Background(t.Surface).Border(1, t.Border).Children(func() {
		if title != "" {
			Heading(c, title)
		}
		fn()
	})
}

// SubCard is a group inside a card (one emulator of a game, one side of a
// comparison): darker than the card, so what belongs together reads as one block.
func SubCard(c *ui.Context, fn func()) ui.Element {
	t := c.Theme()
	return ui.Column(c).Padding(S3, S4).Gap(S2).Radius(Radius).Background(t.Background).Border(1, t.Border).Children(fn)
}

// ListRow is one row of a list inside a card: a name over an optional line of
// details on the left, actions on the right. Rows are told apart by a divider
// above every one but the first (first says which this is).
func ListRow(c *ui.Context, first bool, name string, details func(), actions func()) ui.Element {
	return ui.Column(c).Gap(S2).Children(func() {
		if !first {
			ui.Divider(c)
		}
		ui.Row(c).Gap(S3).AlignItems(ui.Center).Children(func() {
			ui.Column(c).Gap(2).Grow(1).Children(func() {
				ItemName(c, name)
				if details != nil {
					details()
				}
			})
			if actions != nil {
				ui.Row(c).Gap(S2).AlignItems(ui.Center).Children(actions)
			}
		})
	})
}

// Footer is the row of buttons that ends a dialog or a form, right-aligned.
func Footer(c *ui.Context, fn func()) ui.Element {
	return ui.Row(c).Gap(S2).Justify(ui.End).AlignItems(ui.Center).Wrap().Children(fn)
}

// Page is the frame every page draws its content in: a scrolling column with
// the same padding, gap and a width limit. width is PageWide or PageForm; scroll
// is the page's scroll state, which the gamepad's right stick drives.
func Page(c *ui.Context, width float32, scroll *ui.ScrollState, fn func()) ui.Element {
	return pageFrame(c, width, scroll, nil, fn)
}

// ArtPage is Page with artwork (a game's hero image) behind its top, across
// the whole width of the page, however wide the window: the content stays
// within width over it. path "" draws no artwork.
func ArtPage(c *ui.Context, width float32, scroll *ui.ScrollState, path string, version int64, fn func()) ui.Element {
	return pageFrame(c, width, scroll, func() {
		w, _ := c.Size()
		// Hero artwork is about 3:1: a taller band on a wider window shows the
		// same part of the picture.
		ArtBackdrop(c, path, version, min(max(w*0.22, 300), 520))
	}, fn)
}

// pageFrame is the scrolling page: an optional backdrop across its width, and
// the content within width, centered once the window is wider than that.
func pageFrame(c *ui.Context, width float32, scroll *ui.ScrollState, backdrop func(), fn func()) ui.Element {
	return ui.Scroll(c).Grow(1).TrackScroll(scroll).Children(func() {
		ui.Column(c).Children(func() {
			if backdrop != nil {
				backdrop()
			}
			ui.Row(c).Justify(ui.Center).Children(func() {
				ui.Column(c).Grow(1).Shrink(1).Padding(S6, S8).Gap(S4).MaxWidth(width).Children(fn)
			})
		})
	})
}

// RingRoom is how far a focus ring reaches outside its control: the
// Navigator's 2 DIP ring and its antialiased edge, with room to spare.
const RingRoom float32 = 4

// ScrollBox is a scrolling column at most maxHeight tall, its children gap
// apart, for a list or a form inside a dialog or a card. A scroll clips what it
// holds, so the column is padded by RingRoom to keep a focused control's ring
// in sight even at the edge; the box reaches as far out again, so the content
// sits where it would without the padding.
func ScrollBox(c *ui.Context, maxHeight, gap float32, fn func()) ui.Element {
	return ui.Scroll(c).MaxHeight(maxHeight).Margin(-RingRoom).Children(func() {
		ui.Column(c).Gap(gap).Padding(RingRoom).Children(fn)
	})
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
	return ui.Column(c).Gap(S1).Children(func() {
		ui.Text(c, label).FontSize(TextSM).TextColor(c.Theme().TextMuted)
		fn()
	})
}

// PageBackKey is the key of the Back button PageHead draws (see BtnHandle).
const PageBackKey = "page-back"

// PageHead is the top of a page: a "back" button on its own line when the page
// has somewhere to go back to (back is nil otherwise; backLabel is the
// translated word), then the title and the page's actions.
func PageHead(c *ui.Context, nav *Navigator, back func(), backLabel, text string, actions func()) ui.Element {
	return ui.Column(c).Gap(S2).Children(func() {
		if back != nil {
			BackButton(c, nav, PageBackKey, backLabel, back).AlignSelf(ui.Start)
		}
		if text == "" && actions == nil {
			return
		}
		ui.Row(c).Gap(S3).AlignItems(ui.Center).Children(func() {
			Title(c, text).Grow(1)
			if actions != nil {
				actions()
			}
		})
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
