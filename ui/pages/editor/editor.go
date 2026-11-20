package editor

import (
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/lang"
	"fyne.io/fyne/v2/widget"

	"github.com/akfaiz/carryover/services"
	"github.com/akfaiz/carryover/ui/env"
	"github.com/akfaiz/carryover/ui/flows"
	"github.com/akfaiz/carryover/ui/kit"
)

// Page is the game editor: add a game, or edit one.
type Page struct {
	u  env.Env
	vm *ViewModel

	scroll    *container.Scroll
	root      *fyne.Container // the page: the scrolling form over the bar
	bar       *fyne.Container // the Save / Cancel bar under the scrolling form
	nameEntry *widget.Entry
	exeEntry  *widget.Entry
	advItem   *kit.AccordionSection
	artItem   *kit.AccordionSection
	slotViews map[*slotForm]*slotView
}

// Dirty: the form has unsaved changes (env.Guarded).
func (e *Page) Dirty() bool { return e.vm.Dirty() }

var (
	_ env.Guarded    = (*Page)(nil)
	_ env.Redrawable = (*Page)(nil)
)

// New returns the editor for the game id, or for a new game when id is empty.
// steamAppID pre-fills a new game from the installed Steam library.
func New(u env.Env, id, steamAppID string) *Page {
	return &Page{u: u, slotViews: map[*slotForm]*slotView{}, vm: newViewModel(id, steamAppID, serviceBackend{u})}
}

// NewFromLutris returns the editor for a new game, pre-filled from the game
// Lutris has under this config id.
func NewFromLutris(u env.Env, configID string) *Page {
	e := New(u, "", "")
	e.vm.lutrisID = configID
	return e
}

// NewEmulated returns the editor for a new game that is played through
// emulators: it starts on that kind.
func NewEmulated(u env.Env) *Page {
	e := New(u, "", "")
	e.vm.kind = KindEmulator
	return e
}

// Name is the game name as typed.
func (e *Page) Name() string { return e.vm.name }

// Kind is how the game is played: directly, through emulators, or both.
func (e *Page) Kind() PlayKind { return e.vm.kind }

// EmulatorNames lists the emulators the game is set up with.
func (e *Page) EmulatorNames() []string {
	var out []string
	for _, m := range e.vm.emus {
		out = append(out, m.name)
	}
	return out
}

func (e *Page) View() fyne.CanvasObject {
	e.scroll = kit.ScrollPage(kit.Muted(lang.L("Loading…")))
	e.bar = container.NewStack()
	kit.Async(e.vm.Load, func(l Loaded, err error) {
		e.vm.Apply(l, err)
		e.render()
		if e.vm.id == "" && e.vm.steamAppID != "" {
			e.prefillFromSteam(e.vm.steamAppID)
		}
		if e.vm.id == "" && e.vm.lutrisID != "" {
			e.prefillFromLutris(e.vm.lutrisID)
		}
	})
	// Save and Cancel stay in view under the form, however long it gets.
	e.root = container.NewBorder(nil, e.bar, nil, nil, e.scroll)
	return e.root
}

func (e *Page) prefillFromSteam(appID string) {
	kit.Async(func() ([]services.SteamLibraryGame, error) { return e.vm.be.SteamLibrary(), nil }, func(lib []services.SteamLibraryGame, _ error) {
		for _, a := range lib {
			if a.AppID == appID {
				e.vm.PrefillFromSteam(a)
				e.render()
				e.suggestSaves(true)
				return
			}
		}
	})
}

func (e *Page) prefillFromLutris(configID string) {
	kit.Async(func() ([]services.LutrisLibraryGame, error) { return e.vm.be.LutrisLibrary(), nil }, func(lib []services.LutrisLibraryGame, _ error) {
		for _, g := range lib {
			if g.ConfigID == configID {
				e.vm.PrefillFromLutris(g)
				e.render()
				e.suggestSaves(true)
				return
			}
		}
	})
}

