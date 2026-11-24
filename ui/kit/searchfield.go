package kit

import "github.com/egoist/mygo/ui"

// SearchField is a search box: a text input bound to v with a search icon at
// its start and, while it holds text, a button at its end that clears it
// (labelled clearLabel). The input registers with the Navigator under key, as
// a text field (A opens Steam's keyboard), so nav.Handle(key) focuses it.
func SearchField(c *ui.Context, nav *Navigator, key any, v *string, placeholder, clearLabel string) ui.Element {
	t := c.Theme()
	return ui.Box(c).Children(func() {
		Field(c, nav, key, v, placeholder).Padding(S2, S8+S1, S2, S8).AlignSelf(ui.Stretch)
		ui.Row(c).Absolute().Top(0).Bottom(0).Left(S3).AlignItems(ui.Center).PassThrough().Children(func() {
			ui.Icon(c, IconSearch).FontSize(IconSM).TextColor(t.TextMuted)
		})
		if *v == "" {
			return
		}
		ui.Row(c).Absolute().Top(0).Bottom(0).Right(S1).AlignItems(ui.Center).Children(func() {
			IconBtn(c, nav, [2]any{"search-clear", key}, BtnGhost, IconX, clearLabel, func() { *v = "" }).Width(28).Height(28)
		})
	})
}
