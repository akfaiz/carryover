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
	ui.Scroll(c).Grow(1).TrackScroll(&p.scroll).Children(func() {
		ui.Column(c).Gap(0).Padding(0, 20, 20, 20).Children(func() {
			kit.PageHead(c, nav, nil, nil, i18n.L("Library"), func() {
				p.syncButton(c)
				add := u.AddGame
				if nav.Item(nav.Handle("add"), ui.PrimaryButton(c.Key("add"), i18n.L("＋ Add game")), add).Clicked() {
					add()
				}
			})
			p.progress(c)
			if p.vm.HasGames() {
				p.tools(c)
			}
			p.empty(c)
			p.grid(c)
		})
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
	sync := p.u.SyncAll
	if nav.Item(nav.Handle("sync"), ui.Button(c.Key("sync"), label).Disabled(!enabled), sync).Clicked() {
		sync()
	}
}

// progress shows how many games "Sync all" has finished.
func (p *Page) progress(c *ui.Context) {
	pr, running := p.vm.Progress()
	if !running {
		return
	}
	ui.Column(c).Gap(4).Padding(0, 0, 12, 0).Children(func() {
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
	ui.Column(c).Gap(8).Padding(0, 0, 8, 0).Children(func() {
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			in := ui.TextInput(c.Key("search"), &p.search).Placeholder(i18n.L("Search…")).Label(i18n.L("Search…")).Width(360)
			nav.Item(nav.Handle("search"), in, nil, kit.TextField)
			kit.SelectOf(c, nav, p.u.Dialogs(), "sort", i18n.L("Sort"), p.sorts, string(p.vm.Sort()), func(v string) { p.vm.SetSort(Sort(v)) })
		})
		if p.vm.ShowFilters() {
			n := p.vm.Counts()
			kit.Chips(c, nav, "filter", []kit.ChipOption{
				{Value: string(FilterAll), Label: i18n.L("All ({{.Count}})", map[string]any{"Count": n.All})},
				{Value: string(FilterReady), Label: i18n.L("Ready to play ({{.Count}})", map[string]any{"Count": n.Ready})},
				{Value: string(FilterAttention), Label: i18n.L("Needs attention ({{.Count}})", map[string]any{"Count": n.Attention})},
			}, string(p.vm.Filter()), func(v string) { p.vm.SetFilter(Filter(v)) })
		}
	})
}

// showAll is the "Show all games" button of the empty states that come from a
// filter.
func (p *Page) showAll(c *ui.Context) {
	nav := p.u.Navigator()
	show := func() { p.vm.SetFilter(FilterAll) }
	if nav.Item(nav.Handle("show-all"), ui.Button(c.Key("show-all"), i18n.L("Show all games")), show).Clicked() {
		show()
	}
}

func (p *Page) empty(c *ui.Context) {
	u, nav := p.u, p.u.Navigator()
	switch p.vm.Empty() {
	case EmptyNoGames:
		kit.Card(c, i18n.L("No games yet"), func() {
			kit.Muted(c, i18n.L("Add a game, tell Carryover where its saves live on this device, and it will keep them in sync through your cloud storage every time you play — from here or from Steam."))
			ui.Row(c).Gap(8).Children(func() {
				add := u.AddGame
				if nav.Item(nav.Handle("add-first"), ui.PrimaryButton(c.Key("add-first"), i18n.L("＋ Add your first game")), add).Clicked() {
					add()
				}
				if !p.vm.src.HasRemote() {
					cloud := func() { u.Navigate("/cloud") }
					if nav.Item(nav.Handle("setup-cloud"), ui.Button(c.Key("setup-cloud"), i18n.L("Set up cloud storage")), cloud).Clicked() {
						cloud()
					}
				}
			})
		})
	case EmptyNoMatch:
		kit.Card(c, "", func() {
			kit.Muted(c, i18n.L("No games match “{{.Query}}”.", map[string]any{"Query": p.vm.Query()}))
			clear := func() { p.search = "" }
			if nav.Item(nav.Handle("clear-search"), ui.Button(c.Key("clear-search"), i18n.L("Clear search")), clear).Clicked() {
				clear()
			}
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
	ui.Row(c).Wrap().Gap(12).Children(func() {
		for _, g := range games {
			p.tile(c, g)
		}
	})
}

// tile is a library grid entry: cover, name and sync status (press to open the
// game) and a play (or "Set up") button.
func (p *Page) tile(c *ui.Context, g services.GameView) {
	u, nav := p.u, p.u.Navigator()
	g.Status = g.Overall // the tile shows the game as a whole: its own saves and all its emulators
	ui.Column(c.Key([2]any{"tile", g.ID})).Width(170).Gap(6).Children(func() {
		open := func() { u.Navigate("/game/" + g.ID) }
		k := [2]any{"open", g.ID}
		// Long names get a second line rather than a tooltip: there is no hover
		// on a Steam Deck.
		b := ui.ButtonBase(c.Key(k)).Label(g.Name).Radius(8).Children(func() {
			ui.Column(c).Gap(6).Children(func() {
				kit.CoverView(c, kit.CoverSource{Path: g.CoverPath, Version: g.CoverVersion, Name: g.Name}, 160, 240)
				ui.Text(c, g.Name).Bold().Wrap().MaxLines(2)
				ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
					components.StatusBadge(c, g)
					when := ""
					if g.Ready {
						when = components.TimeAgo(g.Status.LastSyncAt)
					}
					kit.Small(c, when)
				})
			})
		})
		if nav.Item(nav.Handle(k), b, open).Clicked() {
			open()
		}
		p.play(c, g)
	})
}

// play is the one Play button of a game.
func (p *Page) play(c *ui.Context, g services.GameView) {
	nav := p.u.Navigator()
	label, enabled, primary := i18n.L("Play"), !p.playing[g.ID], true
	switch {
	case g.Status.Running:
		label, enabled, primary = i18n.L("Running"), false, false
	case !g.Ready:
		label, primary = i18n.L("Set up"), false
	case g.Device.LaunchMode == "steamwrap":
		// Steam starts it (and Carryover through its launch options).
		label, primary, enabled = i18n.L("Play in Steam"), false, services.SteamRunURL(g) != ""
	}
	k := [2]any{"play", g.ID}
	var b ui.Element
	if primary {
		b = ui.PrimaryButton(c.Key(k), label)
	} else {
		b = ui.Button(c.Key(k), label)
	}
	b = b.Disabled(!enabled).AlignSelf(ui.Stretch)
	action := func() { p.playTapped(g) }
	if nav.Item(nav.Handle(k), b, action).Clicked() {
		action()
	}
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
