package ui

import (
	"fmt"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/lang"
	"fyne.io/fyne/v2/widget"

	"github.com/akfaiz/carryover/internal/steamgrid"
	"github.com/akfaiz/carryover/services"
)

// showCoverPicker lets the user pick a SteamGridDB cover.
// coverPicker is the "Find cover on SteamGridDB" dialog: search for a game,
// then pick one of its portrait covers.
type coverPicker struct {
	u          *UI
	steamAppID string
	onPick     func(steamgrid.Grid)

	query    *widget.Entry
	body     *fyne.Container
	errLabel *widget.Label
	d        *dialog.CustomDialog
	games    []steamgrid.Game // last search results; nil until searched
}

func showCoverPicker(u *UI, term, steamAppID string, onPick func(steamgrid.Grid)) {
	p := &coverPicker{u: u, steamAppID: steamAppID, onPick: onPick}
	p.query = widget.NewEntry()
	p.query.SetPlaceHolder(lang.L("Game name"))
	p.query.SetText(term)
	p.query.OnSubmitted = func(string) { p.search() }
	p.body = container.NewStack()
	p.errLabel = colored("", widget.DangerImportance)
	p.errLabel.Hide()

	top := vbox(rowFill(p.query, widget.NewButton(lang.L("Search"), p.search)), p.errLabel)
	p.d = dialog.NewCustom(lang.L("Find cover on SteamGridDB"), lang.L("Close"), container.NewBorder(top, nil, nil, nil, p.body), u.win)
	p.d.Resize(dialogSize(u.win))
	p.d.Show()

	// A Steam-linked game can show its covers straight away.
	if steamAppID != "" {
		p.loadGrids(0, lang.L("Steam app {{.ID}}", map[string]any{"ID": steamAppID}))
	} else if strings.TrimSpace(term) != "" {
		p.search()
	}
}

func (p *coverPicker) setBody(o fyne.CanvasObject) {
	p.body.Objects = []fyne.CanvasObject{o}
	p.body.Refresh()
}

func (p *coverPicker) showErr(err error) {
	if err == nil {
		p.errLabel.Hide()
		return
	}
	p.errLabel.SetText(err.Error())
	p.errLabel.Show()
}

func (p *coverPicker) showGames(list []steamgrid.Game) {
	showGames(p.body, list, func(g steamgrid.Game) { p.loadGrids(g.ID, g.Name) })
}

func (p *coverPicker) search() {
	if strings.TrimSpace(p.query.Text) == "" {
		return
	}
	p.showErr(nil)
	p.setBody(muted(lang.L("Searching…")))
	term := p.query.Text
	async(func() ([]steamgrid.Game, error) { return p.u.lib.SearchGridGames(term) }, func(list []steamgrid.Game, err error) {
		p.showErr(err)
		if list == nil {
			list = []steamgrid.Game{}
		}
		p.games = list
		p.showGames(list)
	})
}

// loadGrids shows the covers of a SteamGridDB game (id 0: the linked Steam app).
func (p *coverPicker) loadGrids(id int, name string) {
	p.showErr(nil)
	back := lowButton(lang.L("← Results"), func() {
		if p.games == nil && strings.TrimSpace(p.query.Text) != "" {
			p.search()
			return
		}
		p.showGames(p.games)
	})
	crumbs := hbox(back, widget.NewLabelWithStyle(name, fyne.TextAlignLeading, fyne.TextStyle{Bold: true}))
	p.setBody(container.NewBorder(crumbs, nil, nil, nil, muted(lang.L("Loading covers…"))))
	appID := ""
	if id == 0 {
		appID = p.steamAppID
	}
	async(func() ([]steamgrid.Grid, error) { return p.u.lib.GridCovers(id, appID) }, func(grids []steamgrid.Grid, err error) {
		p.showErr(err)
		if len(grids) == 0 {
			p.setBody(container.NewBorder(crumbs, nil, nil, nil, muted(lang.L("No portrait covers for this game yet."))))
			return
		}
		grid := container.NewGridWrap(fyne.NewSize(120, 180))
		for _, g := range grids {
			src := g.Thumb
			if src == "" {
				src = g.URL
			}
			grid.Add(newTappable(coverView(coverSource{url: src}, fyne.NewSize(120, 180)), func() {
				p.d.Hide()
				p.onPick(g)
			}))
		}
		p.setBody(container.NewBorder(crumbs, muted(lang.L("Art by the SteamGridDB community. Click one to use it.")), nil, nil,
			container.NewVScroll(grid)))
	})
}

func showGames(body *fyne.Container, list []steamgrid.Game, onPick func(steamgrid.Game)) {
	items := vbox()
	if len(list) == 0 {
		items.Add(muted(lang.L("No games found.")))
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
	filter.SetPlaceHolder(lang.L("Filter…"))
	list := container.NewVBox(muted(lang.L("Reading Steam library…")))
	var games []services.SteamLibraryGame
	var d *dialog.CustomDialog

	render := func() {
		f := strings.ToLower(filter.Text)
		list.Objects = nil
		if len(games) == 0 {
			list.Add(muted(lang.L("No installed Steam games found on this device.")))
		}
		for _, g := range games {
			if !strings.Contains(strings.ToLower(g.Name), f) {
				continue
			}
			var right fyne.CanvasObject
			if g.GameID != "" {
				right = container.NewCenter(badge(lang.L("in library"), badgeOK))
			}
			lead := coverView(coverSource{url: steamCoverURL(g.AppID)}, fyne.NewSize(36, 54))
			list.Add(mediaRow(lead, g.Name, right, func() {
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
		muted(lang.L("Games installed through Steam on this device. The title and cover come from Steam; you still set the save locations. Steam keeps launching the game — Carryover syncs around it through the launch options shown on the game page.")),
		filter)
	d = dialog.NewCustom(lang.L("Add from Steam library"), lang.L("Close"), container.NewBorder(top, nil, nil, nil, container.NewVScroll(list)), u.win)
	d.Resize(dialogSize(u.win))
	d.Show()
	async(func() ([]services.SteamLibraryGame, error) { return u.lib.SteamLibrary(), nil }, func(g []services.SteamLibraryGame, _ error) {
		games = g
		render()
	})
}
