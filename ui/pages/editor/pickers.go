package editor

import (
	"fmt"
	"image/color"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	"github.com/akfaiz/carryover/ui/i18n"

	"github.com/akfaiz/carryover/services"
	"github.com/akfaiz/carryover/ui/env"
	"github.com/akfaiz/carryover/ui/kit"
)

// artPicker is the "Find artwork on SteamGridDB" dialog. The user picks the
// game once, then flips between tabs (cover, wide cover, background, logo,
// icon) and picks an image in each, so a game isn't searched for per image type.
type artPicker struct {
	u          env.Env
	be         Backend
	steamAppID string
	start      services.ArtKind
	onPick     func(services.ArtKind, services.ArtGrid)
	onGame     func(id int, name string) // remembers the chosen game for next time

	query    *widget.Entry
	body     *fyne.Container
	errLabel *widget.Label
	d        *dialog.CustomDialog
	games    []services.ArtGame // last search results; nil until searched

	// State of the game being browsed; gen guards against late responses
	// after the user switched games.
	gen    int
	gameID int
	hosts  map[services.ArtKind]*fyne.Container
	grids  map[services.ArtKind][]services.ArtGrid
	loaded map[services.ArtKind]bool
	picked map[services.ArtKind]int // grid id chosen in this dialog
}

// artPickerOpts configures showArtPicker.
type artPickerOpts struct {
	be         Backend          // where the games and images come from
	term       string           // initial search text (the game's name)
	steamAppID string           // linked Steam app: its art is shown straight away
	gameID     int              // SteamGridDB game chosen earlier (0: none)
	gameName   string           // its name
	kind       services.ArtKind // tab to open first
	onPick     func(services.ArtKind, services.ArtGrid)
	onGame     func(id int, name string)
	onClosed   func()
}

