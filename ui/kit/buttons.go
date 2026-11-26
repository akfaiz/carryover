package kit

import "github.com/egoist/mygo/ui"

// BtnKind is how a button looks: Secondary is the default, Primary the one
// action of a view, Danger removes something, Ghost has no face until hovered.
type BtnKind int

// The button kinds.
const (
	BtnSecondary BtnKind = iota
	BtnPrimary
	BtnDanger
	BtnGhost
)

type btnConfig struct {
	busy, disabled bool
	item           []ItemOption
	size           btnSize
	tip            string // shown beside the pointer or the focus, for an icon-only button
}

// btnSize is how big a button is: normal, small (inline actions such as
// "More") or large (the one main action of a page, such as Play).
type btnSize int

const (
	btnNormal btnSize = iota
	btnSmall
	btnLarge
)

// BtnSmall makes a smaller button, for inline actions such as "More".
func BtnSmall() BtnOption { return func(o *btnConfig) { o.size = btnSmall } }

// BtnLarge makes a larger button, for the one main action of a page.
func BtnLarge() BtnOption { return func(o *btnConfig) { o.size = btnLarge } }

// padding is the padding of a button of the size.
func (s btnSize) padding() (v, h float32) {
	switch s {
	case btnSmall:
		return S1, S2
	case btnLarge:
		return S3, S6
	}
	return S2, 14
}

// text is the font size of the label of a button of the size.
func (s btnSize) text() float32 {
	switch s {
	case btnSmall:
		return TextSM
	case btnLarge:
		return TextLG
	}
	return TextBase
}

// icon is the size of the icon of a button of the size.
func (s btnSize) icon() float32 {
	if s == btnLarge {
		return IconMD
	}
	return IconSM
}

// BtnOption changes how Btn and IconBtn behave.
type BtnOption func(*btnConfig)

// BtnBusy shows a spinner in place of the icon and ignores clicks while a
// task the button started runs.
func BtnBusy(busy bool) BtnOption { return func(o *btnConfig) { o.busy = busy } }

// BtnDisabled dims the button and ignores clicks.
func BtnDisabled(disabled bool) BtnOption { return func(o *btnConfig) { o.disabled = disabled } }

// BtnItem passes options to the Navigator (FocusSafe for a dialog's start).
func BtnItem(opts ...ItemOption) BtnOption {
	return func(o *btnConfig) { o.item = append(o.item, opts...) }
}

// BtnHandle is the Navigator handle of the Btn or IconBtn drawn with key, for
// a page's DefaultFocus.
func BtnHandle(nav *Navigator, key any) *ui.Handle { return nav.Handle([2]any{"btn", key}) }

func btnConfigOf(opts []BtnOption) btnConfig {
	var cfg btnConfig
	for _, o := range opts {
		o(&cfg)
	}
	return cfg
}

// btnFace is the background and text color of a button of this kind.
func btnFace(c *ui.Context, kind BtnKind, hovered, pressed bool) (bg, fg ui.Color) {
	t := c.Theme()
	switch kind {
	case BtnPrimary:
		bg, fg = t.Accent, t.AccentText
		switch {
		case pressed:
			bg = bg.Alpha(0.75)
		case hovered:
			bg = bg.Alpha(0.9)
		}
	case BtnDanger:
		fg = t.Danger
		bg = fg.Alpha(0.15)
		switch {
		case pressed:
			bg = fg.Alpha(0.35)
		case hovered:
			bg = fg.Alpha(0.25)
		}
	case BtnGhost:
		fg = t.Text
		switch {
		case pressed:
			bg = t.SurfacePressed
		case hovered:
			bg = t.SurfaceHover
		}
	default:
		fg, bg = t.Text, t.SurfaceHover
		switch {
		case pressed:
			bg = t.SurfacePressed
		case hovered:
			bg = col(ColBg4)
		}
	}
	return bg, fg
}

// button draws and registers the button; body draws the content.
func button(c *ui.Context, nav *Navigator, key any, kind BtnKind, label string, cfg btnConfig, action func(), body func(fg ui.Color)) ui.Element {
	k := [2]any{"btn", key}
	b := ui.ButtonBase(c.Key(k)).Label(label)
	if cfg.tip != "" {
		b = b.Tooltip(cfg.tip)
	}
	// Pressed is read as the view builds (a press is rare), Hovered only as the
	// button paints: read while building, the pointer crossing any button would
	// build the whole page anew, which costs a page of hundreds of Play buttons
	// a frame of milliseconds. The face is painted over no background, in either
	// look, so a translucent one does not blend with another.
	pressed := b.Pressed()
	bg, fg := btnFace(c, kind, false, pressed)
	hoverBg, _ := btnFace(c, kind, true, pressed)
	pv, ph := cfg.size.padding()
	b = b.TextColor(fg).Radius(Radius).Gap(S2).Padding(pv, ph).Draw(func(p *ui.Painter, r ui.Rect) {
		face := bg
		if b.Hovered() {
			face = hoverBg
		}
		if face.A != 0 {
			p.Fill(r, face, Radius)
		}
	}).Children(func() { body(fg) })
	if cfg.disabled || cfg.busy {
		return b.Disabled(true)
	}
	if nav.Item(nav.Handle(k), b, action, cfg.item...).Clicked() {
		action()
	}
	return b
}

