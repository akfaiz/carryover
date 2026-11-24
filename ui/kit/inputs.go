package kit

import "github.com/egoist/mygo/ui"

// SelectOf is a choice of one value from a list. It is MyGo's own Select (a
// drop-down) for the mouse and keyboard. While a gamepad is connected it is a
// button showing the current choice that opens a dialog of the options
// instead, since the drop-down can't be opened with A and the pad reaches a
// dialog's buttons like any other. title names the dialog; key tells it from
// the other selects on the page.
func SelectOf(c *ui.Context, nav *Navigator, dlg *Dialogs, key any, title string, ch *Choice, v string, onChange func(string)) ui.Element {
	k := [2]any{"select", key}
	label := ch.Label(v)
	if label == "" && len(ch.Labels) > 0 {
		label = ch.Labels[0]
	}
	if !nav.PadPresent() {
		return dropDown(c, nav, k, ch, label, onChange)
	}
	open := func() {
		var d *Dialog
		d = dlg.Show(&Dialog{Title: title, Width: 360, Body: func(c *ui.Context, nav *Navigator) {
			for i, val := range ch.Values {
				ok := [3]any{"select-opt", key, i}
				var b ui.Element
				if val == v {
					b = ui.PrimaryButton(c.Key(ok), ch.Labels[i])
				} else {
					b = ui.Button(c.Key(ok), ch.Labels[i])
				}
				pick := func() {
					d.Close()
					if val != v && onChange != nil {
						onChange(val)
					}
				}
				var opts []ItemOption
				if val == v {
					opts = append(opts, FocusSafe) // starts on the current choice
				}
				if nav.Item(nav.Handle(ok), b.AlignSelf(ui.Stretch), pick, opts...).Clicked() {
					pick()
				}
			}
		}})
	}
	b := ui.Button(c.Key(k), label+"  ▾")
	if nav.Item(nav.Handle(k), b, open).Clicked() {
		open()
	}
	return b
}

// dropDown is SelectOf's drop-down: ui.Select keeps the label chosen in a
// string the Navigator holds under k, set from the model each frame.
func dropDown(c *ui.Context, nav *Navigator, k [2]any, ch *Choice, label string, onChange func(string)) ui.Element {
	chosen := nav.selected(k)
	*chosen = label
	sel := nav.Item(nav.Handle(k), ui.Select(c.Key(k), chosen, ch.Labels), nil)
	if sel.Changed() && onChange != nil {
		onChange(ch.Value(*chosen))
	}
	return sel
}

// ChipOption is one button of Chips; Icon may be nil.
type ChipOption struct {
	Value, Label string
	Icon         *ui.SVG
}

// Chips is a row of toggle buttons of which one is selected, such as the
// filters above the library grid. The others are plain, so they read as
// choices rather than as text.
func Chips(c *ui.Context, nav *Navigator, key any, options []ChipOption, selected string, onSelect func(string)) ui.Element {
	return ui.Row(c).Gap(8).Wrap().Children(func() {
		for _, o := range options {
			k := [3]any{"chip", key, o.Value}
			var b ui.Element
			if o.Value == selected {
				b = ui.PrimaryButton(c.Key(k), o.Label)
			} else {
				b = ui.Button(c.Key(k), o.Label)
			}
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
				ui.Text(c, initial(src.Name)).FontSize(h / 5).Bold().TextColor(t.TextMuted).Absolute().Top(0).Bottom(0).Left(0).Right(0).TextAlign(ui.Center)
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
