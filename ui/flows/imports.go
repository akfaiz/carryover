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

	// A launcher's library can be hundreds of games: the list builds the rows in
	// view, and which games the filter lets through is worked out when the filter
	// or the games change, not in every frame.
	var (
		list    ui.ListState
		keys    []string // the games' names, lower-cased
		shown   []int    // the games the filter lets through
		shownBy string   // the filter they were worked out for
		fresh   = true   // they are to be worked out, whatever the filter
	)
	list.Key = func(i int) any { return shown[i] }
	refilter := func() {
		f := strings.ToLower(filter)
		shown, shownBy, fresh = shown[:0], filter, false
		for i, k := range keys {
			if strings.Contains(k, f) {
				shown = append(shown, i)
			}
		}
	}

	d = u.Dialogs().Show(&kit.Dialog{Title: t.title, Wide: true, Body: func(c *ui.Context, nav *kit.Navigator) {
		kit.Muted(c, t.intro)
		kit.Field(c, nav, "import-filter", &filter, i18n.L("Filter…"))
		if fresh || filter != shownBy {
			refilter()
		}
		// Padded by room for a focused row's ring, which the list clips, and as far
		// out again, so that the rows are where they would be without it (ScrollBox).
		ui.List(c, &list, len(shown), func(i int) {
			at := shown[i]
			g := games[at]
			open := func() {
				d.Close()
				if g.gameID != "" {
					u.Navigate("/game/" + g.gameID)
				} else {
					u.Navigate(g.route)
				}
			}
			kit.ListButton(c, nav, [2]any{"import-row", at}, g.name, open, func() {
				ui.Row(c).Gap(kit.S3).Padding(kit.S2, kit.S3).AlignItems(ui.Center).Children(func() {
					kit.CoverView(c, kit.CoverSource{Name: g.name}, 36, 54)
					ui.Text(c, g.name).Grow(1)
					if g.gameID != "" {
						kit.Badge(c, i18n.L("In library"), kit.BadgeOK)
					}
				})
			})
		}).MaxHeight(380).Margin(-kit.RingRoom).Padding(kit.RingRoom).Gap(kit.S1).Children(func() {
			// What shows while there are no rows.
			switch {
			case !loaded:
				kit.Muted(c, t.reading)
			case len(games) == 0:
				kit.Muted(c, t.empty)
			}
		})
		kit.Footer(c, func() {
			kit.Btn(c, nav, "import-close", kit.BtnSecondary, nil, i18n.L("Close"), func() { d.Close() }, kit.BtnItem(kit.FocusSafe))
		})
	}})
	kit.Async(func() ([]importRow, error) { return load(), nil }, func(rows []importRow, _ error) {
		games, loaded = rows, true
		keys = make([]string, len(rows))
		for i, r := range rows {
			keys[i] = strings.ToLower(r.name)
		}
		fresh = true // work the filter out again
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
