package ui

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"runtime"
	"slices"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/lang"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/akfaiz/carryover/internal/library"
	"github.com/akfaiz/carryover/internal/saves/pathvar"
	"github.com/akfaiz/carryover/internal/stores/steamgrid"
	"github.com/akfaiz/carryover/internal/stores/steamstore"
	"github.com/akfaiz/carryover/internal/ui/kit"
	"github.com/akfaiz/carryover/services"
)

type editor struct {
	u          *UI
	id         string
	steamAppID string // pre-fill from the installed Steam library

	name         string
	slots        []*slotForm
	emus         []*emuForm
	kind         playKind                // how the game is played: directly, through emulators, or both
	emuList      []services.EmulatorView // emulators that can be added
	dev          services.DeviceInput
	envText      string
	tokens       []pathvar.Token
	coverSrc     string          // local image chosen by the user
	gridPick     *steamgrid.Grid // SteamGridDB cover, downloaded on save
	hasGridKey   bool
	coverPath    string
	art          map[steamgrid.Kind]*artEdit // extra Steam artwork
	gridGameID   int                         // SteamGridDB game picked in this editor session
	gridGameName string
	coverVersion int64
	storeAppID   string
	steamResults []steamstore.Match
	showResults  bool
	searching    bool
	suggest      *services.SuggestResult
	suggesting   bool
	others       []services.DevicePaths
	saving       bool
	saved        bool // saved: leaving no longer loses anything
	errText      string
	advanced     bool
	artOpen      bool
	loading      bool
	orig         string // the form as loaded, to tell whether it changed

	scroll    *container.Scroll
	root      *fyne.Container // the page: the scrolling form over the bar
	bar       *fyne.Container // the Save / Cancel bar under the scrolling form
	nameEntry *widget.Entry
	exeEntry  *widget.Entry
	advItem   *kit.AccordionSection
	artItem   *kit.AccordionSection
}

// snapshot is the form's content as text, to compare with what it was loaded
// with.
func (e *editor) snapshot() string {
	type slot struct{ Key, Label, Include, Exclude, Path string }
	slots := func(list []*slotForm) []slot {
		var out []slot
		for _, s := range list {
			out = append(out, slot{s.key, s.label, s.include, s.exclude, s.path})
		}
		return out
	}
	type emu struct {
		ID, Rom, Share string
		Slots          []slot
	}
	var emus []emu
	for _, m := range e.emus {
		emus = append(emus, emu{m.id, m.rom, m.share, slots(m.slots)})
	}
	art := map[string]string{}
	for k, a := range e.art {
		if a.changed() {
			art[string(k)] = a.src + "|" + fmt.Sprint(a.pick != nil, a.clear)
		}
	}
	b, _ := json.Marshal(struct {
		Name, Store, Env, Cover string
		Kind                    playKind
		Grid                    bool
		Dev                     services.DeviceInput
		Slots                   []slot
		Emus                    []emu
		Art                     map[string]string
	}{e.name, e.storeAppID, e.envText, e.coverSrc, e.kind, e.gridPick != nil, e.dev, slots(e.slots), emus, art})
	return string(b)
}

// dirty reports changes that leaving the page would lose.
func (e *editor) dirty() bool {
	return !e.loading && !e.saved && !e.saving && e.snapshot() != e.orig
}

func newEditor(u *UI, id, steamAppID string) *editor {
	return &editor{u: u, id: id, steamAppID: steamAppID, loading: true, kind: kindDirect,
		dev: services.DeviceInput{Paths: map[string]string{}, Env: map[string]string{}, LaunchMode: library.LaunchDirect}}
}

