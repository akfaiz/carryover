package components

import (
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/lang"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/akfaiz/carryover/services"
	"github.com/akfaiz/carryover/ui/env"
	"github.com/akfaiz/carryover/ui/kit"
)

// ShowAddGame asks what kind of game to add: one installed by Steam, another
// PC game, or a game played through an emulator. Each opens the editor set up
// for it.
func ShowAddGame(u env.Env) {
	var d DialogHider
	pick := func(f func()) func() { return func() { d.Hide(); f() } }
	tile := func(name string, icon fyne.Resource, f func()) fyne.CanvasObject {
		return container.New(layout.NewGridWrapLayout(fyne.NewSize(kit.ChoiceTileSize+16, kit.ChoiceTileSize)),
			kit.NewChoiceTile(name, icon, false, pick(f)))
	}
	body := kit.VBox(
		kit.HBox(
			tile(lang.L("From Steam"), theme.DownloadIcon(), func() { ShowSteamImport(u) }),
			tile(lang.L("PC game"), theme.ComputerIcon(), func() { u.Navigate("/new") }),
			tile(lang.L("Emulated game"), theme.MediaPlayIcon(), func() { u.Navigate("/new/emulator") }),
		),
		kit.Muted(lang.L("From Steam: a game Steam installed on this device. PC game: any other game, started from its executable or through Heroic. Emulated game: a game file you play through an emulator such as Eden or Ryujinx.")),
		kit.HBox(layout.NewSpacer(), widget.NewButton(lang.L("Cancel"), func() { d.Hide() })),
	)
	d = ShowCustom(u, lang.L("Add a game"), body, 500)
}

// ShowSteamImport lists games installed through Steam; picking one opens the
// editor pre-filled (or the existing game).
func ShowSteamImport(u env.Env) {
	filter := widget.NewEntry()
	filter.SetPlaceHolder(lang.L("Filter…"))
	list := container.NewVBox(kit.Muted(lang.L("Reading Steam library…")))
	var games []services.SteamLibraryGame
	var d *dialog.CustomDialog

	render := func() {
		f := strings.ToLower(filter.Text)
		list.Objects = nil
		if len(games) == 0 {
			list.Add(kit.Muted(lang.L("No installed Steam games found on this device.")))
		}
		for _, g := range games {
			if !strings.Contains(strings.ToLower(g.Name), f) {
				continue
			}
			var right fyne.CanvasObject
			if g.GameID != "" {
				right = container.NewCenter(kit.Badge(lang.L("In library"), kit.BadgeOK))
			}
			lead := kit.CoverView(kit.CoverSource{Name: g.Name}, fyne.NewSize(36, 54))
			list.Add(kit.MediaRow(lead, g.Name, right, func() {
				d.Hide()
				if g.GameID != "" {
					u.Navigate("/game/" + g.GameID)
				} else {
					u.Navigate("/new/steam/" + g.AppID)
				}
			}))
		}
		list.Refresh()
	}
	filter.OnChanged = func(string) { render() }

	top := kit.VBox(
		kit.Muted(lang.L("Games installed through Steam on this device. The title comes from Steam; you still set the save locations. Steam keeps launching the game — Carryover syncs around it through the launch options shown on the game page.")),
		kit.FullHeight(filter))
	d = dialog.NewCustom(lang.L("Add from Steam library"), lang.L("Close"), container.NewBorder(top, nil, nil, nil, container.NewVScroll(list)), u.Window())
	d.Resize(kit.DialogSize(u.Window()))
	d.Show()
	kit.OnBack(u.Window(), d.Hide)
	kit.Async(func() ([]services.SteamLibraryGame, error) { return u.Lib().SteamLibrary(), nil }, func(g []services.SteamLibraryGame, _ error) {
		games = g
		render()
	})
}
