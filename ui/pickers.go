package ui

import (
	"fmt"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	"github.com/akfaiz/playrelay/internal/steamgrid"
	"github.com/akfaiz/playrelay/services"
)

// showCoverPicker lets the user pick a SteamGridDB cover.
func showCoverPicker(u *UI, term, steamAppID string, onPick func(steamgrid.Grid)) {
	query := widget.NewEntry()
	query.SetPlaceHolder("Game name")
	query.SetText(term)
	body := container.NewStack()
	errLabel := colored("", widget.DangerImportance)
	errLabel.Hide()
	var d *dialog.CustomDialog
	var games []steamgrid.Game
	var search func()

	setBody := func(o fyne.CanvasObject) {
		body.Objects = []fyne.CanvasObject{o}
		body.Refresh()
	}
	showErr := func(err error) {
		if err == nil {
			errLabel.Hide()
			return
		}
		errLabel.SetText(err.Error())
		errLabel.Show()
	}

	loadGrids := func(id int, name string) {
		showErr(nil)
		back := lowButton("← Results", func() {
			if games == nil && strings.TrimSpace(query.Text) != "" {
				search()
				return
			}
			showGames(body, games, nil)
		})
		crumbs := hbox(back, widget.NewLabelWithStyle(name, fyne.TextAlignLeading, fyne.TextStyle{Bold: true}))
		setBody(container.NewBorder(crumbs, nil, nil, nil, muted("Loading covers…")))
		appID := ""
		if id == 0 {
			appID = steamAppID
		}
		async(func() ([]steamgrid.Grid, error) { return u.lib.GridCovers(id, appID) }, func(grids []steamgrid.Grid, err error) {
			showErr(err)
			if len(grids) == 0 {
				setBody(container.NewBorder(crumbs, nil, nil, nil, muted("No portrait covers for this game yet.")))
				return
			}
			grid := container.NewGridWrap(fyne.NewSize(120, 180))
			for _, g := range grids {
				src := g.Thumb
				if src == "" {
					src = g.URL
				}
				grid.Add(newTappable(coverView(coverSource{url: src}, fyne.NewSize(120, 180)), func() {
					d.Hide()
					onPick(g)
				}))
			}
			setBody(container.NewBorder(crumbs, muted("Art by the SteamGridDB community. Click one to use it."), nil, nil,
				container.NewVScroll(grid)))
		})
	}

	showGamesFn := func(list []steamgrid.Game) {
		showGames(body, list, func(g steamgrid.Game) { loadGrids(g.ID, g.Name) })
	}
	search = func() {
		if strings.TrimSpace(query.Text) == "" {
			return
		}
		showErr(nil)
		setBody(muted("Searching…"))
		async(func() ([]steamgrid.Game, error) { return u.lib.SearchGridGames(query.Text) }, func(list []steamgrid.Game, err error) {
			showErr(err)
			games = list
			if list == nil {
				games = []steamgrid.Game{}
			}
			showGamesFn(games)
		})
	}
	query.OnSubmitted = func(string) { search() }

	top := vbox(rowFill(query, widget.NewButton("Search", search)), errLabel)
	content := container.NewBorder(top, nil, nil, nil, body)
	d = dialog.NewCustom("Find cover on SteamGridDB", "Close", content, u.win)
	d.Resize(dialogSize(u.win))
	d.Show()

	// A Steam-linked game can show its covers straight away.
	if steamAppID != "" {
		loadGrids(0, "Steam app "+steamAppID)
	} else if strings.TrimSpace(term) != "" {
		search()
	}
	_ = showGamesFn
}

func showGames(body *fyne.Container, list []steamgrid.Game, onPick func(steamgrid.Game)) {
	items := vbox()
	if len(list) == 0 {
		items.Add(muted("No games found."))
	}
	for _, g := range list {
		info := ""
		if g.Year > 0 {
			info = fmt.Sprint(g.Year)
		}
		if g.Verified {
			info += " ✓"
		}
		items.Add(listRow(g.Name, info, func() {
			if onPick != nil {
				onPick(g)
			}
		}))
	}
	body.Objects = []fyne.CanvasObject{container.NewVScroll(items)}
	body.Refresh()
}

// showSteamImport lists games installed through Steam; picking one opens the
// editor pre-filled (or the existing game).
func showSteamImport(u *UI) {
	filter := widget.NewEntry()
	filter.SetPlaceHolder("Filter…")
	list := container.NewVBox(muted("Reading Steam library…"))
	var games []services.SteamLibraryGame
	var d *dialog.CustomDialog

	render := func() {
		f := strings.ToLower(filter.Text)
		list.Objects = nil
		if len(games) == 0 {
			list.Add(muted("No installed Steam games found on this device."))
		}
		for _, g := range games {
			if !strings.Contains(strings.ToLower(g.Name), f) {
				continue
			}
			name := widget.NewLabel(g.Name)
			name.Truncation = fyne.TextTruncateEllipsis
			var right fyne.CanvasObject = widget.NewLabel("")
			if g.GameID != "" {
				right = container.NewCenter(badge("in library", badgeOK))
			}
			row := container.NewBorder(nil, nil, coverView(coverSource{url: steamCoverURL(g.AppID)}, fyne.NewSize(36, 54)), right, name)
			list.Add(newTappable(container.NewPadded(row), func() {
				d.Hide()
				if g.GameID != "" {
					u.navigate("/game/" + g.GameID)
				} else {
					u.navigate("/new/steam/" + g.AppID)
				}
			}))
		}
		list.Refresh()
	}
	filter.OnChanged = func(string) { render() }

	top := vbox(
		muted("Games installed through Steam on this device. The title and cover come from Steam; you still set the save locations. "+
			"Steam keeps launching the game — PlayRelay syncs around it through the launch options shown on the game page."),
		filter)
	d = dialog.NewCustom("Add from Steam library", "Close", container.NewBorder(top, nil, nil, nil, container.NewVScroll(list)), u.win)
	d.Resize(dialogSize(u.win))
	d.Show()
	async(func() ([]services.SteamLibraryGame, error) { return u.lib.SteamLibrary(), nil }, func(g []services.SteamLibraryGame, _ error) {
		games = g
		render()
	})
}
