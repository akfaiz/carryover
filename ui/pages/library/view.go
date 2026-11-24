package library

import (
	"github.com/egoist/mygo/ui"

	"github.com/akfaiz/carryover/services"
	"github.com/akfaiz/carryover/ui/components"
	"github.com/akfaiz/carryover/ui/env"
	"github.com/akfaiz/carryover/ui/flows"
	"github.com/akfaiz/carryover/ui/i18n"
	"github.com/akfaiz/carryover/ui/kit"
	"github.com/akfaiz/carryover/ui/store"
)

// ShowAttention sets the library's filter to the games that need attention,
// for the next time the page opens.
func ShowAttention(u env.Env) {
	NewViewModel(StoreSource(u.Store()), u.Prefs()).SetFilter(FilterAttention)
}

// Page is the library page. MyGo draws it again every frame: View reads the
// ViewModel (whose state is the shared store and the user's choices) and
// declares the controls.
type Page struct {
	u  env.Env
	vm *ViewModel

	search string // bound to the search field
	scroll ui.ScrollState
	// playing: games whose Play was pressed and has not answered yet.
	playing map[string]bool

	sorts *kit.Choice

	// gridWin and gridW are the window's and the grid's width as of the last frame.
	gridWin, gridW float32
}

// New returns the library page.
func New(u env.Env) *Page {
	p := &Page{u: u, vm: NewViewModel(StoreSource(u.Store()), u.Prefs()), playing: map[string]bool{}}
	p.sorts = &kit.Choice{}
	p.sorts.Add(string(SortName), i18n.L("Sort by name"))
	p.sorts.Add(string(SortRecent), i18n.L("Sort by last sync"))
	return p
}

// Scroll is the page's scroll container, which the pad's right stick scrolls.
func (p *Page) Scroll() *ui.ScrollState { return &p.scroll }

// FocusSearch puts the cursor in the search field (Ctrl+F, the pad's X).
func (p *Page) FocusSearch() { p.u.Navigator().Handle("search").Focus() }

// SyncNow starts a Sync all (F5, the pad's Y).
func (p *Page) SyncNow() {
	if p.vm.CanSyncNow() {
		p.u.SyncAll()
	}
}

// StateChanged: the page reads the store when it draws, so a redraw is all it
// needs; kit.OnUI asked for one already.
func (p *Page) StateChanged(store.Topic) {}

// DefaultFocus starts a gamepad on the first game.
func (p *Page) DefaultFocus() *ui.Handle {
	if games := p.vm.Visible(); len(games) > 0 {
		return p.u.Navigator().Handle([2]any{"open", games[0].ID})
	}
	return nil
}

// View draws the page.
func (p *Page) View(c *ui.Context) {
	u, nav := p.u, p.u.Navigator()
	p.vm.SetQuery(p.search)
	kit.Page(c, kit.PageFull, &p.scroll, func() { // the grid fills the window
		kit.PageHead(c, nav, nil, "", i18n.L("Library"), func() {
			p.syncButton(c)
			kit.Btn(c, nav, "add", kit.BtnPrimary, kit.IconPlus, i18n.L("Add game"), u.AddGame)
		})
		p.progress(c)
		if p.vm.HasGames() {
			p.tools(c)
		}
		p.empty(c)
		p.grid(c)
	})
}

func (p *Page) syncButton(c *ui.Context) {
	nav := p.u.Navigator()
	state, enabled := p.vm.SyncButton()
	label := i18n.L("Sync all")
	switch state {
	case SyncCancelling:
		label = i18n.L("Cancelling…")
	case SyncRunning:
		label = i18n.L("Cancel sync")
	}
	kit.Btn(c, nav, "sync", kit.BtnSecondary, kit.IconRefresh, label, p.u.SyncAll, kit.BtnDisabled(!enabled))
}

// progress shows how many games "Sync all" has finished.
func (p *Page) progress(c *ui.Context) {
	pr, running := p.vm.Progress()
	if !running {
		return
	}
	ui.Column(c).Gap(kit.S1).Children(func() {
		text := i18n.L("Synced {{.Done}} of {{.Total}}", map[string]any{"Done": pr.Done, "Total": pr.Total})
		if pr.Name != "" {
			text += " · " + pr.Name
		}
		kit.Small(c, text)
		if pr.Total > 0 {
			ui.Progress(c, float64(pr.Done)/float64(pr.Total))
		} else {
			ui.Progress(c, -1)
		}
	})
}

