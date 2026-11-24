package flows

import (
	"net/url"
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/akfaiz/carryover/ui/env"
	"github.com/akfaiz/carryover/ui/i18n"
	"github.com/akfaiz/carryover/ui/kit"
)

// importRow is one game in another launcher's library.
type importRow struct {
	name   string
	gameID string // set when the library already has the game
	route  string // the editor pre-filled from it
}

// importText is what an import dialog says.
type importText struct {
	title, intro, reading, empty string
}

// showImport lists the games another launcher installed on this device;
// picking one opens the editor pre-filled from it, or the game already in the
// library.
func showImport(u env.Env, t importText, load func() []importRow) {
	var games []importRow
	loaded := false
	filter := ""
	var d *kit.Dialog
	d = u.Dialogs().Show(&kit.Dialog{Title: t.title, Wide: true, Body: func(c *ui.Context, nav *kit.Navigator) {
		kit.Muted(c, t.intro)
		nav.Item(nav.Handle("import-filter"), ui.TextInput(c.Key("import-filter"), &filter).Placeholder(i18n.L("Filter…")).Label(i18n.L("Filter…")), nil, kit.TextField)
		ui.Scroll(c).MaxHeight(380).Children(func() {
			ui.Column(c).Gap(4).Children(func() {
				switch {
				case !loaded:
					kit.Muted(c, t.reading)
				case len(games) == 0:
					kit.Muted(c, t.empty)
				}
				f := strings.ToLower(filter)
				for i, g := range games {
					if !strings.Contains(strings.ToLower(g.name), f) {
						continue
					}
					k := [2]any{"import-row", i}
					open := func() {
						d.Close()
						if g.gameID != "" {
							u.Navigate("/game/" + g.gameID)
						} else {
							u.Navigate(g.route)
						}
					}
					b := ui.ButtonBase(c.Key(k)).Label(g.name).Radius(8).Children(func() {
						ui.Row(c).Gap(10).Padding(6, 8).AlignItems(ui.Center).Children(func() {
							kit.CoverView(c, kit.CoverSource{Name: g.name}, 36, 54)
							ui.Text(c, g.name).Grow(1)
							if g.gameID != "" {
								kit.Badge(c, i18n.L("In library"), kit.BadgeOK)
							}
						})
					})
					if nav.Item(nav.Handle(k), b.AlignSelf(ui.Stretch), open).Clicked() {
						open()
					}
				}
			})
		})
		close := func() { d.Close() }
		if nav.Item(nav.Handle("import-close"), ui.Button(c.Key("import-close"), i18n.L("Close")), close, kit.FocusSafe).Clicked() {
			close()
		}
	}})
	kit.Async(func() ([]importRow, error) { return load(), nil }, func(rows []importRow, _ error) {
		games, loaded = rows, true
	})
}

// ShowSteamImport lists games installed through Steam; picking one opens the
// editor pre-filled (or the existing game).
func ShowSteamImport(u env.Env) {
	showImport(u, importText{
		title:   i18n.L("Add from Steam library"),
		intro:   i18n.L("Games installed through Steam on this device. The title comes from Steam; you still set the save locations. Steam keeps launching the game — Carryover syncs around it through the launch options shown on the game page."),
		reading: i18n.L("Reading Steam library…"),
		empty:   i18n.L("No installed Steam games found on this device."),
	}, func() []importRow {
		var rows []importRow
		for _, g := range u.Lib().SteamLibrary() {
			rows = append(rows, importRow{name: g.Name, gameID: g.GameID, route: "/new/steam/" + g.AppID})
		}
		return rows
	})
}

// ShowHeroicImport lists the games Heroic installed on this device (Linux and
// Windows); picking one opens the editor pre-filled (or the existing game).
func ShowHeroicImport(u env.Env) {
	showImport(u, importText{
		title:   i18n.L("Add from Heroic library"),
		intro:   i18n.L("Games installed through Heroic on this device. Carryover starts them through Heroic, so they keep its settings (and, on Linux, its Wine prefix); you still set the save locations."),
		reading: i18n.L("Reading Heroic library…"),
		empty:   i18n.L("No installed Heroic games found on this device."),
	}, func() []importRow {
		var rows []importRow
		for _, g := range u.Lib().HeroicLibrary() {
			rows = append(rows, importRow{name: g.Name, gameID: g.GameID,
				route: "/new/heroic/" + g.Runner + "/" + url.PathEscape(g.AppName)})
		}
		return rows
	})
}

// ShowLutrisImport lists the games Lutris has installed on this device
// (Linux); picking one opens the editor pre-filled (or the existing game).
func ShowLutrisImport(u env.Env) {
	showImport(u, importText{
		title:   i18n.L("Add from Lutris library"),
		intro:   i18n.L("Games installed through Lutris on this device. Carryover starts them through Lutris, so they keep their runner and Wine prefix; you still set the save locations."),
		reading: i18n.L("Reading Lutris library…"),
		empty:   i18n.L("No installed Lutris games found on this device."),
	}, func() []importRow {
		var rows []importRow
		for _, g := range u.Lib().LutrisLibrary() {
			rows = append(rows, importRow{name: g.Name, gameID: g.GameID, route: "/new/lutris/" + g.ConfigID})
		}
		return rows
	})
}