func (e *editor) view() fyne.CanvasObject {
	e.scroll = kit.ScrollPage(kit.Muted(lang.L("Loading…")))
	e.bar = container.NewStack()
	u := e.u
	type loaded struct {
		tokens []pathvar.Token
		hasKey bool
		game   *services.GameView
		emus   []services.EmulatorView
	}
	kit.Async(func() (loaded, error) {
		l := loaded{tokens: u.lib.Tokens(e.id), hasKey: u.lib.HasSteamGridKey(), emus: u.emus.List()}
		if e.id != "" {
			g, err := u.lib.Get(e.id)
			if err != nil {
				return l, err
			}
			l.game = &g
		}
		return l, nil
	}, func(l loaded, err error) {
		e.loading = false
		e.tokens, e.hasGridKey, e.emuList = l.tokens, l.hasKey, l.emus
		if err != nil {
			e.errText = err.Error()
		}
		if e.id == "" {
			e.slots = []*slotForm{{key: "saves", label: "Saves", isNew: true}}
			e.orig = e.snapshot()
			e.render()
			if e.steamAppID != "" {
				e.prefillFromSteam(e.steamAppID)
			}
			return
		}
		if g := l.game; g != nil {
			e.name = g.Name
			e.coverPath, e.coverVersion = g.CoverPath, g.CoverVersion
			for kind, f := range g.Art {
				a := e.artEdit(steamgrid.Kind(kind))
				a.path, a.version = f.Path, f.Version
			}
			e.storeAppID = g.StoreAppID
			e.others = g.OtherDevices
			d := g.Device
			e.dev = services.DeviceInput{
				Paths: map[string]string{}, Exe: d.Exe, Args: d.Args, WorkDir: d.WorkDir,
				Env: map[string]string{}, WaitProcess: d.WaitProcess, LaunchMode: d.LaunchMode,
			}
			if e.dev.LaunchMode == "" {
				e.dev.LaunchMode = library.LaunchDirect
			}
			var env []string
			for k, v := range d.Env {
				env = append(env, k+"="+v)
			}
			slices.Sort(env)
			e.envText = strings.Join(env, "\n")
			e.advanced = e.dev.WorkDir != "" || e.dev.WaitProcess != "" || e.envText != ""
			for _, s := range g.Slots {
				e.slots = append(e.slots, &slotForm{
					key: s.Key, label: labelOf(s.Label, s.Key),
					include: strings.Join(s.Include, "\n"), exclude: strings.Join(s.Exclude, "\n"),
					path: d.Paths[s.Key],
				})
			}
			e.loadEmulators(g)
			e.kind = playKindOf(g, len(e.emus) > 0)
		}
		e.orig = e.snapshot()
		e.render()
	})
	// Save and Cancel stay in view under the form, however long it gets.
	e.root = container.NewBorder(nil, e.bar, nil, nil, e.scroll)
	return e.root
}

