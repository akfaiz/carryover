package library

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/lang"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/akfaiz/carryover/ui/env"
	"github.com/akfaiz/carryover/ui/kit"
	"github.com/akfaiz/carryover/ui/store"
)

// ShowAttention sets the library's filter to the games that need attention,
// for the next time the page opens.
func ShowAttention(u env.Env) {
	NewViewModel(StoreSource(u.Store()), u.App().Preferences()).SetFilter(FilterAttention)
}

// Page is the library page. It builds its widgets once and draws them again
// from the ViewModel when the shared state or the user's choices change.
type Page struct {
	u  env.Env
	vm *ViewModel

	scroll  *container.Scroll
	body    *fyne.Container // the page's rows, laid out again when one changes height
	search  *widget.Entry
	syncBtn *widget.Button
	head    fyne.CanvasObject // header row, refreshed when syncBtn's label changes
	// chipsHost holds toolsRow (search and sort, the filters under them) once
	// there are games; filterHost is the row of filter buttons, the only part
	// drawn again (the search field stays the same widget while typing).
	chipsHost  *fyne.Container
	toolsRow   fyne.CanvasObject
	filterHost *fyne.Container
	gridHost   *fyne.Container
	emptyHost  *fyne.Container
	barHost    *fyne.Container // "Sync all" progress, shown only while syncing
	bar        *widget.ProgressBar
	barLabel   *widget.Label
	tiles      map[string]*Tile // by game id, kept across renders
}

// New returns the library page.
func New(u env.Env) *Page {
	return &Page{
		u:     u,
		vm:    NewViewModel(StoreSource(u.Store()), u.App().Preferences()),
		tiles: map[string]*Tile{},
	}
}

// ChipsHost and GridHost are the rows whose layout the page's tests check.
func (p *Page) ChipsHost() *fyne.Container { return p.chipsHost }
func (p *Page) GridHost() *fyne.Container  { return p.gridHost }

// Tiles lists the tiles on the page.
func (p *Page) Tiles() map[string]*Tile { return p.tiles }

func (p *Page) View() fyne.CanvasObject {
	p.search = widget.NewEntry()
	p.search.SetPlaceHolder(lang.L("Search…"))
	p.search.OnChanged = func(s string) { p.vm.SetQuery(s); p.renderGrid() }
	p.syncBtn = widget.NewButtonWithIcon(lang.L("Sync all"), theme.ViewRefreshIcon(), p.u.SyncAll)
	p.head = kit.PageHead(nil, lang.L("Library"),
		p.syncBtn,
		kit.Primary(lang.L("＋ Add game"), p.u.AddGame),
	)
	// Search and sort on one row, the filters on their own row under it: three
	// kinds of control in one row ran together.
	sorts := &kit.Choice{}
	sorts.Add(string(SortName), lang.L("Sort by name"))
	sorts.Add(string(SortRecent), lang.L("Sort by last sync"))
	sortSel := kit.SelectOf(sorts, string(p.vm.Sort()), func(v string) {
		p.vm.SetSort(Sort(v))
		p.renderGrid()
	})
	p.filterHost = container.NewStack()
	p.filterHost.Hide()
	// The extra space below sets the tools apart from the grid; it goes with
	// them, so an empty library has none.
	p.toolsRow = container.New(layout.NewCustomPaddedLayout(0, 8, 0, 0), kit.VBox(
		container.NewHBox(kit.FixedWidth(360, p.search), kit.FixedWidth(190, kit.Outlined(sortSel))),
		p.filterHost))
	p.chipsHost = container.NewStack()
	p.gridHost = container.NewStack()
	p.emptyHost = container.NewStack()
	p.bar = widget.NewProgressBar()
	p.barLabel = kit.Small("")
	p.barHost = container.NewVBox(p.barLabel, p.bar)
	p.barHost.Hide()
	p.renderGrid()
	p.updateProgress()
	p.body = kit.VBox(p.head, p.barHost, p.chipsHost, p.emptyHost, p.gridHost)
	p.scroll = kit.ScrollPage(p.body)
	return p.scroll
}

// relayout lays the page out again after a row's height changed: the filter
// row and the empty-state card come and go (on startup the page is drawn
// before the games are read), and a refreshed row alone keeps its old height.
func (p *Page) relayout() {
	if p.body == nil {
		return
	}
	p.body.Refresh()
	p.scroll.Refresh()
}

func (p *Page) FocusSearch() { p.u.Window().Canvas().Focus(p.search) }

// DefaultFocus starts a gamepad on the first game.
func (p *Page) DefaultFocus() fyne.Focusable {
	if games := p.vm.Visible(); len(games) > 0 {
		if t := p.tiles[games[0].ID]; t != nil {
			return t.open
		}
	}
	return nil
}

func (p *Page) SyncNow() {
	if p.vm.CanSyncNow() {
		p.u.SyncAll()
	}
}

