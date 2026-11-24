package kit

import "github.com/egoist/mygo/ui"

// PadHint is one entry of a controller hint row: the button (or keys) and what
// pressing it does.
type PadHint struct {
	Button, Label string
}

// PadHints draws a row of controller hints, each button as a small key cap
// followed by what it does.
func PadHints(c *ui.Context, hints []PadHint) ui.Element {
	t := c.Theme()
	return ui.Row(c).Gap(S4).AlignItems(ui.Center).Wrap().Children(func() {
		for _, h := range hints {
			ui.Row(c).Gap(S1 + 2).AlignItems(ui.Center).Children(func() {
				ui.Row(c).Padding(2, S2).Radius(RadiusSM).Background(col(ColBg4)).Border(1, t.Border).Children(func() {
					ui.Text(c, h.Button).FontSize(TextXS).FontWeight(700).TextColor(t.Text)
				})
				ui.Text(c, h.Label).FontSize(TextSM).TextColor(t.TextMuted)
			})
		}
	})
}
