package editor

import (
	"path/filepath"
	"runtime"
	"slices"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/lang"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/akfaiz/carryover/internal/launcher"
	"github.com/akfaiz/carryover/internal/library"
	"github.com/akfaiz/carryover/internal/stores/steamgrid"
	"github.com/akfaiz/carryover/internal/stores/steamstore"
	"github.com/akfaiz/carryover/services"
	"github.com/akfaiz/carryover/ui/components"
	"github.com/akfaiz/carryover/ui/env"
	"github.com/akfaiz/carryover/ui/kit"
)

// Page is the game editor: add a game, or edit one.
type Page struct {
	u env.Env
	*ViewModel

	scroll    *container.Scroll
	root      *fyne.Container // the page: the scrolling form over the bar
	bar       *fyne.Container // the Save / Cancel bar under the scrolling form
	nameEntry *widget.Entry
	exeEntry  *widget.Entry
	advItem   *kit.AccordionSection
	artItem   *kit.AccordionSection
	slotViews map[*slotForm]*slotView
}

// New returns the editor for the game id, or for a new game when id is empty.
// steamAppID pre-fills a new game from the installed Steam library.
func New(u env.Env, id, steamAppID string) *Page {
	return &Page{u: u, slotViews: map[*slotForm]*slotView{}, ViewModel: &ViewModel{
		id: id, steamAppID: steamAppID, loading: true, kind: KindDirect,
		dev: services.DeviceInput{Paths: map[string]string{}, Env: map[string]string{}, LaunchMode: library.LaunchDirect},
	}}
}

// NewEmulated returns the editor for a new game that is played through
// emulators: it starts on that kind.
func NewEmulated(u env.Env) *Page {
	e := New(u, "", "")
	e.kind = KindEmulator
	return e
}

// Name is the game name as typed.
func (e *Page) Name() string { return e.name }

// Kind is how the game is played: directly, through emulators, or both.
func (e *Page) Kind() PlayKind { return e.kind }

// EmulatorNames lists the emulators the game is set up with.
func (e *Page) EmulatorNames() []string {
	var out []string
	for _, m := range e.emus {
		out = append(out, m.name)
	}
	return out
}

func (e *Page) View() fyne.CanvasObject {
	e.scroll = kit.ScrollPage(kit.Muted(lang.L("Loading…")))
	e.bar = container.NewStack()
	u := e.u
	kit.Async(func() (Loaded, error) {
		l := Loaded{Tokens: u.Lib().Tokens(e.id), HasKey: u.Lib().HasSteamGridKey(), Emulators: u.Emus().List()}
		if e.id != "" {
			g, err := u.Lib().Get(e.id)
			if err != nil {
				return l, err
			}
			l.Game = &g
		}
		return l, nil
	}, func(l Loaded, err error) {
		e.Apply(l, err)
		e.render()
		if e.id == "" && e.steamAppID != "" {
			e.prefillFromSteam(e.steamAppID)
		}
	})
	// Save and Cancel stay in view under the form, however long it gets.
	e.root = container.NewBorder(nil, e.bar, nil, nil, e.scroll)
	return e.root
}