// updateProgress shows how many games "Sync all" has finished, and turns its
// button into Cancel while it runs.
func (p *Page) updateProgress() {
	state, enabled := p.vm.SyncButton()
	switch state {
	case SyncCancelling:
		p.syncBtn.SetText(lang.L("Cancelling…"))
		p.syncBtn.SetIcon(theme.CancelIcon())
	case SyncRunning:
		p.syncBtn.SetText(lang.L("Cancel sync"))
		p.syncBtn.SetIcon(theme.CancelIcon())
	default:
		p.syncBtn.SetText(lang.L("Sync all"))
		p.syncBtn.SetIcon(theme.ViewRefreshIcon())
	}
	kit.SetEnabled(p.syncBtn, enabled)
	if p.head != nil {
		p.head.Refresh() // the button's label width changed
	}
	pr, running := p.vm.Progress()
	if !running {
		if p.barHost.Visible() {
			p.barHost.Hide()
			p.relayout()
		}
		return
	}
	p.bar.Max = float64(pr.Total)
	p.bar.SetValue(float64(pr.Done))
	text := lang.L("Synced {{.Done}} of {{.Total}}", map[string]any{"Done": pr.Done, "Total": pr.Total})
	if pr.Name != "" {
		text += " · " + pr.Name
	}
	p.barLabel.SetText(text)
	if !p.barHost.Visible() {
		p.barHost.Show()
		p.relayout()
	}
}

func (p *Page) StateChanged(what store.Topic) {
	if what == store.Progress {
		p.updateProgress()
		return
	}
	p.updateProgress() // the remote may have been set up
	p.renderGrid()
}

func (p *Page) renderChips() {
	if !p.vm.HasGames() {
		p.chipsHost.Objects = nil
		p.chipsHost.Refresh()
		return
	}
	p.filterHost.Objects = nil
	p.filterHost.Hide() // no gap above the grid for an empty row
	if p.vm.ShowFilters() {
		c := p.vm.Counts()
		p.filterHost.Objects = []fyne.CanvasObject{kit.Chips([]kit.ChipOption{
			{Value: string(FilterAll), Label: lang.L("All ({{.Count}})", map[string]any{"Count": c.All})},
			{Value: string(FilterReady), Label: lang.L("Ready to play ({{.Count}})", map[string]any{"Count": c.Ready}), Icon: theme.MediaPlayIcon()},
			{Value: string(FilterAttention), Label: lang.L("Needs attention ({{.Count}})", map[string]any{"Count": c.Attention}), Icon: theme.WarningIcon()},
		}, string(p.vm.Filter()), func(v string) {
			p.vm.SetFilter(Filter(v))
			p.renderGrid()
		})}
		p.filterHost.Show()
	}
	p.filterHost.Refresh()
	p.chipsHost.Objects = []fyne.CanvasObject{p.toolsRow}
	p.chipsHost.Refresh()
}

// showAll is the "Show all games" button of the empty states that come from a
// filter.
func (p *Page) showAll() fyne.CanvasObject {
	return kit.HBox(widget.NewButton(lang.L("Show all games"), func() { p.vm.SetFilter(FilterAll); p.renderGrid() }))
}

func (p *Page) renderEmpty() {
	u := p.u
	p.emptyHost.Objects = nil
	switch p.vm.Empty() {
	case EmptyNoGames:
		actions := kit.HBox(kit.Primary(lang.L("＋ Add your first game"), u.AddGame))
		if !p.vm.src.HasRemote() {
			actions.Add(widget.NewButtonWithIcon(lang.L("Set up cloud storage"), theme.StorageIcon(), func() { u.Navigate("/cloud") }))
		}
		p.emptyHost.Objects = []fyne.CanvasObject{kit.Card(lang.L("No games yet"),
			kit.Muted(lang.L("Add a game, tell Carryover where its saves live on this device, and it will keep them in sync through your cloud storage every time you play — from here or from Steam.")),
			actions)}
	case EmptyNoMatch:
		p.emptyHost.Objects = []fyne.CanvasObject{kit.Card("",
			kit.Muted(lang.L("No games match “{{.Query}}”.", map[string]any{"Query": p.vm.Query()})),
			kit.HBox(widget.NewButton(lang.L("Clear search"), func() { p.search.SetText("") })))}
	case EmptyNoReady:
		p.emptyHost.Objects = []fyne.CanvasObject{kit.Card("",
			kit.Muted(lang.L("No game can be played on this device yet. Set one up to play it here.")),
			p.showAll())}
	case EmptyNoAttention:
		p.emptyHost.Objects = []fyne.CanvasObject{kit.Card("",
			kit.Muted(lang.L("Nothing needs attention: every game is set up and synced.")),
			p.showAll())}
	}
	p.emptyHost.Refresh()
}

func (p *Page) renderGrid() {
	p.renderChips()
	games := p.vm.Visible()
	p.renderEmpty()

	// Reuse the tiles: only games new to the page get one built.
	var tiles []fyne.CanvasObject
	cell := fyne.NewSize(170, 0)
	for _, g := range games {
		t := p.tiles[g.ID]
		if t == nil {
			t = newTile(p.u, g)
			p.tiles[g.ID] = t
		} else {
			t.set(g)
		}
		// As tall as a tile needs (it depends on the theme's text size), so
		// rows never overlap.
		cell.Height = max(cell.Height, t.MinSize().Height)
		tiles = append(tiles, t)
	}
	grid := kit.GridWrap(cell, 12, tiles...)
	for id := range p.tiles {
		if _, ok := p.u.Store().Game(id); !ok {
			delete(p.tiles, id)
		}
	}
	p.gridHost.Objects = []fyne.CanvasObject{grid}
	p.gridHost.Refresh()
	p.relayout()
}
