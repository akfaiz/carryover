package kit

import "github.com/egoist/mygo/ui"

// Icons are small SVGs for the places MyGo has no stock glyph.
var (
	// IconPlay is a play triangle: the game itself, no emulator.
	IconPlay = ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="currentColor"><path d="M7 4.5v15l12-7.5z"/></svg>`))
	// IconDownload is an arrow into a tray: a game installed by another launcher.
	IconDownload = ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M12 4v11M7 11l5 5 5-5M5 20h14"/></svg>`))
	// IconStorage is a stack of drives.
	IconStorage = ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect x="3" y="4" width="18" height="6" rx="1"/><rect x="3" y="14" width="18" height="6" rx="1"/><path d="M7 7h.01M7 17h.01"/></svg>`))
	// IconFolder is an open folder.
	IconFolder = ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M3 7a2 2 0 0 1 2-2h4l2 2h8a2 2 0 0 1 2 2v8a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2z"/></svg>`))
	// IconComputer is a monitor on a stand: an emulator.
	IconComputer = ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect x="3" y="4" width="18" height="12" rx="2"/><path d="M8 20h8M12 16v4"/></svg>`))
)

// ChoiceTile is a square button for one way to play a game: an icon over its
// name, with a ring around the highlighted one. key tells it from the others on
// the page.
func ChoiceTile(c *ui.Context, nav *Navigator, key any, name string, icon *ui.SVG, highlighted bool, tapped func(), opts ...ItemOption) ui.Element {
	t := c.Theme()
	border, width := t.Border, float32(1)
	if highlighted {
		border, width = t.Accent, 2
	}
	k := [2]any{"tile", key}
	b := ui.ButtonBase(c.Key(k)).Label(name).Width(ChoiceTileSize).Height(ChoiceTileSize).Radius(10).
		Background(t.Surface).Border(width, border).Children(func() {
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