func (e *Page) prefillFromSteam(appID string) {
	kit.Async(func() ([]services.SteamLibraryGame, error) { return e.u.Lib().SteamLibrary(), nil }, func(lib []services.SteamLibraryGame, _ error) {
		for _, a := range lib {
			if a.AppID == appID {
				e.PrefillFromSteam(a)
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
	if strings.TrimSpace(e.name) == "" && e.storeAppID == "" {
		return
	}
	e.suggesting = true
	e.render()
	in := services.SuggestInput{GameID: e.id, Name: e.name, StoreAppID: e.storeAppID, Exe: e.dev.Exe, LaunchMode: e.dev.LaunchMode}
	kit.Async(func() (services.SuggestResult, error) { return e.u.Lib().SuggestSaves(in) }, func(res services.SuggestResult, err error) {
		e.suggesting = false
		if err != nil {
			e.u.Toast(lang.L("Save suggestions: {{.Err}}", map[string]any{"Err": err}), kit.ToastErr)
			e.render()
			return
		}
		if e.ApplySuggestions(res, autofill) {
			e.u.Toast(lang.L("Found {{.Game}} saves on this device", map[string]any{"Game": res.Matched}), kit.ToastOK)
		}
		e.render()
	})
}

func (e *Page) addSlot() {
	e.ViewModel.addSlot()
	e.render()
}
func (e *Page) searchSteam() {
	term := strings.TrimSpace(e.name)
	if len(term) < 2 {
		return
	}
	e.searching = true
	e.render()
	kit.Async(func() ([]steamstore.Match, error) { return e.u.Lib().SearchSteam(term) }, func(res []steamstore.Match, err error) {
		e.searching = false
		if err != nil {
			e.u.Toast(lang.L("Steam search failed: {{.Err}}", map[string]any{"Err": err}), kit.ToastErr)
			e.showResults = false
		} else {
			e.steamResults, e.showResults = res, true
		}
		e.render()
	})
}

func (e *Page) pickFile(title string, exts []string, done func(string)) {
	kit.PickFile(e.u.Window(), title, exts, done)
}

func (e *Page) render() {
	if e.loading {
		return
	}
	u := e.u
	// Keep accordion states across re-renders.
	if e.advItem != nil {
		e.advanced = e.advItem.Open()
	}
	if e.artItem != nil {
		e.artOpen = e.artItem.Open()
	}
	for _, s := range e.slots {
		e.slotViewOf(s).rememberOpen()
	}
	head := lang.L("Add game")
	if e.id != "" {
		head = lang.L("Edit game")
	}
	var body fyne.CanvasObject
	if e.id == "" {
		// A new game is filled in from top to bottom, one card after the other;
		// the artwork is optional, so it starts folded.
		col := kit.Stack(e.gameCard(), e.howCard())
		if e.kind != KindEmulator {
			col.Add(e.launchCard())
			col.Add(e.savesCard())
		}
		if e.kind != KindDirect {
			col.Add(e.emulatorsCard())
		}
		e.artItem = kit.NewAccordionSection(lang.L("Steam artwork (optional)"), e.artworkCard(), e.artOpen)
		col.Add(e.artItem)
		body = kit.MaxWidth(780, col)
	} else {
		e.artItem = nil
		left, right := kit.Stack(e.gameCard(), e.howCard()), kit.Stack()
		if e.kind != KindEmulator {
			left.Add(e.launchCard())
			right.Add(e.savesCard())
		}
		if e.kind != KindDirect {
			left.Add(e.emulatorsCard())
		}
		right.Add(e.artworkCard())
		// On a very wide window the page stops growing: fields 1100 px wide
		// are hard to read. The form gets the larger share when the right
		// column only holds the artwork; the save locations need room.
		var ratios []float32
		if e.kind == KindEmulator {
			ratios = []float32{3, 2}
		}
		body = kit.MaxWidth(1400, kit.NewCols(440, ratios, left, right))
	}
	kit.ReplaceScroll(e.scroll, kit.Stack(kit.PageHead(u.Back, head), body))
	e.renderBar()
}

// renderBar draws the bar under the form: an error or a hint about what is
// still missing, Cancel and Save.
func (e *Page) renderBar() {
	if e.bar == nil || e.root == nil || e.loading {
		return
	}
	saveLabel := lang.L("Save")
	if e.saving {
		saveLabel = lang.L("Saving…")
	}
	save := kit.Primary(saveLabel, e.save)
	kit.SetEnabled(save, e.CanSave())
	note := layout.NewSpacer()
	switch e.BarNote() {
	case NoteError:
		note = kit.Colored(e.errText, widget.DangerImportance)
	case NoteName:
		note = kit.Muted(lang.L("Give the game a name to save it."))
	case NoteEmulator:
		note = kit.Muted(lang.L("Add the emulator you play it with."))
	case NoteSaveLocation:
		note = kit.Muted(lang.L("No save location is set for this device yet. You can save anyway; the game shows “needs setup” here until you set one."))
	}
	line := canvas.NewRectangle(kit.ColLine)
	line.SetMinSize(fyne.NewSize(0, 1))
	row := container.NewBorder(nil, nil, nil, kit.HBox(widget.NewButton(lang.L("Cancel"), e.u.Back), save), note)
	// As wide as the form above it, so Save stays under it on a wide window.
	width := float32(780)
	if e.id != "" {
		width = 1400
	}
	e.bar.Objects = []fyne.CanvasObject{canvas.NewRectangle(kit.ColBg2),
		container.NewBorder(line, nil, nil, nil, container.New(layout.NewCustomPaddedLayout(10, 10, 28, 28), kit.MaxWidth(width, row)))}
	// The bar's height can change (a longer note wraps; it starts empty): lay
	// the page out again so the bar gets the height it needs.
	e.root.Refresh()
}

// askGridKey asks for a SteamGridDB key in a dialog, so the form stays as it is.
func (e *Page) askGridKey() {
	components.ShowGridKeyDialog(e.u, func() {
		e.hasGridKey = true
		e.render()
	})
}

func (e *Page) gameCard() fyne.CanvasObject {
	e.nameEntry = widget.NewEntry()
	e.nameEntry.SetPlaceHolder(lang.L("Game name"))
	e.nameEntry.SetText(e.name)
	findLabel := lang.L("Find on Steam")
	if e.searching {
		findLabel = lang.L("Searching…")
	}
	find := widget.NewButtonWithIcon(findLabel, theme.SearchIcon(), e.searchSteam)
	kit.SetEnabled(find, !e.searching && len(strings.TrimSpace(e.name)) >= 2)
	e.nameEntry.OnChanged = func(s string) {
		e.name = s
		kit.SetEnabled(find, !e.searching && len(strings.TrimSpace(s)) >= 2)
		e.renderBar()
	}
	e.nameEntry.OnSubmitted = func(string) { e.searchSteam() }
	c := kit.VBox(kit.Field(lang.L("Name"), kit.RowFill(e.nameEntry, find)))

	if e.showResults {
		res := kit.VBox()
		if len(e.steamResults) == 0 {
			res.Add(kit.Muted(lang.L("No match on Steam. You can still add the game.")))
		}
		for _, m := range e.steamResults {
			res.Add(kit.ListRow(m.Name, m.AppID, func() {
				e.name, e.storeAppID, e.showResults = m.Name, m.AppID, false
				e.render()
				e.suggestSaves(true)
			}))
		}
		res.Add(kit.HBox(kit.LowButton(lang.L("Close"), func() { e.showResults = false; e.render() })))
		scroll := container.NewVScroll(res)
		scroll.SetMinSize(fyne.NewSize(0, min(260, res.MinSize().Height)))
		c.Add(kit.Card("", scroll))
	}
	if e.storeAppID != "" {
		c.Add(kit.HBox(kit.Badge(lang.L("Steam app {{.ID}}", map[string]any{"ID": e.storeAppID}), kit.BadgeInfo), kit.LowButton(lang.L("Unlink"), func() {
			e.storeAppID, e.showResults = "", false
			e.render()
		})))
	}

	// Cover.
	var preview fyne.CanvasObject
	size := fyne.NewSize(80, 120)
	switch {
	case e.coverSrc != "":
		preview = kit.CoverView(kit.CoverSource{Path: e.coverSrc, Name: filepath.Base(e.coverSrc)}, size)
	case e.gridPick != nil:
		src := e.gridPick.Thumb
		if src == "" {
			src = e.gridPick.URL
		}
		preview = kit.CoverView(kit.CoverSource{URL: src}, size)
	case e.coverPath != "":
		preview = kit.CoverView(kit.CoverSource{Path: e.coverPath, Version: e.coverVersion}, size)
	default:
		preview = kit.CoverView(kit.CoverSource{Name: e.name}, size)
	}
	btns := kit.HBox(widget.NewButton(lang.L("Choose file…"), func() {
		e.pickFile(lang.L("Choose cover image"), []string{".png", ".jpg", ".jpeg", ".webp"}, func(p string) {
			e.coverSrc, e.gridPick = p, nil
			e.render()
		})
	}))
	if e.hasGridKey {
		btns.Add(widget.NewButton(lang.L("Find cover…"), func() {
			e.openArtPicker(steamgrid.KindCover)
		}))
	}
	side := kit.VBox(btns)
	hint := lang.L("Portrait image (600×900 works well). Also used as Steam grid art.")
	switch {
	case e.coverSrc != "":
		hint = lang.L("{{.File}} — copied when you save.", map[string]any{"File": filepath.Base(e.coverSrc)})
	case e.gridPick != nil:
		hint = lang.L("SteamGridDB cover — downloaded when you save.")
	}
	side.Add(kit.Muted(hint))
	if !e.hasGridKey {
		side.Add(container.NewBorder(nil, nil, nil, kit.LowButton(lang.L("Add key…"), e.askGridKey), kit.Muted(lang.L("Add a SteamGridDB API key to search community covers."))))
	}
	c.Add(container.NewBorder(nil, nil, preview, nil, container.NewPadded(side)))
	return kit.Card(lang.L("Game"), c)
}

func (e *Page) launchCard() fyne.CanvasObject {
	modes := &kit.Choice{}
	modes.Add(string(library.LaunchDirect), lang.L("Carryover"))
	if runtime.GOOS == "linux" || runtime.GOOS == "windows" || e.dev.LaunchMode == library.LaunchHeroic {
		modes.Add(string(library.LaunchHeroic), lang.L("Heroic Games Launcher"))
	}
	modes.Add(string(library.LaunchSteamWrap), lang.L("Steam"))
	e.exeEntry = kit.MonoEntry("", e.dev.Exe, func(s string) { e.dev.Exe = s })
	hint := kit.Muted("")
	args := kit.MonoEntry("-windowed", e.dev.Args, func(s string) { e.dev.Args = s })
	argsField := kit.Field(lang.L("Arguments"), args)
	exeExample := func(dir string) string {
		switch runtime.GOOS {
		case "windows":
			return `C:\Games\` + dir + `A\Game.exe`
		case "darwin":
			return "/Applications/GameA.app"
		}
		return "/home/deck/Games/" + strings.ReplaceAll(dir, `\`, "/") + "A/Game.exe"
	}
	applyMode := func() {
		switch e.dev.LaunchMode {
		case library.LaunchSteamWrap:
			hint.SetText(lang.L("Steam starts the game, and Carryover syncs before and after through the game's Steam launch options (shown on the game page). On Linux, Steam chooses the Proton version."))
			e.exeEntry.SetPlaceHolder(exeExample(""))
		case library.LaunchHeroic:
			hint.SetText(lang.L("Heroic starts the game with its own settings (and, on Linux, its Wine prefix). Pick the game's executable inside its Heroic install folder; Carryover waits for it to close, then syncs."))
			e.exeEntry.SetPlaceHolder(exeExample(`Heroic\`))
		default:
			hint.SetText(lang.L("Carryover runs the executable below. It syncs saves before the game starts and after it closes."))
			e.exeEntry.SetPlaceHolder(exeExample(""))
		}
		// Heroic launches the game with its own settings, so arguments set here are unused.
		if e.dev.LaunchMode == library.LaunchHeroic {
			argsField.Hide()
		} else {
			argsField.Show()
		}
	}
	applyMode()
	mode := kit.SelectOf(modes, string(e.dev.LaunchMode), func(v string) {
		e.dev.LaunchMode = library.LaunchMode(v)
		applyMode()
	})
	browse := widget.NewButton(lang.L("Browse…"), func() {
		kit.PickProgram(e.u.Window(), lang.L("Choose game executable"), func(p string) {
			p = launcher.AppBundle(p) // a program picked inside a macOS .app
			e.dev.Exe = p
			e.exeEntry.SetText(p)
			e.detectHeroic(p)
			if e.name != "" {
				return
			}
			// Name it after the exe if it's a known game, else its folder.
			kit.Async(func() (string, error) { return e.u.Lib().GuessName(p), nil }, func(name string, _ error) {
				if e.name == "" && name != "" {
					e.nameEntry.SetText(name)
				}
			})
		})
	})
	wait := kit.MonoEntry("Game-Win64-Shipping.exe", e.dev.WaitProcess, func(s string) { e.dev.WaitProcess = s })
	env := kit.MonoMultiLine("", e.envText, 3, func(s string) { e.envText = s })
	workDir := components.NewPathInput(e.u, e.id, e.tokens, e.dev.WorkDir, "", func(s string) { e.dev.WorkDir = s })
	e.advItem = kit.NewAccordionSection(lang.L("Advanced"), kit.VBox(
		kit.Field(lang.L("Working directory (default: executable's folder)"), workDir),
		kit.Field(lang.L("Wait for process (when the exe is a launcher that exits immediately)"), wait),
		kit.Field(lang.L("Environment variables (KEY=value per line)"), env),
	), e.advanced)

	launchTitle := lang.L("Launch on this device")
	if e.kind == KindBoth {
		launchTitle = lang.L("Launch without an emulator")
	}
	return kit.Card("", kit.CardHead(launchTitle, kit.Badge(e.u.Store().Settings.DeviceName, kit.BadgeNone)),
		kit.Field(lang.L("Launch with"), kit.VBox(kit.Outlined(mode), hint)),
		kit.Field(lang.L("Executable"), kit.RowFill(e.exeEntry, browse)),
		argsField,
		e.advItem)
}

// detectHeroic switches the launch mode to Heroic when exe is inside a game
// folder Heroic installed, so the game keeps Heroic's settings (and on Linux
// its Wine prefix instead of a new one from Steam).
func (e *Page) detectHeroic(exe string) {
	if e.dev.LaunchMode == library.LaunchHeroic {
		return
	}
	type found struct {
		title string
		ok    bool
	}
	kit.Async(func() (found, error) {
		title, ok := e.u.Lib().HeroicGame(exe)
		return found{title, ok}, nil
	}, func(f found, _ error) {
		if !f.ok || e.dev.Exe != exe {
			return
		}
		e.dev.LaunchMode = library.LaunchHeroic
		e.u.Toast(lang.L("{{.Game}} is installed by Heroic — it will be started through Heroic.", map[string]any{"Game": f.title}), kit.ToastOK)
		e.render()
	})
}

func (e *Page) savesCard() fyne.CanvasObject {
	suggestLabel := lang.L("Suggest")
	if e.suggesting {
		suggestLabel = lang.L("Looking…")
	}
	sugBtn := widget.NewButtonWithIcon(suggestLabel, theme.SearchIcon(), func() { e.suggestSaves(false) })
	kit.SetEnabled(sugBtn, !e.suggesting && (strings.TrimSpace(e.name) != "" || e.storeAppID != ""))
	savesTitle := lang.L("Save locations")
	if e.kind == KindBoth {
		savesTitle = lang.L("Save locations (without an emulator)")
	}
	c := kit.VBox(kit.CardHead(savesTitle, sugBtn, widget.NewButtonWithIcon(lang.L("Add"), theme.ContentAddIcon(), e.addSlot)))

	if e.suggest != nil {
		c.Add(kit.Card("", newSuggestionList(e.name, e.suggest,
			func(s services.SaveSuggestion) { e.useSuggestion(s, nil); e.render() },
			func(s services.SaveSuggestion) { e.addSuggestion(s); e.render() },
			func() { e.suggest = nil; e.render() })))
	}
	c.Add(components.Help(e.u, lang.L("Where the game keeps its saves on this device. Each device sets its own path."),
		lang.L("Each location is synced to its own cloud folder. Every device sets its own path for the same location — e.g. {{.Win}} on Windows and {{.Deck}} on a Steam Deck.",
			map[string]any{"Win": "{appdata}/GameA", "Deck": "{proton:shortcut}/AppData/Roaming/GameA"})))

	for i, s := range e.slots {
		c.Add(widget.NewSeparator())
		var onRemove func()
		if len(e.slots) > 1 {
			onRemove = func() {
				e.slots = slices.Delete(e.slots, i, i+1)
				e.render()
			}
		}
		c.Add(newSlotEditor(e.u, e.id, e.tokens, s, e.slotViewOf(s), e.others, e.uniqueKey, onRemove, e.renderBar))
	}
	return kit.Card("", c)
}

func (e *Page) save() {
	u := e.u
	e.errText = ""
	in, err := e.Input()
	if err != nil {
		e.errText = lang.L("Add at least one emulator, or choose “PC or Steam game”.")
		e.renderBar()
		return
	}
	e.saving = true
	e.renderBar()
	coverSrc, gridPick := e.coverSrc, e.gridPick

	artJobs := e.artworkJobs()
	var coverErr, artErr error
	kit.Async(func() (services.GameView, error) {
		g, err := u.Lib().Save(in)
		if err != nil {
			return g, err
		}
		switch {
		case coverSrc != "":
			if _, err := u.Lib().SetCover(g.ID, coverSrc); err != nil {
				return g, err
			}
		case gridPick != nil:
			_, coverErr = u.Lib().SetCoverFromURL(g.ID, gridPick.URL)
		}
		artErr = applyArtwork(u.Lib(), g.ID, artJobs)
		return g, nil
	}, func(g services.GameView, err error) {
		e.saving = false
		if err != nil {
			e.errText = err.Error()
			e.renderBar()
			return
		}
		e.saved = true
		if coverErr != nil {
			u.Toast(lang.L("Cover: {{.Err}}", map[string]any{"Err": coverErr}), kit.ToastErr)
		}
		if artErr != nil {
			u.Toast(lang.L("Artwork: {{.Err}}", map[string]any{"Err": artErr}), kit.ToastErr)
		}
		u.Store().Refresh()
		u.Toast(lang.L("Saved"), kit.ToastOK)
		// In place of the editor: Back from the game goes where the editor was
		// opened from, not into the editor again.
		u.Replace("/game/" + g.ID)
	})
}

// Redraw draws the page again from its state (env.Redrawable).
func (e *Page) Redraw() { e.render() }
