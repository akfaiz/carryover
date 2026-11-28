package editor

import (
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/akfaiz/carryover/services"
	"github.com/akfaiz/carryover/ui/components"
	"github.com/akfaiz/carryover/ui/env"
	"github.com/akfaiz/carryover/ui/flows"
	"github.com/akfaiz/carryover/ui/i18n"
	"github.com/akfaiz/carryover/ui/kit"
)

// Page is the game editor: add a game, or edit one. MyGo draws it every frame:
// the fields are bound straight to the ViewModel's, so typing changes the
// ViewModel without a callback; what must follow a change (the save-location
// key from a label, a path's preview) is noticed as the frame is built.
type Page struct {
	u  env.Env
	vm *ViewModel

	scroll    ui.ScrollState
	slotOpen  map[*slotForm]*bool // whether a slot's Filters section is open
	slotLabel map[*slotForm]string
	paths     map[*string]*components.PathInput
}

var (
	_ env.Guarded    = (*Page)(nil)
	_ env.Scrollable = (*Page)(nil)
)

// Dirty: the form has unsaved changes (env.Guarded).
func (e *Page) Dirty() bool { return e.vm.Dirty() }

// New returns the editor for the game id, or for a new game when id is empty.
// steamAppID pre-fills a new game from the installed Steam library.
func New(u env.Env, id, steamAppID string) *Page {
	e := &Page{
		u: u, vm: newViewModel(id, steamAppID, serviceBackend{u}),
		slotOpen: map[*slotForm]*bool{}, slotLabel: map[*slotForm]string{}, paths: map[*string]*components.PathInput{},
	}
	kit.Async(e.vm.Load, func(l Loaded, err error) {
		e.vm.Apply(l, err)
		if e.vm.id == "" && e.vm.steamAppID != "" {
			e.prefillFromSteam(e.vm.steamAppID)
		}
		if e.vm.id == "" && e.vm.lutrisID != "" {
			e.prefillFromLutris(e.vm.lutrisID)
		}
		if e.vm.id == "" && e.vm.heroicGame[1] != "" {
			e.prefillFromHeroic(e.vm.heroicGame[0], e.vm.heroicGame[1])
		}
		if e.vm.id == "" && e.vm.bottlesID[1] != "" {
			e.prefillFromBottles(e.vm.bottlesID[0], e.vm.bottlesID[1])
		}
	})
	return e
}

// NewFromLutris returns the editor for a new game, pre-filled from the game
// Lutris has under this config id.
func NewFromLutris(u env.Env, configID string) *Page {
	e := New(u, "", "")
	e.vm.lutrisID = configID
	return e
}

// NewFromHeroic returns the editor for a new game, pre-filled from the game
// Heroic installed with this runner and app name.
func NewFromHeroic(u env.Env, runner, appName string) *Page {
	e := New(u, "", "")
	e.vm.heroicGame = [2]string{runner, appName}
	return e
}

