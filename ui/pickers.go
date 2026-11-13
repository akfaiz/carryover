package ui

import (
	"fmt"
	"image/color"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/lang"
	"fyne.io/fyne/v2/widget"

	"github.com/akfaiz/carryover/internal/stores/steamgrid"
	"github.com/akfaiz/carryover/services"
)

// artPicker is the "Find artwork on SteamGridDB" dialog. The user picks the
// game once, then flips between tabs (cover, wide cover, background, logo,
// icon) and picks an image in each, so a game isn't searched for per image type.
type artPicker struct {
	u          *UI
	steamAppID string
	start      steamgrid.Kind
	onPick     func(steamgrid.Kind, steamgrid.Grid)
	onGame     func(id int, name string) // remembers the chosen game for next time

	query    *widget.Entry
	body     *fyne.Container
	errLabel *widget.Label
	d        *dialog.CustomDialog
	games    []steamgrid.Game // last search results; nil until searched

	// State of the game being browsed; gen guards against late responses
	// after the user switched games.
	gen    int
	gameID int
	hosts  map[steamgrid.Kind]*fyne.Container
	grids  map[steamgrid.Kind][]steamgrid.Grid
	loaded map[steamgrid.Kind]bool
	picked map[steamgrid.Kind]int // grid id chosen in this dialog
}

// artPickerOpts configures showArtPicker.
type artPickerOpts struct {
	term       string         // initial search text (the game's name)
	steamAppID string         // linked Steam app: its art is shown straight away
	gameID     int            // SteamGridDB game chosen earlier (0: none)
	gameName   string         // its name
	kind       steamgrid.Kind // tab to open first
	onPick     func(steamgrid.Kind, steamgrid.Grid)
	onGame     func(id int, name string)
	onClosed   func()
}

// showArtPicker lets the user pick SteamGridDB artwork; every pick is reported
// through onPick while the dialog stays open, onClosed runs when it closes.
func showArtPicker(u *UI, o artPickerOpts) {
	p := &artPicker{u: u, steamAppID: o.steamAppID, start: o.kind, onPick: o.onPick, onGame: o.onGame,
		picked: map[steamgrid.Kind]int{}}
	p.query = widget.NewEntry()
	p.query.SetPlaceHolder(lang.L("Game name"))
	p.query.SetText(o.term)
	p.query.OnSubmitted = func(string) { p.search() }
	p.body = container.NewStack()
	p.errLabel = colored("", widget.DangerImportance)
	p.errLabel.Hide()

	top := vbox(rowFill(p.query, widget.NewButton(lang.L("Search"), p.search)), p.errLabel)
	p.d = dialog.NewCustom(lang.L("Find artwork on SteamGridDB"), lang.L("Done"), container.NewBorder(top, nil, nil, nil, p.body), u.win)
	if o.onClosed != nil {
		p.d.SetOnClosed(o.onClosed)
	}
	p.d.Resize(dialogSize(u.win))
	p.d.Show()

	switch {
	case o.gameID > 0:
		p.loadGame(o.gameID, o.gameName)
	case o.steamAppID != "":
		// A Steam-linked game can show its art straight away.
		p.loadGame(0, lang.L("Steam app {{.ID}}", map[string]any{"ID": o.steamAppID}))
	case strings.TrimSpace(o.term) != "":
		p.search()
	}
}

func (p *artPicker) setBody(o fyne.CanvasObject) {
	p.body.Objects = []fyne.CanvasObject{o}
	p.body.Refresh()
}

func (p *artPicker) showErr(err error) {
	if err == nil {
		p.errLabel.Hide()
		return
	}
	p.errLabel.SetText(err.Error())
	p.errLabel.Show()
}