// tools is search and sort on one row, the filters on their own row under it.
func (p *Page) tools(c *ui.Context) {
	nav := p.u.Navigator()
	ui.Column(c).Gap(kit.S3).Children(func() {
		ui.Row(c).Gap(kit.S3).AlignItems(ui.Center).Children(func() {
			kit.SearchField(c, nav, "search", &p.search, i18n.L("Search…"), i18n.L("Clear search")).Width(360)
			kit.SelectOf(c, nav, "sort", i18n.L("Sort"), p.sorts, string(p.vm.Sort()), func(v string) { p.vm.SetSort(Sort(v)) })
		})
		if p.vm.ShowFilters() {
			n := p.vm.Counts()
			kit.Chips(c, nav, "filter", []kit.ChipOption{
				{Value: string(FilterAll), Label: i18n.L("All ({{.Count}})", map[string]any{"Count": n.All})},
				{Value: string(FilterReady), Label: i18n.L("Ready to play ({{.Count}})", map[string]any{"Count": n.Ready})},
				{Value: string(FilterAttention), Label: i18n.L("Needs attention ({{.Count}})", map[string]any{"Count": n.Attention}), Warn: n.Attention > 0},
			}, string(p.vm.Filter()), func(v string) { p.vm.SetFilter(Filter(v)) })
		}
	})
}

// showAll is the "Show all games" button of the empty states that come from a
// filter.
func (p *Page) showAll(c *ui.Context) {
	nav := p.u.Navigator()
	kit.Btn(c, nav, "show-all", kit.BtnSecondary, nil, i18n.L("Show all games"), func() { p.vm.SetFilter(FilterAll) }).AlignSelf(ui.Start)
}

func (p *Page) empty(c *ui.Context) {
	u, nav := p.u, p.u.Navigator()
	switch p.vm.Empty() {
	case EmptyNoGames:
		kit.EmptyState(c, kit.IconLibrary, i18n.L("No games yet"),
			i18n.L("Add a game, tell Carryover where its saves live on this device, and it will keep them in sync through your cloud storage every time you play — from here or from Steam."),
			func() {
				kit.Btn(c, nav, "add-first", kit.BtnPrimary, kit.IconPlus, i18n.L("Add your first game"), u.AddGame)
				if !p.vm.src.HasRemote() {
					kit.Btn(c, nav, "setup-cloud", kit.BtnSecondary, kit.IconCloud, i18n.L("Set up cloud storage"), func() { u.Navigate("/cloud") })
				}
			})
	case EmptyNoMatch:
		kit.Card(c, "", func() {
			kit.Muted(c, i18n.L("No games match “{{.Query}}”.", map[string]any{"Query": p.vm.Query()}))
			kit.Btn(c, nav, "clear-search", kit.BtnSecondary, nil, i18n.L("Clear search"), func() { p.search = "" }).AlignSelf(ui.Start)
		})
	case EmptyNoReady:
		kit.Card(c, "", func() {
			kit.Muted(c, i18n.L("No game can be played on this device yet. Set one up to play it here."))
			p.showAll(c)
		})
	case EmptyNoAttention:
		kit.Card(c, "", func() {
			kit.Muted(c, i18n.L("Nothing needs attention: every game is set up and synced."))
			p.showAll(c)
		})
	}
}

func (p *Page) grid(c *ui.Context) {
	games := p.vm.Visible()
	if len(games) == 0 {
		return
	}
	// The grid takes the width it is given: its columns are counted from the
	// width it had in the last frame, and another frame is asked for when that
	// changed (the window was resized, a scroll bar came or went), so the
	// layout settles within a frame or two.
	box := ui.Column(c.Key("library-grid"))
	width := box.Bounds().W
	if win, _ := c.Size(); win != p.gridWin || width != p.gridW {
		p.gridWin, p.gridW = win, width
		c.AnimationFrame()
	}
	if width <= 0 {
		win, _ := c.Size()
		width = win - gridChrome // the first frame: the window less the sidebar and the page's margins
	}
	cols := GridColumns(width, kit.S4)
	box.Children(func() {
		ui.Grid(c).Columns(cols).Gap(kit.S4).AlignItems(ui.Start).Children(func() {
			for _, g := range games {
				p.tile(c, g)
			}
		})
	})
}