func (e *editor) prefillFromSteam(appID string) {
	kit.Async(func() ([]services.SteamLibraryGame, error) { return e.u.lib.SteamLibrary(), nil }, func(lib []services.SteamLibraryGame, _ error) {
		for _, a := range lib {
			if a.AppID == appID {
				e.name, e.storeAppID = a.Name, a.AppID
				// A Steam-installed game is started by Steam; Carryover wraps it via launch options.
				e.dev.LaunchMode = library.LaunchSteamWrap
				e.orig = e.snapshot() // the game picked in Steam is where the form starts
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
func (e *editor) suggestSaves(autofill bool) {
	if strings.TrimSpace(e.name) == "" && e.storeAppID == "" {
		return
	}
	e.suggesting = true
	e.render()
	in := services.SuggestInput{GameID: e.id, Name: e.name, StoreAppID: e.storeAppID, Exe: e.dev.Exe, LaunchMode: e.dev.LaunchMode}
	kit.Async(func() (services.SuggestResult, error) { return e.u.lib.SuggestSaves(in) }, func(res services.SuggestResult, err error) {
		e.suggesting = false
		if err != nil {
			e.u.toast(lang.L("Save suggestions: {{.Err}}", map[string]any{"Err": err}), kit.ToastErr)
			e.render()
			return
		}
		e.suggest = &res
		var found []services.SaveSuggestion
		for _, s := range res.Suggestions {
			if s.Files > 0 {
				found = append(found, s)
			}
		}
		if autofill && len(found) == 1 && len(e.slots) > 0 && e.slots[0].path == "" {
			e.useSuggestion(found[0], e.slots[0])
			e.u.toast(lang.L("Found {{.Game}} saves on this device", map[string]any{"Game": res.Matched}), kit.ToastOK)
		}
		e.render()
	})
}

func (e *editor) useSuggestion(sg services.SaveSuggestion, target *slotForm) {
	if target == nil {
		for _, s := range e.slots {
			if s.path == "" {
				target = s
				break
			}
		}
	}
	if target == nil {
		e.addSuggestion(sg)
		return
	}
	target.path = sg.Path
	target.include = strings.Join(sg.Include, "\n")
	target.exclude = ""
}

func (e *editor) uniqueKey(base string, self *slotForm) string {
	k, i := base, 2
	for slices.ContainsFunc(e.slots, func(o *slotForm) bool { return o != self && o.key == k }) {
		k = fmt.Sprintf("%s-%d", base, i)
		i++
	}
	return k
}

func (e *editor) addSuggestion(sg services.SaveSuggestion) {
	parts := strings.FieldsFunc(sg.Path, func(r rune) bool { return r == '/' })
	label := "Saves"
	if len(parts) > 0 {
		label = parts[len(parts)-1]
	}
	e.slots = append(e.slots, &slotForm{key: e.uniqueKey(slug(label), nil), label: label,
		include: strings.Join(sg.Include, "\n"), path: sg.Path, isNew: true})
}

func (e *editor) addSlot() {
	n := len(e.slots) + 1
	for slices.ContainsFunc(e.slots, func(s *slotForm) bool { return s.key == fmt.Sprintf("saves-%d", n) }) {
		n++
	}
	e.slots = append(e.slots, &slotForm{key: fmt.Sprintf("saves-%d", n), label: fmt.Sprintf("Saves %d", n), isNew: true})
	e.render()
}

func (e *editor) searchSteam() {
	term := strings.TrimSpace(e.name)
	if len(term) < 2 {
		return
	}
	e.searching = true
	e.render()
	kit.Async(func() ([]steamstore.Match, error) { return e.u.lib.SearchSteam(term) }, func(res []steamstore.Match, err error) {
		e.searching = false
		if err != nil {
			e.u.toast(lang.L("Steam search failed: {{.Err}}", map[string]any{"Err": err}), kit.ToastErr)
			e.showResults = false
		} else {
			e.steamResults, e.showResults = res, true
		}
		e.render()
	})
}

func (e *editor) pickFile(title string, exts []string, done func(string)) {
	d := dialog.NewFileOpen(func(r fyne.URIReadCloser, err error) {
		if err != nil || r == nil {
			return
		}
		p := filepath.FromSlash(r.URI().Path())
		_ = r.Close()
		done(p)
	}, e.u.win)
	d.SetTitleText(title)
	if len(exts) > 0 {
		d.SetFilter(storage.NewExtensionFileFilter(exts))
	}
	d.Resize(dialogSize(e.u.win))
	d.Show()
}

func (e *editor) render() {
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
		s.rememberOpen()
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
		if e.kind != kindEmulator {
			col.Add(e.launchCard())
			col.Add(e.savesCard())
		}
		if e.kind != kindDirect {
			col.Add(e.emulatorsCard())
		}
		e.artItem = kit.NewAccordionSection(lang.L("Steam artwork (optional)"), e.artworkCard(), e.artOpen)
		col.Add(e.artItem)
		body = kit.MaxWidth(780, col)
	} else {
		e.artItem = nil
		left, right := kit.Stack(e.gameCard(), e.howCard()), kit.Stack()
		if e.kind != kindEmulator {
			left.Add(e.launchCard())
			right.Add(e.savesCard())
		}
		if e.kind != kindDirect {
			left.Add(e.emulatorsCard())
		}
		right.Add(e.artworkCard())
		body = kit.NewCols(440, nil, left, right)
	}
	kit.ReplaceScroll(e.scroll, kit.Stack(kit.PageHead(u.back, head), body))
	e.renderBar()
}

// renderBar draws the bar under the form: an error or a hint about what is
// still missing, Cancel and Save.
func (e *editor) renderBar() {
	if e.bar == nil || e.root == nil || e.loading {
		return
	}
	saveLabel := lang.L("Save")
	if e.saving {
		saveLabel = lang.L("Saving…")
	}
	save := kit.Primary(saveLabel, e.save)
	kit.SetEnabled(save, !e.saving && strings.TrimSpace(e.name) != "")
	note := layout.NewSpacer()
	switch {
	case e.errText != "":
		note = kit.Colored(e.errText, widget.DangerImportance)
	case strings.TrimSpace(e.name) == "":
		note = kit.Muted(lang.L("Give the game a name to save it."))
	case e.kind != kindDirect && len(e.emus) == 0:
		note = kit.Muted(lang.L("Add the emulator you play it with."))
	case e.kind != kindEmulator && !slices.ContainsFunc(e.slots, func(s *slotForm) bool { return strings.TrimSpace(s.path) != "" }):
		note = kit.Muted(lang.L("No save location is set for this device yet. You can save anyway; the game shows “needs setup” here until you set one."))
	}
	line := canvas.NewRectangle(kit.ColLine)
	line.SetMinSize(fyne.NewSize(0, 1))
	row := container.NewBorder(nil, nil, nil, kit.HBox(widget.NewButton(lang.L("Cancel"), e.u.back), save), note)
	e.bar.Objects = []fyne.CanvasObject{canvas.NewRectangle(kit.ColBg2),
		container.NewBorder(line, nil, nil, nil, container.New(layout.NewCustomPaddedLayout(10, 10, 28, 28), row))}
	// The bar's height can change (a longer note wraps; it starts empty): lay
	// the page out again so the bar gets the height it needs.
	e.root.Refresh()
}

// askGridKey asks for a SteamGridDB key in a dialog, so the form stays as it is.
func (e *editor) askGridKey() {
	showGridKeyDialog(e.u, func() {
		e.hasGridKey = true
		e.render()
	})
}

func (e *editor) gameCard() fyne.CanvasObject {
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

func (e *editor) launchCard() fyne.CanvasObject {
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
		if runtime.GOOS == "windows" {
			return `C:\Games\` + dir + `A\Game.exe`
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
		e.pickFile(lang.L("Choose game executable"), nil, func(p string) {
			e.dev.Exe = p
			e.exeEntry.SetText(p)
			e.detectHeroic(p)
			if e.name != "" {
				return
			}
			// Name it after the exe if it's a known game, else its folder.
			kit.Async(func() (string, error) { return e.u.lib.GuessName(p), nil }, func(name string, _ error) {
				if e.name == "" && name != "" {
					e.nameEntry.SetText(name)
				}
			})
		})
	})
	wait := kit.MonoEntry("Game-Win64-Shipping.exe", e.dev.WaitProcess, func(s string) { e.dev.WaitProcess = s })
	env := kit.MonoMultiLine("", e.envText, 3, func(s string) { e.envText = s })
	workDir := newPathInput(e.u, e.id, e.tokens, e.dev.WorkDir, "", func(s string) { e.dev.WorkDir = s })
	e.advItem = kit.NewAccordionSection(lang.L("Advanced"), kit.VBox(
		kit.Field(lang.L("Working directory (default: executable's folder)"), workDir),
		kit.Field(lang.L("Wait for process (when the exe is a launcher that exits immediately)"), wait),
		kit.Field(lang.L("Environment variables (KEY=value per line)"), env),
	), e.advanced)

	launchTitle := lang.L("Launch on this device")
	if e.kind == kindBoth {
		launchTitle = lang.L("Launch without an emulator")
	}
	return kit.Card("", kit.CardHead(launchTitle, kit.Badge(e.u.settings.DeviceName, kit.BadgeNone)),
		kit.Field(lang.L("Launch with"), kit.VBox(mode, hint)),
		kit.Field(lang.L("Executable"), kit.RowFill(e.exeEntry, browse)),
		argsField,
		e.advItem)
}

// detectHeroic switches the launch mode to Heroic when exe is inside a game
// folder Heroic installed, so the game keeps Heroic's settings (and on Linux
// its Wine prefix instead of a new one from Steam).
func (e *editor) detectHeroic(exe string) {
	if e.dev.LaunchMode == library.LaunchHeroic {
		return
	}
	type found struct {
		title string
		ok    bool
	}
	kit.Async(func() (found, error) {
		title, ok := e.u.lib.HeroicGame(exe)
		return found{title, ok}, nil
	}, func(f found, _ error) {
		if !f.ok || e.dev.Exe != exe {
			return
		}
		e.dev.LaunchMode = library.LaunchHeroic
		e.u.toast(lang.L("{{.Game}} is installed by Heroic — it will be started through Heroic.", map[string]any{"Game": f.title}), kit.ToastOK)
		e.render()
	})
}

func (e *editor) savesCard() fyne.CanvasObject {
	suggestLabel := lang.L("Suggest")
	if e.suggesting {
		suggestLabel = lang.L("Looking…")
	}
	sugBtn := widget.NewButtonWithIcon(suggestLabel, theme.SearchIcon(), func() { e.suggestSaves(false) })
	kit.SetEnabled(sugBtn, !e.suggesting && (strings.TrimSpace(e.name) != "" || e.storeAppID != ""))
	savesTitle := lang.L("Save locations")
	if e.kind == kindBoth {
		savesTitle = lang.L("Save locations (without an emulator)")
	}
	c := kit.VBox(kit.CardHead(savesTitle, sugBtn, widget.NewButtonWithIcon(lang.L("Add"), theme.ContentAddIcon(), e.addSlot)))

	if e.suggest != nil {
		c.Add(kit.Card("", newSuggestionList(e.name, e.suggest,
			func(s services.SaveSuggestion) { e.useSuggestion(s, nil); e.render() },
			func(s services.SaveSuggestion) { e.addSuggestion(s); e.render() },
			func() { e.suggest = nil; e.render() })))
	}
	c.Add(e.u.help(lang.L("Where the game keeps its saves on this device. Each device sets its own path."),
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
		c.Add(newSlotEditor(e.u, e.id, e.tokens, s, e.others, e.uniqueKey, onRemove, e.renderBar))
	}
	return kit.Card("", c)
}

func (e *editor) save() {
	u := e.u
	e.errText = ""
	if e.kind != kindDirect && len(e.emus) == 0 {
		e.errText = lang.L("Add at least one emulator, or choose “PC or Steam game”.")
		e.renderBar()
		return
	}
	e.saving = true
	e.renderBar()
	env := map[string]string{}
	for l := range strings.SplitSeq(e.envText, "\n") {
		if i := strings.Index(l, "="); i > 0 {
			env[strings.TrimSpace(l[:i])] = l[i+1:]
		}
	}
	paths := map[string]string{}
	slots := make([]library.SaveSlot, 0, len(e.slots))
	for _, s := range e.slots {
		paths[s.key] = s.path
		slots = append(slots, library.SaveSlot{Key: s.key, Label: s.label, Include: splitLines(s.include), Exclude: splitLines(s.exclude)})
	}
	dev := e.dev
	dev.Paths, dev.Env = paths, env
	emus := e.emulatorInputs()
	switch e.kind {
	case kindEmulator: // played only through emulators: no launch config or save location of its own
		slots, dev = nil, services.DeviceInput{Paths: map[string]string{}, Env: map[string]string{}, LaunchMode: library.LaunchDirect}
	case kindDirect:
		emus = nil
	}
	in := services.GameInput{ID: e.id, Name: e.name, StoreAppID: e.storeAppID, Slots: slots, Device: dev, Emulators: emus}
	coverSrc, gridPick := e.coverSrc, e.gridPick

	artJobs := e.artworkJobs()
	var coverErr, artErr error
	kit.Async(func() (services.GameView, error) {
		g, err := u.lib.Save(in)
		if err != nil {
			return g, err
		}
		switch {
		case coverSrc != "":
			if _, err := u.lib.SetCover(g.ID, coverSrc); err != nil {
				return g, err
			}
		case gridPick != nil:
			_, coverErr = u.lib.SetCoverFromURL(g.ID, gridPick.URL)
		}
		artErr = applyArtwork(u.lib, g.ID, artJobs)
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
			u.toast(lang.L("Cover: {{.Err}}", map[string]any{"Err": coverErr}), kit.ToastErr)
		}
		if artErr != nil {
			u.toast(lang.L("Artwork: {{.Err}}", map[string]any{"Err": artErr}), kit.ToastErr)
		}
		u.refresh()
		u.toast(lang.L("Saved"), kit.ToastOK)
		// In place of the editor: Back from the game goes where the editor was
		// opened from, not into the editor again.
		u.replace("/game/" + g.ID)
	})
}