// suggestSaves looks up save locations in the Ludusavi manifest. With
// autofill, an empty first location is filled when exactly one suggestion
// has files on disk.
func (e *Page) suggestSaves(autofill bool) {
	if strings.TrimSpace(e.vm.name) == "" && e.vm.storeAppID == "" {
		return
	}
	e.vm.suggesting = true
	e.render()
	in := services.SuggestInput{GameID: e.vm.id, Name: e.vm.name, StoreAppID: e.vm.storeAppID, Exe: e.vm.dev.Exe, LaunchMode: e.vm.dev.LaunchMode}
	kit.Async(func() (services.SuggestResult, error) { return e.vm.be.SuggestSaves(in) }, func(res services.SuggestResult, err error) {
		e.vm.suggesting = false
		if err != nil {
			e.u.Toast(lang.L("Save suggestions: {{.Err}}", map[string]any{"Err": err}), kit.ToastErr)
			e.render()
			return
		}
		if e.vm.ApplySuggestions(res, autofill) {
			e.u.Toast(lang.L("Found {{.Game}} saves on this device", map[string]any{"Game": res.Matched}), kit.ToastOK)
		}
		e.render()
	})
}

func (e *Page) addSlot() {
	e.vm.addSlot()
	e.render()
}
func (e *Page) searchSteam() {
	term := strings.TrimSpace(e.vm.name)
	if len(term) < 2 {
		return
	}
	e.vm.searching = true
	e.render()
	kit.Async(func() ([]services.StoreMatch, error) { return e.vm.be.SearchSteam(term) }, func(res []services.StoreMatch, err error) {
		e.vm.searching = false
		if err != nil {
			e.u.Toast(lang.L("Steam search failed: {{.Err}}", map[string]any{"Err": err}), kit.ToastErr)
			e.vm.showResults = false
		} else {
			e.vm.steamResults, e.vm.showResults = res, true
		}
		e.render()
	})
}

func (e *Page) pickFile(title string, exts []string, done func(string)) {
	kit.PickFile(e.u.Window(), title, exts, done)
}

func (e *Page) render() {
	if e.vm.loading {
		return
	}
	u := e.u
	// Keep accordion states across re-renders.
	if e.advItem != nil {
		e.vm.advanced = e.advItem.Open()
	}
	if e.artItem != nil {
		e.vm.artOpen = e.artItem.Open()
	}
	for _, s := range e.vm.slots {
		e.slotViewOf(s).rememberOpen()
	}
	head := lang.L("Add game")
	if e.vm.id != "" {
		head = lang.L("Edit game")
	}
	var body fyne.CanvasObject
	if e.vm.id == "" {
		// A new game is filled in from top to bottom, one card after the other;
		// the artwork is optional, so it starts folded.
		col := kit.Stack(e.gameCard(), e.howCard())
		if e.vm.kind != KindEmulator {
			col.Add(e.launchCard())
			col.Add(e.savesCard())
		}
		if e.vm.kind != KindDirect {
			col.Add(e.emulatorsCard())
		}
		e.artItem = kit.NewAccordionSection(lang.L("Steam artwork (optional)"), e.artworkCard(), e.vm.artOpen)
		col.Add(e.artItem)
		body = kit.MaxWidth(780, col)
	} else {
		e.artItem = nil
		left, right := kit.Stack(e.gameCard(), e.howCard()), kit.Stack()
		if e.vm.kind != KindEmulator {
			left.Add(e.launchCard())
			right.Add(e.savesCard())
		}
		if e.vm.kind != KindDirect {
			left.Add(e.emulatorsCard())
		}
		right.Add(e.artworkCard())
		// On a very wide window the page stops growing: fields 1100 px wide
		// are hard to read. The form gets the larger share when the right
		// column only holds the artwork; the save locations need room.
		var ratios []float32
		if e.vm.kind == KindEmulator {
			ratios = []float32{3, 2}
		}
		body = kit.MaxWidth(1400, kit.NewCols(440, ratios, left, right))
	}
	kit.ReplaceScroll(e.scroll, kit.Stack(kit.PageHead(u.Back, head), body))
	e.renderBar()
}

// askGridKey asks for a SteamGridDB key in a dialog, so the form stays as it is.
func (e *Page) askGridKey() {
	flows.ShowGridKeyDialog(e.u, func() {
		e.vm.hasGridKey = true
		e.render()
	})
}

// Redraw draws the page again from its state (env.Redrawable).
func (e *Page) Redraw() { e.render() }