func btnIcon(c *ui.Context, icon *ui.SVG, busy bool, size float32) {
	switch {
	case busy:
		ui.Spinner(c)
	case icon != nil:
		ui.Icon(c, icon).FontSize(size)
	}
}

// Btn is a button with an optional icon before its label. It is the one way
// pages draw a button: it registers with the Navigator (so A runs it) and
// calls action on a click. key tells it from the other buttons on the page.
func Btn(c *ui.Context, nav *Navigator, key any, kind BtnKind, icon *ui.SVG, label string, action func(), opts ...BtnOption) ui.Element {
	cfg := btnConfigOf(opts)
	b := button(c, nav, key, kind, label, cfg, action, func(ui.Color) {
		btnIcon(c, icon, cfg.busy, cfg.size.icon())
		ui.Text(c, label).SingleLine().FontWeight(600).FontSize(cfg.size.text())
	})
	if cfg.size == btnNormal {
		b = b.MinHeight(ControlHeight)
	}
	return b
}

// IconBtn is a square button with only an icon; label names it for assistive
// technology and tests, and shows as its tooltip.
func IconBtn(c *ui.Context, nav *Navigator, key any, kind BtnKind, icon *ui.SVG, label string, action func(), opts ...BtnOption) ui.Element {
	cfg := btnConfigOf(opts)
	cfg.tip = label
	return button(c, nav, key, kind, label, cfg, action, func(ui.Color) {
		btnIcon(c, icon, cfg.busy, cfg.size.icon())
	}).Width(36).Height(36).Padding(0)
}

// BackButton is the ghost "← Back" button at the top of a page that has
// somewhere to go back to. label is the translated word.
func BackButton(c *ui.Context, nav *Navigator, key any, label string, action func()) ui.Element {
	return Btn(c, nav, key, BtnGhost, IconArrowLeft, label, action)
}

// PathButton shows a folder path on one line, cut at the end, in a button with
// a folder icon; action opens it. It takes the width left in its row.
func PathButton(c *ui.Context, nav *Navigator, key any, path string, action func()) ui.Element {
	return button(c, nav, key, BtnSecondary, path, btnConfig{}, action, func(ui.Color) {
		ui.Icon(c, IconFolderOpen).FontSize(IconSM)
		ui.Text(c, path).SingleLine().Ellipsis("…").Font("monospace").FontSize(TextSM).Grow(1)
	}).Grow(1).Justify(ui.Start)
}

// Toggle is a switch with its label beside it: the way a page turns a setting
// on or off. A on it (the gamepad) flips it too. key tells it from the others
// on the page; a disabled one dims and ignores input.
func Toggle(c *ui.Context, nav *Navigator, key any, label string, on *bool, disabled bool) ui.Element {
	k := [2]any{"toggle", key}
	flip := func() {
		if !disabled {
			*on = !*on
		}
	}
	return ui.Row(c).Gap(S3).AlignItems(ui.Center).Children(func() {
		sw := ui.Switch(c.Key(k), on).Label(label).Disabled(disabled)
		nav.Item(nav.Handle(k), sw, flip) // a click flips *on itself; A runs flip
		ui.Text(c, label).Wrap().Grow(1)
	})
}

// ToggleRow is a setting with a switch: its title over a short description on
// the left, the switch on the right. A on it (the gamepad) flips it. key tells
// it from the others on the page; a disabled one dims and ignores input.
func ToggleRow(c *ui.Context, nav *Navigator, key any, title, desc string, on *bool, disabled bool) ui.Element {
	k := [2]any{"toggle", key}
	flip := func() {
		if !disabled {
			*on = !*on
		}
	}
	t := c.Theme()
	row := ui.Row(c).Gap(S4).AlignItems(ui.Center).Children(func() {
		ui.Column(c).Grow(1).Shrink(1).Gap(2).Children(func() {
			ui.Text(c, title).FontWeight(600).Wrap()
			if desc != "" {
				ui.Text(c, desc).FontSize(TextSM).TextColor(t.TextMuted).Wrap()
			}
		})
		sw := ui.Switch(c.Key(k), on).Label(title).Disabled(disabled)
		nav.Item(nav.Handle(k), sw, flip) // a click flips *on itself; A runs flip
	})
	if disabled {
		row.Opacity(0.55)
	}
	return row
}