// NewFromBottles returns the editor for a new game, pre-filled from the
// program with this id in the bottle of this name.
func NewFromBottles(u env.Env, bottle, programID string) *Page {
	e := New(u, "", "")
	e.vm.bottlesID = [2]string{bottle, programID}
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

// Scroll is the page's scroll container, which the pad's right stick scrolls.
func (e *Page) Scroll() *ui.ScrollState { return &e.scroll }

func (e *Page) prefillFromSteam(appID string) {
	kit.Async(func() ([]services.SteamLibraryGame, error) { return e.vm.be.SteamLibrary(), nil }, func(lib []services.SteamLibraryGame, _ error) {
		for _, a := range lib {
			if a.AppID == appID {
				e.vm.PrefillFromSteam(a)
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
				e.suggestSaves(true)
				return
			}
		}
	})
}

func (e *Page) prefillFromBottles(bottle, programID string) {
	kit.Async(func() ([]services.BottlesLibraryGame, error) { return e.vm.be.BottlesLibrary(), nil }, func(lib []services.BottlesLibraryGame, _ error) {
		for _, g := range lib {
			if g.Bottle == bottle && g.ProgramID == programID {
				e.vm.PrefillFromBottles(g)
				e.suggestSaves(true)
				return
			}
		}
	})
}

func (e *Page) prefillFromHeroic(runner, appName string) {
	kit.Async(func() ([]services.HeroicLibraryGame, error) { return e.vm.be.HeroicLibrary(), nil }, func(lib []services.HeroicLibraryGame, _ error) {
		for _, g := range lib {
			if g.Runner == runner && g.AppName == appName {
				e.vm.PrefillFromHeroic(g)
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
	in := services.SuggestInput{GameID: e.vm.id, Name: e.vm.name, StoreAppID: e.vm.storeAppID, Exe: e.vm.dev.Exe, LaunchMode: e.vm.dev.LaunchMode}
	kit.Async(func() (services.SuggestResult, error) { return e.vm.be.SuggestSaves(in) }, func(res services.SuggestResult, err error) {
		e.vm.suggesting = false
		if err != nil {
			e.u.Toast(i18n.L("Save suggestions: {{.Err}}", map[string]any{"Err": i18n.Err(err)}), kit.ToastErr)
			return
		}
		if e.vm.ApplySuggestions(res, autofill) {
			e.u.Toast(i18n.L("Found {{.Game}} saves on this device", map[string]any{"Game": res.Matched}), kit.ToastOK)
		}
	})
	kit.Redraw()
}

func (e *Page) searchSteam() {
	term := strings.TrimSpace(e.vm.name)
	if len(term) < 2 {
		return
	}
	e.vm.searching = true
	kit.Async(func() ([]services.StoreMatch, error) { return e.vm.be.SearchSteam(term) }, func(res []services.StoreMatch, err error) {
		e.vm.searching = false
		if err != nil {
			e.u.Toast(i18n.L("Steam search failed: {{.Err}}", map[string]any{"Err": i18n.Err(err)}), kit.ToastErr)
			e.vm.showResults = false
		} else {
			e.vm.steamResults, e.vm.showResults = res, true
		}
	})
	kit.Redraw()
}

// pickFile asks for a file, starting at the folder of from (a path that may
// contain tokens; "" for the dialog's default).
func (e *Page) pickFile(title string, exts []string, from string, done func(string)) {
	components.BrowseFrom(e.u, e.vm.id, from, func(dir string) {
		kit.PickFile(e.u.Window(), title, exts, dir, done)
	})
}

// askGridKey asks for a SteamGridDB key in a dialog, so the form stays as it is.
func (e *Page) askGridKey() {
	flows.ShowGridKeyDialog(e.u, func() { e.vm.hasGridKey = true })
}

// pathInput is the path entry for the string at ptr, kept across frames; it
// follows the string when something else changes it (a suggestion is used).
func (e *Page) pathInput(ptr *string, gameID, placeholder string, withTokens bool) *components.PathInput {
	pi := e.paths[ptr]
	if pi == nil {
		tokens := e.vm.tokens
		if !withTokens {
			tokens = nil
		}
		pi = components.NewPathInput(e.u, gameID, tokens, *ptr, placeholder, func(v string) { *ptr = v })
		e.paths[ptr] = pi
	}
	pi.Follow(*ptr)
	return pi
}

// btn is a button with an optional icon that registers with the gamepad. key
// tells it from the others on the page.
func (e *Page) btn(c *ui.Context, key any, kind kit.BtnKind, icon *ui.SVG, label string, disabled bool, action func()) ui.Element {
	return kit.Btn(c, e.u.Navigator(), [2]any{"editor", key}, kind, icon, label, action, kit.BtnDisabled(disabled))
}

// field is a single-line text input bound to *v.
func (e *Page) field(c *ui.Context, key any, v *string, placeholder string, mono bool) ui.Element {
	in := kit.Field(c, e.u.Navigator(), [2]any{"editor-field", key}, v, placeholder)
	if mono {
		in.Font("monospace")
	}
	return in
}

// area is a multi-line text input bound to *v.
func (e *Page) area(c *ui.Context, key any, v *string, placeholder string, height float32) {
	kit.TextArea(c, e.u.Navigator(), [2]any{"editor-area", key}, v, placeholder, height)
}

// View draws the page: the scrolling form over the Save / Cancel bar.
func (e *Page) View(c *ui.Context) {
	nav := e.u.Navigator()
	head := i18n.L("Add game")
	if e.vm.id != "" {
		head = i18n.L("Edit game")
	}
	width := kit.PageWide
	if e.vm.id == "" {
		width = kit.PageForm
	}
	ui.Column(c).Grow(1).Children(func() {
		kit.Page(c, width, &e.scroll, func() {
			kit.PageHead(c, nav, e.u.Back, i18n.L("Back"), head, nil)
			if e.vm.loading {
				kit.Muted(c, i18n.L("Loading…"))
				return
			}
			e.form(c)
		})
		if !e.vm.loading {
			e.bar(c)
		}
	})
}

func (e *Page) form(c *ui.Context) {
	if e.vm.id == "" {
		// A new game is filled in from top to bottom, one card after the other;
		// the artwork is optional, so it starts folded.
		ui.Column(c).Gap(kit.S4).Children(func() {
			e.gameCard(c)
			e.howCard(c)
			if e.vm.kind != KindEmulator {
				e.launchCard(c)
				e.savesCard(c)
			}
			if e.vm.kind != KindDirect {
				e.emulatorsCard(c)
			}
			kit.Disclosure(c, e.u.Navigator(), "art-open", i18n.L("Steam artwork (optional)"), &e.vm.artOpen, func() { e.artworkCard(c) })
		})
		return
	}
	// On a very wide window the page stops growing: fields 1100 px wide are
	// hard to read. The form gets the larger share when the right column only
	// holds the artwork; the save locations need room.
	left, right := float32(1), float32(1)
	if e.vm.kind == KindEmulator {
		left, right = 3, 2
	}
	ui.Row(c).Gap(kit.S4).Wrap().AlignItems(ui.Start).Children(func() {
		ui.Column(c).Grow(left).MinWidth(440).Gap(kit.S4).Children(func() {
			e.gameCard(c)
			e.howCard(c)
			if e.vm.kind != KindEmulator {
				e.launchCard(c)
			}
			if e.vm.kind != KindDirect {
				e.emulatorsCard(c)
			}
		})
		ui.Column(c).Grow(right).MinWidth(440).Gap(kit.S4).Children(func() {
			if e.vm.kind != KindEmulator {
				e.savesCard(c)
			}
			e.artworkCard(c)
		})
	})
}

// bar is the bar under the form: an error or a hint about what is still
// missing, Cancel and Save. It stays in view however long the form gets.
func (e *Page) bar(c *ui.Context) {
	t := c.Theme()
	ui.Column(c).Background(t.Surface).Children(func() {
		ui.Divider(c)
		ui.Row(c).Gap(kit.S3).Padding(kit.S3, kit.S8).AlignItems(ui.Center).Children(func() {
			var note ui.Element
			switch e.vm.BarNote() {
			case NoteError:
				note = kit.Danger(c, i18n.Text(e.vm.errText, e.vm.errMsg))
			case NoteName:
				note = kit.Muted(c, i18n.L("Give the game a name to save it."))
			case NoteEmulator:
				note = kit.Muted(c, i18n.L("Add the emulator you play it with."))
			case NoteSaveLocation:
				note = kit.Muted(c, i18n.L("No save location is set for this device yet. You can save anyway; the game shows “needs setup” here until you set one."))
			default:
				note = ui.Spacer(c)
			}
			note.Grow(1)
			e.btn(c, "cancel", kit.BtnSecondary, nil, i18n.L("Cancel"), false, e.u.Back)
			label := i18n.L("Save")
			if e.vm.saving {
				label = i18n.L("Saving…")
			}
			e.btn(c, "save", kit.BtnPrimary, nil, label, !e.vm.CanSave(), e.save)
		})
	})
}

func (e *Page) save() {
	u := e.u
	e.vm.errText, e.vm.errMsg = "", nil
	in, err := e.vm.Input()
	if err != nil {
		e.vm.errText = i18n.L("Add at least one emulator, or choose “PC or Steam game”.")
		return
	}
	e.vm.saving = true
	coverSrc, gridPick := e.vm.coverSrc, e.vm.gridPick
	artJobs := e.vm.artworkJobs()
	kit.Async(func() (SaveOutcome, error) {
		return e.vm.Submit(in, coverSrc, gridPick, artJobs)
	}, func(out SaveOutcome, err error) {
		g, coverErr, artErr := out.Game, out.CoverErr, out.ArtErr
		e.vm.saving = false
		if err != nil {
			e.vm.errText, e.vm.errMsg = services.ErrorText(err)
			return
		}
		e.vm.saved = true
		if coverErr != nil {
			u.Toast(i18n.L("Cover: {{.Err}}", map[string]any{"Err": i18n.Err(coverErr)}), kit.ToastErr)
		}
		if artErr != nil {
			u.Toast(i18n.L("Artwork: {{.Err}}", map[string]any{"Err": i18n.Err(artErr)}), kit.ToastErr)
		}
		u.Store().Refresh()
		u.Toast(i18n.L("Saved"), kit.ToastOK)
		// In place of the editor: Back from the game goes where the editor was
		// opened from, not into the editor again.
		u.Replace("/game/" + g.ID)
	})
}
