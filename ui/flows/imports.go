package flows

import (
	"net/url"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/lang"
	"fyne.io/fyne/v2/widget"

	"github.com/akfaiz/carryover/ui/env"
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
	filter := widget.NewEntry()
	filter.SetPlaceHolder(lang.L("Filter…"))
	list := container.NewVBox(kit.Muted(t.reading))
	var games []importRow
	var d *dialog.CustomDialog

	render := func() {
		f := strings.ToLower(filter.Text)
		list.Objects = nil
		if len(games) == 0 {
			list.Add(kit.Muted(t.empty))
		}
		for _, g := range games {
			if !strings.Contains(strings.ToLower(g.name), f) {
				continue
			}
			var right fyne.CanvasObject
			if g.gameID != "" {
				right = container.NewCenter(kit.Badge(lang.L("In library"), kit.BadgeOK))
			}
			lead := kit.CoverView(kit.CoverSource{Name: g.name}, fyne.NewSize(36, 54))
			list.Add(kit.MediaRow(lead, g.name, right, func() {
				d.Hide()
				if g.gameID != "" {
					u.Navigate("/game/" + g.gameID)
				} else {
					u.Navigate(g.route)
				}
			}))
		}
		list.Refresh()
	}
	filter.OnChanged = func(string) { render() }

	top := kit.VBox(kit.Muted(t.intro), kit.FullHeight(filter))
	d = dialog.NewCustom(t.title, lang.L("Close"), container.NewBorder(top, nil, nil, nil, container.NewVScroll(list)), u.Window())
	d.Resize(kit.DialogSize(u.Window()))
	d.Show()
	kit.OnBack(u.Window(), d.Hide)
	kit.Async(func() ([]importRow, error) { return load(), nil }, func(rows []importRow, _ error) {
		games = rows
		render()
	})
}

// ShowSteamImport lists games installed through Steam; picking one opens the
// editor pre-filled (or the existing game).
func ShowSteamImport(u env.Env) {
	showImport(u, importText{
		title:   lang.L("Add from Steam library"),
		intro:   lang.L("Games installed through Steam on this device. The title comes from Steam; you still set the save locations. Steam keeps launching the game — Carryover syncs around it through the launch options shown on the game page."),
		reading: lang.L("Reading Steam library…"),
		empty:   lang.L("No installed Steam games found on this device."),
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
		title:   lang.L("Add from Heroic library"),
		intro:   lang.L("Games installed through Heroic on this device. Carryover starts them through Heroic, so they keep its settings (and, on Linux, its Wine prefix); you still set the save locations."),
		reading: lang.L("Reading Heroic library…"),
		empty:   lang.L("No installed Heroic games found on this device."),
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
		title:   lang.L("Add from Lutris library"),
		intro:   lang.L("Games installed through Lutris on this device. Carryover starts them through Lutris, so they keep their runner and Wine prefix; you still set the save locations."),
		reading: lang.L("Reading Lutris library…"),
		empty:   lang.L("No installed Lutris games found on this device."),
	}, func() []importRow {
		var rows []importRow
		for _, g := range u.Lib().LutrisLibrary() {
			rows = append(rows, importRow{name: g.Name, gameID: g.GameID, route: "/new/lutris/" + g.ConfigID})
		}
		return rows
	})
}
