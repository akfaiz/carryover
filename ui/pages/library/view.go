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

	// games is the grid's place: it builds only the rows in view, and scrolls
	// the rows (the page's scroll state follows it).
	games ui.GridState
	// tile0 is bound to the tiles as they are built, so that the width one had
	// in the last frame is known; tileW is that width as of the frame before.
	tile0  ui.Handle
	tileW  float32
	probed bool // tile0 is bound in this build
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

// View draws the page. With games to show, the title, search and filters stay
// at the top and the grid below them scrolls, building only the rows in view:
// a page that built every tile took milliseconds a frame for hundreds of
// games. Without any, the page is a plain scrolling page with what to do.
func (p *Page) View(c *ui.Context) {
	p.vm.SetQuery(p.search)
	if games := p.vm.Visible(); len(games) > 0 {
		ui.Column(c).Grow(1).Children(func() {
			ui.Row(c).Justify(ui.Center).Children(func() {
				ui.Column(c).Grow(1).Shrink(1).Padding(kit.S6, kit.S8, p.gridTop(c), kit.S8).Gap(kit.S4).MaxWidth(kit.PageFull).Children(func() {
					p.head(c)
				})
			})
			p.grid(c, games)
		})
		return
	}
	kit.Page(c, kit.PageFull, &p.scroll, func() { // the grid fills the window
		p.head(c)
		p.empty(c)
	})
}

// head is the title and its actions, the progress of Sync all, search and the
// filters.
func (p *Page) head(c *ui.Context) {
	u, nav := p.u, p.u.Navigator()
	kit.PageHead(c, nav, nil, "", i18n.L("Library"), func() {
		p.syncButton(c)
		kit.Btn(c, nav, "add", kit.BtnPrimary, kit.IconPlus, i18n.L("Add game"), u.AddGame)
	})
	p.progress(c)
	if p.vm.HasGames() {
		p.tools(c)
	}
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

// gridTop is what the title and the filters leave below them, for the first row
// of tiles to be kit.S4 under the last of them.
func (p *Page) gridTop(c *ui.Context) float32 {
	return max(0, kit.S4-rowInset(c.Theme().Space(2)))
}

// grid draws the games as tiles in a ui.GridView, which builds only the rows in
// view. The rows are all one height, which the tiles' is (a cover with a ratio,
// and what is under it), so the height comes from the width a tile had in the
// last frame; another frame is asked for when it changed (the window was
// resized, a scroll bar came or went), so the layout settles within a frame or
// two.
func (p *Page) grid(c *ui.Context, games []services.GameView) {
	gap := c.Theme().Space(2)
	extra := cellExtra(gap)
	side := kit.S8 - extra/2
	width := p.tile0.Bounds(c).W
	if width <= 0 {
		// The first frame: the window less the sidebar and the page's margins.
		win, _ := c.Size()
		width = TileWidth(win-gridChrome+extra, gap)
	}
	if width != p.tileW {
		p.tileW = width
		c.AnimationFrame()
	}
	p.games.Key = func(i int) any { return games[i].ID }
	p.probed = false
	ui.GridView(c, &p.games, len(games), TileMin+extra, TileHeight(width)+extra, func(i int) {
		p.tile(c, games[i])
	}).Grow(1).Padding(0, side, max(0, kit.S6-rowInset(gap)), side).TrackScroll(&p.scroll)
}

// gridChrome is about what the window has besides the grid: the sidebar and
// the page's margins. It only sizes the first frame, before a tile has a width
// of its own to take the height of a row from.
const gridChrome float32 = 224 + 1 + 2*kit.S8

// tile is a library grid entry: cover, name and sync status (press to open the
// game) and a play (or "Set up") button.
func (p *Page) tile(c *ui.Context, g services.GameView) {
	u, nav := p.u, p.u.Navigator()
	g.Status = g.Overall // the tile shows the game as a whole: its own saves and all its emulators
	col := ui.Column(c.Key([2]any{"tile", g.ID})).MaxWidth(TileMax).Margin(cellMargin(c.Theme().Space(2))).Gap(kit.S2)
	if !p.probed { // one tile tells the width of them all
		p.probed = true
		col = col.Bind(&p.tile0)
	}
	k := [2]any{"open", g.ID}
	col.Children(func() {
		open := func() { u.Navigate("/game/" + g.ID) }
		t := c.Theme()
		// Long names get a second line rather than a tooltip: there is no hover
		// on a Steam Deck.
		// The hovered look is painted in Draw, not chosen here: reading Hovered as
		// the view builds would build the whole grid anew each time the pointer
		// crosses a tile.
		b := ui.ButtonBase(c.Key(k)).Label(g.Name)
		b = b.Radius(kit.RadiusLG).Padding(kit.S2).Border(2, ui.Color{}).Draw(func(p *ui.Painter, r ui.Rect) {
			if kit.Hovered(b) {
				p.Fill(r, t.SurfaceHover, kit.RadiusLG)
				p.Stroke(r, t.Accent.Alpha(0.6), kit.RadiusLG, 2)
			}
		}).Children(func() {
			ui.Column(c).FillWidth().Gap(kit.S2).Children(func() {
				kit.CoverFill(c, kit.CoverSource{Path: g.CoverPath, Version: g.CoverVersion, Name: g.Name}, CoverRatio)
				// Always two lines tall, so the Play buttons of a row line up.
				ui.Text(c, g.Name).FontWeight(600).Wrap().MaxLines(2).
					FixedLineHeight(titleLine).Height(2 * titleLine)
				ui.Row(c).Gap(kit.S2).AlignItems(ui.Center).Children(func() {
					components.StatusBadge(c, g)
					// The badge already says "not synced": no "never" after it.
					if g.Ready && g.Status.LastSyncAt != "" {
						// One line, so that a narrow tile cuts it rather than making the row taller than the grid's.
						kit.Small(c, components.TimeAgo(g.Status.LastSyncAt)).SingleLine()
					}
				})
			})
		})
		if nav.Item(nav.Handle(k), b, open).Clicked() {
			open()
		}
		p.play(c, g)
	})
	nav.PadLift(c, col, nav.Handle(k), kit.BtnHandle(nav, [2]any{"play", g.ID}))
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
