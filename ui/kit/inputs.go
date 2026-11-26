package kit

import (
	"fmt"

	"github.com/egoist/mygo/ui"
)

// SelectOf is a choice of one value from a list, drawn as a field showing the
// current choice with a list of the options below it. It is the same for the
// mouse, the keyboard and a gamepad: A on the field opens the list (which
// starts on the current choice), the D-pad moves, A picks and B closes it.
// title names the field for assistive technology; key tells it from the other
// selects on the page.
func SelectOf(c *ui.Context, nav *Navigator, key any, title string, ch *Choice, v string, onChange func(string)) ui.Element {
	k := [2]any{"select", key}
	t := c.Theme()
	label := ch.Label(v)
	if label == "" && len(ch.Labels) > 0 {
		label = ch.Labels[0]
	}
	open := nav.popup(k)
	toggle := func() { *open = !*open }

	field := ui.ButtonBase(c.Key(k)).Label(title)
	border, hoverBorder := t.Border, t.TextMuted
	if *open {
		border, hoverBorder = t.Accent, t.Accent
	}
	owner := field
	field = field.Border(1, ui.Color{}).Radius(Radius).Padding(0, S3).Gap(S2).Height(ControlHeight).MinWidth(160).Draw(func(p *ui.Painter, r ui.Rect) {
		paintFace(p, r, t.Background, HoverColor(owner, border, hoverBorder), 1, Radius)
	}).Children(func() {
		ui.Text(c, label).SingleLine().Grow(1)
		ui.Icon(c, IconChevronDown).FontSize(IconSM).TextColor(t.TextMuted)
	})
	if nav.Item(nav.Handle(k), field, toggle).Clicked() {
		toggle()
	}

	close := func() { *open = false }
	nav.Overlay(fmt.Sprint("select:", key), *open, close, func() {
		ui.PopoverBase(c, field, open, func(panel ui.Element) {
			panel.MinWidth(field.Bounds().W).Margin(S1, 0, 0, 0).Radius(Radius+2).
				Background(t.Surface).Border(1, t.Border).Shadow(0, 6, 20, 0, ui.RGBA(0, 0, 0, 0.3))
			ui.Scroll(c).MaxHeight(320).Children(func() {
				// The padding is inside the scroll, which clips what it holds:
				// the focus ring drawn outside a row needs the room.
				ui.Column(c).Gap(2).Padding(S1).Children(func() {
					for i, val := range ch.Values {
						selectOption(c, nav, key, i, ch.Labels[i], val == v, func() {
							close()
							if val != v && onChange != nil {
								onChange(val)
							}
						})
					}
				})
			})
		})
	})
	return field
}

// selectOption is one row of a select's list: the label, a tick on the
// current choice, a fill under the pointer or the gamepad's cursor.
func selectOption(c *ui.Context, nav *Navigator, key any, i int, label string, current bool, pick func()) {
	t := c.Theme()
	k := [3]any{"select-opt", key, i}
	h := nav.Handle(k)
	row := ui.ButtonBase(c.Key(k)).Label(label)
	focused, owner := h.Focused(c), row
	row = row.Radius(RadiusSM).Padding(S2, S3).Gap(S2).AlignSelf(ui.Stretch).Justify(ui.Start).Draw(func(p *ui.Painter, r ui.Rect) {
		if focused || owner.Hovered() {
			paintFace(p, r, t.SurfaceHover, ui.Color{}, 0, RadiusSM)
		}
	}).Children(func() {
		ui.Text(c, label).SingleLine().Grow(1)
		if current {
			ui.Icon(c, IconCheck).FontSize(IconSM).TextColor(t.Accent)
		}
	})
	var opts []ItemOption
	if current {
		opts = append(opts, FocusSafe) // the pad starts on the current choice
	}
	if nav.Item(h, row, pick, opts...).Clicked() {
		pick()
	}
}