// showArtPicker lets the user pick SteamGridDB artwork; every pick is reported
// through onPick while the dialog stays open, onClosed runs when it closes.
func showArtPicker(u env.Env, o artPickerOpts) {
	p := &artPicker{u: u, be: o.be, steamAppID: o.steamAppID, start: o.kind, onPick: o.onPick, onGame: o.onGame,
		picked: map[services.ArtKind]int{}}
	p.query = widget.NewEntry()
	p.query.SetPlaceHolder(i18n.L("Game name"))
	p.query.SetText(o.term)
	p.query.OnSubmitted = func(string) { p.search() }
	p.body = container.NewStack()
	p.errLabel = kit.Colored("", widget.DangerImportance)
	p.errLabel.Hide()

	top := kit.VBox(kit.RowFill(p.query, widget.NewButton(i18n.L("Search"), p.search)), p.errLabel)
	p.d = dialog.NewCustom(i18n.L("Find artwork on SteamGridDB"), i18n.L("Done"), container.NewBorder(top, nil, nil, nil, p.body), u.Window())
	if o.onClosed != nil {
		p.d.SetOnClosed(o.onClosed)
	}
	p.d.Resize(kit.DialogSize(u.Window()))
	p.d.Show()
	kit.OnBack(u.Window(), p.d.Hide)

	switch {
	case o.gameID > 0:
		p.loadGame(o.gameID, o.gameName)
	case o.steamAppID != "":
		// A Steam-linked game can show its art straight away.
		p.loadGame(0, i18n.L("Steam app {{.ID}}", map[string]any{"ID": o.steamAppID}))
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
	p.setBody(kit.Muted(i18n.L("Searching…")))
	term := p.query.Text
	kit.Async(func() ([]services.ArtGame, error) { return p.be.SearchGridGames(term) }, func(list []services.ArtGame, err error) {
		p.showErr(err)
		if list == nil {
			list = []services.ArtGame{}
		}
		p.games = list
		p.showGames(list)
	})
}

func (p *artPicker) showGames(list []services.ArtGame) {
	showGames(p.body, list, func(g services.ArtGame) {
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
	p.hosts = map[services.ArtKind]*fyne.Container{}
	p.grids = map[services.ArtKind][]services.ArtGrid{}
	p.loaded = map[services.ArtKind]bool{}

	back := kit.LowButton(i18n.L("← Change game"), func() {
		if p.games == nil && strings.TrimSpace(p.query.Text) != "" {
			p.search()
			return
		}
		p.gen++
		p.showGames(p.games)
	})
	crumbs := kit.HBox(back, widget.NewLabelWithStyle(name, fyne.TextAlignLeading, fyne.TextStyle{Bold: true}))

	kinds := append([]services.ArtKind{services.ArtCover}, services.ArtKinds...)
	tabs := container.NewAppTabs()
	byTab := map[*container.TabItem]services.ArtKind{}
	var startItem *container.TabItem
	for _, k := range kinds {
		host := container.NewStack(kit.Muted(i18n.L("Loading…")))
		p.hosts[k] = host
		item := container.NewTabItem(artSpecFor(k).label, host)
		byTab[item] = k
		tabs.Append(item)
		if k == p.start {
			startItem = item
		}
	}
	tabs.OnSelected = func(ti *container.TabItem) { p.ensure(byTab[ti]) }
	p.setBody(container.NewBorder(crumbs, kit.Muted(i18n.L("Pick an image in each tab you want, then press Done. Art by the SteamGridDB community.")), nil, nil, tabs))
	if startItem != nil {
		tabs.Select(startItem)
	}
	p.ensure(p.start)
}

// ensure loads the images of a kind once.
func (p *artPicker) ensure(kind services.ArtKind) {
	if p.loaded == nil || p.loaded[kind] {
		return
	}
	p.loaded[kind] = true
	gen, id, appID := p.gen, p.gameID, ""
	if id == 0 {
		appID = p.steamAppID
	}
	kit.Async(func() ([]services.ArtGrid, error) { return p.be.GridArt(kind, id, appID) }, func(grids []services.ArtGrid, err error) {
		if gen != p.gen {
			return
		}
		p.showErr(err)
		p.grids[kind] = grids
		p.render(kind)
	})
}

func (p *artPicker) render(kind services.ArtKind) {
	host, spec := p.hosts[kind], artSpecFor(kind)
	grids := p.grids[kind]
	if len(grids) == 0 {
		host.Objects = []fyne.CanvasObject{kit.Muted(spec.empty)}
		host.Refresh()
		return
	}
	grid := container.NewGridWrap(fyne.NewSize(spec.pick.Width+8, spec.pick.Height+8))
	for _, g := range grids {
		src := g.Thumb
		if src == "" {
			src = g.URL
		}
		tile := kit.NewTappable(kit.CoverView(kit.CoverSource{URL: src, Contain: spec.contain}, spec.pick), func() {
			p.picked[kind] = g.ID
			p.onPick(kind, g)
			p.render(kind)
		})
		mark := canvas.NewRectangle(color.Transparent)
		mark.CornerRadius = 8
		if p.picked[kind] == g.ID {
			mark.StrokeColor, mark.StrokeWidth = kit.ColAccent, 3
		}
		grid.Add(container.NewStack(mark, container.NewPadded(tile)))
	}
	host.Objects = []fyne.CanvasObject{container.NewVScroll(grid)}
	host.Refresh()
}

func showGames(body *fyne.Container, list []services.ArtGame, onPick func(services.ArtGame)) {
	items := kit.VBox()
	if len(list) == 0 {
		items.Add(kit.Muted(i18n.L("No games found.")))
	}
	for _, g := range list {
		info := ""
		if g.Year > 0 {
			info = fmt.Sprint(g.Year)
		}
		if g.Verified {
			info += " ✓"
		}
		items.Add(kit.ListRow(g.Name, info, func() {
			if onPick != nil {
				onPick(g)
			}
		}))
	}
	body.Objects = []fyne.CanvasObject{container.NewVScroll(items)}
	body.Refresh()
}