// gridChrome is about what the window has besides the grid: the sidebar and
// the page's margins. It only sizes the first frame, before the grid has a
// width of its own to count columns from.
const gridChrome float32 = 224 + 1 + 2*kit.S8

// tile is a library grid entry: cover, name and sync status (press to open the
// game) and a play (or "Set up") button.
func (p *Page) tile(c *ui.Context, g services.GameView) {
	u, nav := p.u, p.u.Navigator()
	g.Status = g.Overall // the tile shows the game as a whole: its own saves and all its emulators
	ui.Column(c.Key([2]any{"tile", g.ID})).MaxWidth(TileMax).Gap(kit.S2).Children(func() {
		open := func() { u.Navigate("/game/" + g.ID) }
		k := [2]any{"open", g.ID}
		t := c.Theme()
		// Long names get a second line rather than a tooltip: there is no hover
		// on a Steam Deck.
		b := ui.ButtonBase(c.Key(k)).Label(g.Name)
		bg, ring := ui.Color{}, ui.Color{}
		if b.Hovered() {
			bg, ring = t.SurfaceHover, t.Accent.Alpha(0.6)
		}
		b = b.Radius(kit.RadiusLG).Padding(kit.S2).Background(bg).Border(2, ring).Children(func() {
			ui.Column(c).FillWidth().Gap(kit.S2).Children(func() {
				kit.CoverFill(c, kit.CoverSource{Path: g.CoverPath, Version: g.CoverVersion, Name: g.Name}, CoverRatio)
				// Always two lines tall, so the Play buttons of a row line up.
				ui.Text(c, g.Name).FontWeight(600).Wrap().MaxLines(2).
					FixedLineHeight(titleLine).Height(2 * titleLine)
				ui.Row(c).Gap(kit.S2).AlignItems(ui.Center).Children(func() {
					components.StatusBadge(c, g)
					// The badge already says "not synced": no "never" after it.
					if g.Ready && g.Status.LastSyncAt != "" {
						kit.Small(c, components.TimeAgo(g.Status.LastSyncAt))
					}
				})
			})
		})
		if nav.Item(nav.Handle(k), b, open).Clicked() {
			open()
		}
		p.play(c, g)
	})
}

// titleLine is the height of one line of a tile's name.
const titleLine float32 = 20

// play is the one Play button of a game.
func (p *Page) play(c *ui.Context, g services.GameView) {
	nav := p.u.Navigator()
	label, enabled, kind, icon := i18n.L("Play"), !p.playing[g.ID], kit.BtnPrimary, kit.IconPlay
	switch {
	case g.Status.Running:
		label, enabled, kind, icon = i18n.L("Running"), false, kit.BtnSecondary, kit.IconDot
	case !g.Ready:
		label, kind, icon = i18n.L("Set up"), kit.BtnSecondary, kit.IconPencil
	case g.Device.LaunchMode == "steamwrap":
		// Steam starts it (and Carryover through its launch options).
		label, kind, enabled = i18n.L("Play in Steam"), kit.BtnSecondary, services.SteamRunURL(g) != ""
	}
	kit.Btn(c, nav, [2]any{"play", g.ID}, kind, icon, label, func() { p.playTapped(g) }, kit.BtnDisabled(!enabled)).
		AlignSelf(ui.Stretch).Margin(0, kit.S2, kit.S2, kit.S2)
}

func (p *Page) playTapped(g services.GameView) {
	u := p.u
	switch {
	case !g.Ready:
		u.Navigate("/edit/" + g.ID)
	case g.Device.LaunchMode == "steamwrap":
		flows.OpenSteam(u, g)
	default:
		p.playing[g.ID] = true
		// A game that can be played in several ways asks which one in a dialog.
		kit.Async0(func() error { return u.Sess().Play(g.ID) }, func(err error) {
			delete(p.playing, g.ID)
			if err != nil {
				u.ToastErr(err)
				return
			}
			u.Navigate("/game/" + g.ID)
		})
	}
}
