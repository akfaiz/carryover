package editor

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/akfaiz/carryover/services"
	"github.com/akfaiz/carryover/ui/env"
	"github.com/akfaiz/carryover/ui/i18n"
	"github.com/akfaiz/carryover/ui/kit"
)

type artSpec struct {
	kind    services.ArtKind
	label   string
	hint    string
	empty   string     // shown when SteamGridDB has none
	preview [2]float32 // shown at this size (w, h), which keeps the kind's proportions
	pick    [2]float32 // thumbnail size in the picker
	contain bool       // show the whole image (logos and icons), not a cropped fill
}

// artSpecs are the kinds of Steam artwork besides the portrait cover. Text goes
// through i18n, so this is a function, not a variable.
func artSpecs() []artSpec {
	return []artSpec{
		{services.ArtWide, i18n.L("Wide cover"), i18n.L("Wide capsule, 460×215. Shown in Steam's recent games and some lists."),
			i18n.L("No wide covers for this game yet."), [2]float32{230, 107}, [2]float32{184, 86}, false},
		{services.ArtHero, i18n.L("Background"), i18n.L("Library background (hero), 1920×620. Shown at the top of the game's page in Steam."),
			i18n.L("No backgrounds for this game yet."), [2]float32{310, 100}, [2]float32{248, 80}, false},
		{services.ArtLogo, i18n.L("Logo"), i18n.L("Transparent title logo drawn over the background."),
			i18n.L("No logos for this game yet."), [2]float32{240, 90}, [2]float32{160, 90}, true},
		{services.ArtIcon, i18n.L("Icon"), i18n.L("Small square icon for the game in Steam's library list."),
			i18n.L("No icons for this game yet."), [2]float32{80, 80}, [2]float32{80, 80}, true},
	}
}

func artSpecFor(kind services.ArtKind) artSpec {
	for _, s := range artSpecs() {
		if s.kind == kind {
			return s
		}
	}
	return artSpec{kind: services.ArtCover, label: i18n.L("Cover"), empty: i18n.L("No portrait covers for this game yet."),
		preview: [2]float32{80, 120}, pick: [2]float32{120, 180}}
}

func (e *Page) openArtPicker(kind services.ArtKind) {
	showArtPicker(e.u, artPickerOpts{
		be:   e.vm.be,
		term: e.vm.name, steamAppID: e.vm.storeAppID, gameID: e.vm.gridGameID, gameName: e.vm.gridGameName, kind: kind,
		onGame: func(id int, name string) { e.vm.gridGameID, e.vm.gridGameName = id, name },
		onPick: func(k services.ArtKind, g services.ArtGrid) {
			if k == services.ArtCover {
				e.vm.gridPick, e.vm.coverSrc = &g, ""
				return
			}
			a := e.vm.artEdit(k)
			a.pick, a.src, a.clear = &g, "", false
		},
	})
}

func (e *Page) artworkCard(c *ui.Context) {
	kit.Card(c, "", func() {
		kit.Heading(c, i18n.L("Steam artwork"))
		kit.Muted(c, i18n.L("Written to Steam's grid folder when you add the game to Steam (or press Update Steam shortcut), so it has its images ready. The portrait cover is chosen above."))
		for _, spec := range artSpecs() {
			a := e.vm.artEdit(spec.kind)
			src := kit.CoverSource{Contain: spec.contain}
			switch {
			case a.clear:
			case a.src != "":
				src.Path = a.src
			case a.pick != nil:
				src.URL = a.pick.Thumb
				if src.URL == "" {
					src.URL = a.pick.URL
				}
			case a.path != "":
				src.Path, src.Version = a.path, a.version
			}
			status := spec.hint
			switch {
			case a.clear:
				status = i18n.L("Removed when you save.")
			case a.src != "":
				status = i18n.L("{{.File}} — copied when you save.", map[string]any{"File": filepath.Base(a.src)})
			case a.pick != nil:
				status = i18n.L("SteamGridDB image — downloaded when you save.")
			}
			ui.Divider(c)
			ui.Column(c).Gap(6).Children(func() {
				ui.Text(c, spec.label).Bold()
				kit.CoverView(c, src, spec.preview[0], spec.preview[1])
				ui.Row(c).Gap(8).Children(func() {
					e.button(c, [2]any{"art-file", spec.kind}, i18n.L("Choose file…"), false, false, func() {
						e.pickFile(i18n.L("Choose image"), []string{".png", ".jpg", ".jpeg", ".webp"}, "", func(p string) {
							a.src, a.pick, a.clear = p, nil, false
						})
					})
					if e.vm.hasGridKey {
						e.button(c, [2]any{"art-find", spec.kind}, i18n.L("Find…"), false, false, func() { e.openArtPicker(spec.kind) })
					}
					if a.has() {
						e.button(c, [2]any{"art-clear", spec.kind}, i18n.L("Clear"), false, false, func() {
							a.src, a.pick, a.clear = "", nil, a.path != ""
						})
					}
				})
				kit.Muted(c, status)
			})
		}
		if !e.vm.hasGridKey {
			ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
				kit.Muted(c, i18n.L("Add a SteamGridDB API key to search community artwork.")).Grow(1)
				e.button(c, "art-key", i18n.L("Add key…"), false, false, e.askGridKey)
			})
		}
	})
}

