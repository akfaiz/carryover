package ui

import (
	"slices"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/lang"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/akfaiz/carryover/internal/ui/kit"
	"github.com/akfaiz/carryover/services"
)

// Library filters and sort orders (kept in the app's preferences).
const (
	filterAll       = "all"
	filterAttention = "attention"

	sortName   = "name"
	sortRecent = "recent"

	prefLibraryFilter = "library.filter"
	prefLibrarySort   = "library.sort"
)

// libraryFilter is the library's filter, as last chosen.
func (u *UI) libraryFilter() string {
	return u.app.Preferences().StringWithFallback(prefLibraryFilter, filterAll)
}

func (u *UI) setLibraryFilter(f string) { u.app.Preferences().SetString(prefLibraryFilter, f) }

func (u *UI) librarySort() string {
	return u.app.Preferences().StringWithFallback(prefLibrarySort, sortName)
}

type libraryPage struct {
	u       *UI
	filter  string // search text
	scroll  *container.Scroll
	body    *fyne.Container // the page's rows, laid out again when one changes height
	search  *widget.Entry
	syncBtn *widget.Button
	head    fyne.CanvasObject // header row, refreshed when syncBtn's label changes
	// chipsHost holds toolsRow (filters, search and sort) once there are games;
	// filterHost is the part of it with the filter buttons, the only part
	// drawn again (the search field stays the same widget while typing).
	chipsHost  *fyne.Container
	toolsRow   fyne.CanvasObject
	filterHost *fyne.Container
	gridHost   *fyne.Container
	emptyHost  *fyne.Container
	barHost    *fyne.Container // "Sync all" progress, shown only while syncing
	bar        *widget.ProgressBar
	barLabel   *widget.Label
	tiles      map[string]*gameTile // by game id, kept across renders
}

func newLibraryPage(u *UI) *libraryPage {
	return &libraryPage{u: u, tiles: map[string]*gameTile{}}
}