// Field is a one-line text input bound to v, registered with the Navigator
// under key as a text field (A opens Steam's keyboard), so nav.Handle(key)
// focuses it. placeholder shows while it is empty and names it for assistive
// technology. Chain more styling on the result (Font, Password, Grow, Width).
func Field(c *ui.Context, nav *Navigator, key any, v *string, placeholder string) ui.Element {
	in := ui.TextInput(c.Key(key), v).Placeholder(placeholder).Label(placeholder).Height(ControlHeight)
	return nav.Item(nav.Handle(key), in, nil, TextField)
}

// TextArea is Field for text of several lines, in a monospaced font, height
// DIPs tall.
func TextArea(c *ui.Context, nav *Navigator, key any, v *string, placeholder string, height float32) ui.Element {
	in := ui.TextArea(c.Key(key), v).Placeholder(placeholder).Label(placeholder).Font("monospace").Height(height)
	return nav.Item(nav.Handle(key), in, nil, TextField)
}

// ChipOption is one button of Chips; Icon may be nil. Warn draws it, while not
// selected, in the warning color: a filter that needs a look.
type ChipOption struct {
	Value, Label string
	Icon         *ui.SVG
	Warn         bool
}

// Chips is a row of pill buttons of which one is selected, such as the filters
// above the library grid. The selected pill is tinted in the accent color, the
// others are plain.
func Chips(c *ui.Context, nav *Navigator, key any, options []ChipOption, selected string, onSelect func(string)) ui.Element {
	return ui.Row(c).Gap(S2).Wrap().Children(func() {
		for _, o := range options {
			k := [3]any{"chip", key, o.Value}
			t := c.Theme()
			b := ui.ButtonBase(c.Key(k)).Label(o.Label)
			on := o.Value == selected
			// The look under the pointer is painted as the pill paints, not chosen
			// as the view builds (see hoverpaint.go).
			look := func(hovered bool) (bg, fg ui.Color) {
				bg, fg = t.SurfaceHover, t.TextMuted
				switch {
				case on:
					bg, fg = t.Accent.Alpha(0.18), t.Accent
				case o.Warn && hovered:
					bg, fg = t.Warning.Alpha(0.24), t.Warning
				case o.Warn:
					bg, fg = t.Warning.Alpha(0.14), t.Warning
				case hovered:
					bg, fg = col(ColBg4), t.Text
				}
				return bg, fg
			}
			bg0, fg0 := look(false)
			bg1, fg1 := look(true)
			owner := b
			b = b.TextColor(fg0).Radius(18).Gap(6).Padding(6, S3).Draw(func(p *ui.Painter, r ui.Rect) {
				paintFace(p, r, HoverColor(owner, bg0, bg1), ui.Color{}, 0, 18)
			}).Children(func() {
				if o.Icon != nil {
					HoverIcon(c, owner, o.Icon, IconSM, fg0, fg1)
				}
				HoverText(c, owner, o.Label, 0, 600, fg0, fg1)
			})
			pick := func() { onSelect(o.Value) }
			if nav.Item(nav.Handle(k), b, pick).Clicked() {
				pick()
			}
		}
	})
}

// CoverView shows a cover image with a letter placeholder, w by h DIPs. While
// the image loads, and when there is none, the name's initial shows.
func CoverView(c *ui.Context, src CoverSource, w, h float32) ui.Element {
	t := c.Theme()
	var bm *ui.Bitmap
	switch {
	case src.Path != "":
		bm = Images.File(src.Path, src.Version, int(2*h))
	case src.URL != "":
		bm = Images.URL(src.URL, int(2*h))
	}
	box := ui.Box(c).Width(w).Height(h).Radius(8).Background(col(ColBg3)).Border(1, t.Border).Clip()
	return box.Children(func() {
		if bm == nil {
			if src.Name != "" {
				ui.Column(c).Absolute().Top(0).Bottom(0).Left(0).Right(0).Center().Children(func() {
					ui.Text(c, initial(src.Name)).FontSize(h / 5).Bold().TextColor(t.TextMuted)
				})
			}
			return
		}
		fit := ui.Cover
		if src.Contain {
			fit = ui.Contain
		}
		ui.Image(c, bm).Width(w).Height(h).Fit(fit)
	})
}