// artPicker is the dialog that searches SteamGridDB for a game and shows its
// artwork by kind; picking an image tells the editor at once, and Done closes.
type artPicker struct {
	u          env.Env
	be         Backend
	steamAppID string
	start      services.ArtKind
	onPick     func(services.ArtKind, services.ArtGrid)
	onGame     func(id int, name string) // remembers the chosen game for next time

	d         *kit.Dialog
	query     string
	errText   string
	games     []services.ArtGame // last search results; nil until searched
	searching bool

	gen      int
	inGame   bool // a game is open (else the list of games shows)
	gameID   int
	gameName string
	kind     services.ArtKind // the tab shown
	grids    map[services.ArtKind][]services.ArtGrid
	loaded   map[services.ArtKind]bool
	picked   map[services.ArtKind]int // grid id chosen in this dialog
}

type artPickerOpts struct {
	be         Backend          // where the games and images come from
	term       string           // initial search text (the game's name)
	steamAppID string           // linked Steam app: its art is shown straight away
	gameID     int              // SteamGridDB game chosen earlier (0: none)
	gameName   string           // its name
	kind       services.ArtKind // tab to open first
	onPick     func(services.ArtKind, services.ArtGrid)
	onGame     func(id int, name string)
}

func showArtPicker(u env.Env, o artPickerOpts) {
	p := &artPicker{u: u, be: o.be, steamAppID: o.steamAppID, start: o.kind, kind: o.kind, onPick: o.onPick, onGame: o.onGame,
		query: o.term, picked: map[services.ArtKind]int{}}
	p.d = u.Dialogs().Show(&kit.Dialog{Title: i18n.L("Find artwork on SteamGridDB"), Wide: true, Body: p.body})
	switch {
	case o.gameID > 0:
		p.loadGame(o.gameID, o.gameName)
	case o.steamAppID != "":
		p.loadGame(0, i18n.L("Steam app {{.ID}}", map[string]any{"ID": o.steamAppID}))
	case strings.TrimSpace(o.term) != "":
		p.search()
	}
}

func (p *artPicker) body(c *ui.Context, nav *kit.Navigator) {
	ui.Row(c).Gap(8).Children(func() {
		nav.Item(nav.Handle("art-query"), ui.TextInput(c.Key("art-query"), &p.query).Placeholder(i18n.L("Game name")).Label(i18n.L("Game name")).Grow(1), nil, kit.TextField)
		p.button(c, nav, "art-search", i18n.L("Search"), false, p.search)
	})
	if p.errText != "" {
		kit.Danger(c, p.errText)
	}
	ui.Scroll(c).MaxHeight(420).Children(func() {
		ui.Column(c).Gap(6).Children(func() {
			switch {
			case p.inGame:
				p.game(c, nav)
			case p.searching:
				kit.Muted(c, i18n.L("Searching…"))
			case p.games != nil:
				p.gameList(c, nav)
			}
		})
	})
	p.button(c, nav, "art-done", i18n.L("Done"), true, func() { p.d.Close() })
}

func (p *artPicker) button(c *ui.Context, nav *kit.Navigator, key, label string, primary bool, action func()) {
	k := [2]any{"artpicker", key}
	var b ui.Element
	if primary {
		b = ui.PrimaryButton(c.Key(k), label)
	} else {
		b = ui.Button(c.Key(k), label)
	}
	if nav.Item(nav.Handle(k), b, action).Clicked() {
		action()
	}
}