func (p *libraryPage) view() fyne.CanvasObject {
	p.search = widget.NewEntry()
	p.search.SetPlaceHolder(lang.L("Search…"))
	p.search.OnChanged = func(s string) { p.filter = s; p.renderGrid() }
	p.syncBtn = widget.NewButtonWithIcon(lang.L("Sync all"), theme.ViewRefreshIcon(), p.u.syncAll)
	p.head = kit.PageHead(nil, lang.L("Library"),
		p.syncBtn,
		kit.Primary(lang.L("＋ Add game"), func() { showAddGame(p.u) }),
	)
	// Search first, over the games it filters, then the filters; sort out of
	// the way on the right. All one height.
	sorts := &kit.Choice{}
	sorts.Add(sortName, lang.L("Sort by name"))
	sorts.Add(sortRecent, lang.L("Sort by last sync"))
	sortSel := kit.SelectOf(sorts, p.u.librarySort(), func(v string) {
		p.u.app.Preferences().SetString(prefLibrarySort, v)
		p.renderGrid()
	})
	p.filterHost = container.NewStack()
	p.toolsRow = container.NewHBox(kit.FixedWidth(280, p.search), p.filterHost, layout.NewSpacer(), kit.FixedWidth(190, sortSel))
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
func (p *libraryPage) relayout() {
	if p.body == nil {
		return
	}
	p.body.Refresh()
	p.scroll.Refresh()
}

func (p *libraryPage) focusSearch() { p.u.win.Canvas().Focus(p.search) }

func (p *libraryPage) syncNow() {
	if !p.u.syncingAll && p.u.settings.Remote != "" {
		p.u.syncAll()
	}
}

// updateProgress shows how many games "Sync all" has finished, and turns its
// button into Cancel while it runs.
func (p *libraryPage) updateProgress() {
	u := p.u
	switch {
	case u.cancellingAll:
		p.syncBtn.SetText(lang.L("Cancelling…"))
		p.syncBtn.SetIcon(theme.CancelIcon())
		p.syncBtn.Disable()
	case u.syncingAll:
		p.syncBtn.SetText(lang.L("Cancel sync"))
		p.syncBtn.SetIcon(theme.CancelIcon())
		p.syncBtn.Enable()
	default:
		p.syncBtn.SetText(lang.L("Sync all"))
		p.syncBtn.SetIcon(theme.ViewRefreshIcon())
		kit.SetEnabled(p.syncBtn, u.settings.Remote != "")
	}
	if p.head != nil {
		p.head.Refresh() // the button's label width changed
	}
	pr := u.syncAllProg
	if !u.syncingAll || pr.Total == 0 {
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

func (p *libraryPage) stateChanged(what change) {
	if what == changeProgress {
		p.updateProgress()
		return
	}
	p.updateProgress() // the remote may have been set up
	p.renderGrid()
}

// visible returns the games the search and filter let through, in the chosen
// order.
func (p *libraryPage) visible() []services.GameView {
	u := p.u
	f := strings.ToLower(strings.TrimSpace(p.filter))
	attention := u.libraryFilter() == filterAttention
	var out []services.GameView
	for _, g := range u.games {
		if !strings.Contains(strings.ToLower(g.Name), f) || (attention && !needsAttention(g)) {
			continue
		}
		out = append(out, g)
	}
	if u.librarySort() == sortRecent {
		// Most recently synced first (the times are RFC 3339, so they sort as
		// text); never synced last.
		slices.SortStableFunc(out, func(a, b services.GameView) int {
			return strings.Compare(b.Overall.LastSyncAt, a.Overall.LastSyncAt)
		})
	} else {
		slices.SortStableFunc(out, func(a, b services.GameView) int {
			return strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name))
		})
	}
	return out
}

func (p *libraryPage) renderChips() {
	u := p.u
	if len(u.games) == 0 {
		p.chipsHost.Objects = nil
		p.chipsHost.Refresh()
		return
	}
	n := 0
	for _, g := range u.games {
		if needsAttention(g) {
			n++
		}
	}
	// The filters only show when there is something to filter for (or the
	// attention filter is on, so it can be turned off again).
	p.filterHost.Objects = nil
	if n > 0 || u.libraryFilter() == filterAttention {
		p.filterHost.Objects = []fyne.CanvasObject{kit.Chips([]kit.ChipOption{
			{Value: filterAll, Label: lang.L("All ({{.Count}})", map[string]any{"Count": len(u.games)})},
			{Value: filterAttention, Label: lang.L("Needs attention ({{.Count}})", map[string]any{"Count": n}), Icon: theme.WarningIcon()},
		}, u.libraryFilter(), func(v string) {
			u.setLibraryFilter(v)
			p.renderGrid()
		})}
	}
	p.filterHost.Refresh()
	p.chipsHost.Objects = []fyne.CanvasObject{p.toolsRow}
	p.chipsHost.Refresh()
}

func (p *libraryPage) renderGrid() {
	u := p.u
	p.renderChips()
	games := p.visible()

	p.emptyHost.Objects = nil
	switch {
	case u.loaded && len(u.games) == 0:
		actions := kit.HBox(kit.Primary(lang.L("＋ Add your first game"), func() { showAddGame(u) }))
		if u.settings.Remote == "" {
			actions.Add(widget.NewButtonWithIcon(lang.L("Set up cloud storage"), theme.StorageIcon(), func() { u.navigate("/cloud") }))
		}
		p.emptyHost.Objects = []fyne.CanvasObject{kit.Card(lang.L("No games yet"),
			kit.Muted(lang.L("Add a game, tell Carryover where its saves live on this device, and it will keep them in sync through your cloud storage every time you play — from here or from Steam.")),
			actions)}
	case len(games) == 0 && strings.TrimSpace(p.filter) != "":
		p.emptyHost.Objects = []fyne.CanvasObject{kit.Card("",
			kit.Muted(lang.L("No games match “{{.Query}}”.", map[string]any{"Query": strings.TrimSpace(p.filter)})),
			kit.HBox(widget.NewButton(lang.L("Clear search"), func() { p.search.SetText("") })))}
	case len(games) == 0 && u.loaded:
		p.emptyHost.Objects = []fyne.CanvasObject{kit.Card("",
			kit.Muted(lang.L("Nothing needs attention: every game is set up and synced.")),
			kit.HBox(widget.NewButton(lang.L("Show all games"), func() { u.setLibraryFilter(filterAll); p.renderGrid() })))}
	}
	p.emptyHost.Refresh()

	// Reuse the tiles: only games new to the page get one built.
	grid := container.NewGridWrap(fyne.NewSize(170, 330))
	for _, g := range games {
		t := p.tiles[g.ID]
		if t == nil {
			t = newGameTile(u, g)
			p.tiles[g.ID] = t
		} else {
			t.set(g)
		}
		grid.Add(t)
	}
	for id := range p.tiles {
		if _, ok := u.game(id); !ok {
			delete(p.tiles, id)
		}
	}
	p.gridHost.Objects = []fyne.CanvasObject{grid}
	p.gridHost.Refresh()
	p.relayout()
}