func (p *artPicker) search() {
	if strings.TrimSpace(p.query.Text) == "" {
		return
	}
	p.gen++
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

func (p *artPicker) showGames(list []steamgrid.Game) {
	showGames(p.body, list, func(g steamgrid.Game) {
		if p.onGame != nil {
			p.onGame(g.ID, g.Name)
		}
		p.loadGame(g.ID, g.Name)
	})
}

// loadGame shows one tab per kind for a SteamGridDB game (id 0: the linked
// Steam app); a tab's images load when it is first opened.
func (p *artPicker) loadGame(id int, name string) {
	p.showErr(nil)
	p.gen++
	p.gameID = id
	p.hosts = map[steamgrid.Kind]*fyne.Container{}
	p.grids = map[steamgrid.Kind][]steamgrid.Grid{}
	p.loaded = map[steamgrid.Kind]bool{}

	back := lowButton(lang.L("← Change game"), func() {
		if p.games == nil && strings.TrimSpace(p.query.Text) != "" {
			p.search()
			return
		}
		p.gen++
		p.showGames(p.games)
	})
	crumbs := hbox(back, widget.NewLabelWithStyle(name, fyne.TextAlignLeading, fyne.TextStyle{Bold: true}))

	kinds := append([]steamgrid.Kind{steamgrid.KindCover}, steamgrid.ArtKinds...)
	tabs := container.NewAppTabs()
	byTab := map[*container.TabItem]steamgrid.Kind{}
	var startItem *container.TabItem
	for _, k := range kinds {
		host := container.NewStack(muted(lang.L("Loading…")))
		p.hosts[k] = host
		item := container.NewTabItem(artSpecFor(k).label, host)
		byTab[item] = k
		tabs.Append(item)
		if k == p.start {
			startItem = item
		}
	}
	tabs.OnSelected = func(ti *container.TabItem) { p.ensure(byTab[ti]) }
	p.setBody(container.NewBorder(crumbs, muted(lang.L("Pick an image in each tab you want, then press Done. Art by the SteamGridDB community.")), nil, nil, tabs))
	if startItem != nil {
		tabs.Select(startItem)
	}
	p.ensure(p.start)
}

// ensure loads the images of a kind once.
func (p *artPicker) ensure(kind steamgrid.Kind) {
	if p.loaded == nil || p.loaded[kind] {
		return
	}
	p.loaded[kind] = true
	gen, id, appID := p.gen, p.gameID, ""
	if id == 0 {
		appID = p.steamAppID
	}
	async(func() ([]steamgrid.Grid, error) { return p.u.lib.GridArt(string(kind), id, appID) }, func(grids []steamgrid.Grid, err error) {
		if gen != p.gen {
			return
		}
		p.showErr(err)
		p.grids[kind] = grids
		p.render(kind)
	})
}

func (p *artPicker) render(kind steamgrid.Kind) {
	host, spec := p.hosts[kind], artSpecFor(kind)
	grids := p.grids[kind]
	if len(grids) == 0 {
		host.Objects = []fyne.CanvasObject{muted(spec.empty)}
		host.Refresh()
		return
	}
	grid := container.NewGridWrap(fyne.NewSize(spec.pick.Width+8, spec.pick.Height+8))
	for _, g := range grids {
		src := g.Thumb
		if src == "" {
			src = g.URL
		}
		tile := newTappable(coverView(coverSource{url: src, contain: spec.contain}, spec.pick), func() {
			p.picked[kind] = g.ID
			p.onPick(kind, g)
			p.render(kind)
		})
		mark := canvas.NewRectangle(color.Transparent)
		mark.CornerRadius = 8
		if p.picked[kind] == g.ID {
			mark.StrokeColor, mark.StrokeWidth = colAccent, 3
		}
		grid.Add(container.NewStack(mark, container.NewPadded(tile)))
	}
	host.Objects = []fyne.CanvasObject{container.NewVScroll(grid)}
	host.Refresh()
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
			lead := coverView(coverSource{name: g.Name}, fyne.NewSize(36, 54))
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
		muted(lang.L("Games installed through Steam on this device. The title comes from Steam; you still set the save locations. Steam keeps launching the game — Carryover syncs around it through the launch options shown on the game page.")),
		filter)
	d = dialog.NewCustom(lang.L("Add from Steam library"), lang.L("Close"), container.NewBorder(top, nil, nil, nil, container.NewVScroll(list)), u.win)
	d.Resize(dialogSize(u.win))
	d.Show()
	async(func() ([]services.SteamLibraryGame, error) { return u.lib.SteamLibrary(), nil }, func(g []services.SteamLibraryGame, _ error) {
		games = g
		render()
	})
}