func (p *artPicker) search() {
	if strings.TrimSpace(p.query) == "" {
		return
	}
	p.gen++
	p.errText, p.inGame, p.searching = "", false, true
	term := p.query
	kit.Async(func() ([]services.ArtGame, error) { return p.be.SearchGridGames(term) }, func(list []services.ArtGame, err error) {
		p.searching = false
		if err != nil {
			p.errText = err.Error()
		}
		if list == nil {
			list = []services.ArtGame{}
		}
		p.games = list
	})
}

func (p *artPicker) gameList(c *ui.Context, nav *kit.Navigator) {
	if len(p.games) == 0 {
		kit.Muted(c, i18n.L("No games found."))
	}
	for _, g := range p.games {
		info := ""
		if g.Year > 0 {
			info = fmt.Sprint(g.Year)
		}
		if g.Verified {
			info += " ✓"
		}
		k := [2]any{"art-game", g.ID}
		pick := func() {
			if p.onGame != nil {
				p.onGame(g.ID, g.Name)
			}
			p.loadGame(g.ID, g.Name)
		}
		b := ui.ButtonBase(c.Key(k)).Label(g.Name).Radius(6).Children(func() {
			ui.Row(c).Gap(10).Padding(6, 8).Children(func() {
				ui.Text(c, g.Name).Grow(1)
				kit.Small(c, info)
			})
		})
		if nav.Item(nav.Handle(k), b.AlignSelf(ui.Stretch), pick).Clicked() {
			pick()
		}
	}
}

func (p *artPicker) loadGame(id int, name string) {
	p.errText = ""
	p.gen++
	p.inGame, p.gameID, p.gameName = true, id, name
	p.grids = map[services.ArtKind][]services.ArtGrid{}
	p.loaded = map[services.ArtKind]bool{}
	p.kind = p.start
	p.ensure(p.kind)
}

// game is a game's artwork: tabs by kind, each a grid of images to pick from.
func (p *artPicker) game(c *ui.Context, nav *kit.Navigator) {
	ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
		p.button(c, nav, "art-change", i18n.L("← Change game"), false, func() {
			if p.games == nil && strings.TrimSpace(p.query) != "" {
				p.search()
				return
			}
			p.gen++
			p.inGame = false
		})
		ui.Text(c, p.gameName).Bold()
	})
	var tabs []kit.ChipOption
	for _, k := range append([]services.ArtKind{services.ArtCover}, services.ArtKinds...) {
		tabs = append(tabs, kit.ChipOption{Value: string(k), Label: artSpecFor(k).label})
	}
	kit.Chips(c, nav, "art-tabs", tabs, string(p.kind), func(v string) {
		p.kind = services.ArtKind(v)
		p.ensure(p.kind)
	})
	spec := artSpecFor(p.kind)
	switch grids := p.grids[p.kind]; {
	case !p.done(p.kind):
		kit.Muted(c, i18n.L("Loading…"))
	case len(grids) == 0:
		kit.Muted(c, spec.empty)
	default:
		ui.Row(c).Wrap().Gap(8).Children(func() {
			for _, g := range grids {
				src := g.Thumb
				if src == "" {
					src = g.URL
				}
				k := [3]any{"art-grid", p.kind, g.ID}
				pick := func() { p.picked[p.kind] = g.ID; p.onPick(p.kind, g) }
				border, width := c.Theme().Border, float32(1)
				if p.picked[p.kind] == g.ID {
					border, width = c.Theme().Accent, 3
				}
				b := ui.ButtonBase(c.Key(k)).Label(spec.label).Radius(8).Border(width, border).Padding(4).Children(func() {
					kit.CoverView(c, kit.CoverSource{URL: src, Contain: spec.contain}, spec.pick[0], spec.pick[1])
				})
				if nav.Item(nav.Handle(k), b, pick).Clicked() {
					pick()
				}
			}
		})
	}
	kit.Muted(c, i18n.L("Pick an image in each tab you want, then press Done. Art by the SteamGridDB community."))
}

// done reports whether the images of a kind were read (even if there were none).
func (p *artPicker) done(kind services.ArtKind) bool { _, ok := p.grids[kind]; return ok }

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
		if err != nil {
			p.errText = err.Error()
		}
		if grids == nil {
			grids = []services.ArtGrid{}
		}
		p.grids[kind] = grids
	})
}
