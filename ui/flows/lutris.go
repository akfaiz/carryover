package flows

import (
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/lang"
	"fyne.io/fyne/v2/widget"

	"github.com/akfaiz/carryover/services"
	"github.com/akfaiz/carryover/ui/env"
	"github.com/akfaiz/carryover/ui/kit"
)

// ShowLutrisImport lists the games Lutris has installed on this device
// (Linux); picking one opens the editor pre-filled (or the existing game).
func ShowLutrisImport(u env.Env) {
	filter := widget.NewEntry()
	filter.SetPlaceHolder(lang.L("Filter…"))
	list := container.NewVBox(kit.Muted(lang.L("Reading Lutris library…")))
	var games []services.LutrisLibraryGame
	var d *dialog.CustomDialog

	render := func() {
		f := strings.ToLower(filter.Text)
		list.Objects = nil
		if len(games) == 0 {
			list.Add(kit.Muted(lang.L("No installed Lutris games found on this device.")))
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
					u.Navigate("/new/lutris/" + g.ConfigID)
				}
			}))
		}
		list.Refresh()
	}
	filter.OnChanged = func(string) { render() }

	top := kit.VBox(
		kit.Muted(lang.L("Games installed through Lutris on this device. Carryover starts them through Lutris, so they keep their runner and Wine prefix; you still set the save locations.")),
		kit.FullHeight(filter))
	d = dialog.NewCustom(lang.L("Add from Lutris library"), lang.L("Close"), container.NewBorder(top, nil, nil, nil, container.NewVScroll(list)), u.Window())
	d.Resize(kit.DialogSize(u.Window()))
	d.Show()
	kit.OnBack(u.Window(), d.Hide)
	kit.Async(func() ([]services.LutrisLibraryGame, error) { return u.Lib().LutrisLibrary(), nil }, func(g []services.LutrisLibraryGame, _ error) {
		games = g
		render()
	})
}
