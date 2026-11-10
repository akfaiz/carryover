package ui

import (
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/lang"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/akfaiz/carryover/internal/library"
	"github.com/akfaiz/carryover/internal/pathvar"
	"github.com/akfaiz/carryover/internal/steamgrid"
	"github.com/akfaiz/carryover/internal/steamstore"
	"github.com/akfaiz/carryover/services"
)

type editor struct {
	u          *UI
	id         string
	steamAppID string // pre-fill from the installed Steam library

	name               string
	slots              []*slotForm
	dev                services.DeviceInput
	envText            string
	tokens             []pathvar.Token
	coverSrc           string          // local image chosen by the user
	gridPick           *steamgrid.Grid // SteamGridDB cover, downloaded on save
	hasGridKey         bool
	coverPath          string
	art                map[steamgrid.Kind]*artEdit // extra Steam artwork
	gridGameID         int                         // SteamGridDB game picked in this editor session
	gridGameName       string
	coverVersion       int64
	storeAppID         string
	originalStoreAppID string
	steamResults       []steamstore.Match
	showResults        bool
	searching          bool
	suggest            *services.SuggestResult
	suggesting         bool
	others             []services.DevicePaths
	saving             bool
	errText            string
	advanced           bool
	loading            bool

	scroll    *container.Scroll
	nameEntry *widget.Entry
	exeEntry  *widget.Entry
	saveBtn   *widget.Button
	advItem   *accordionSection
}

func newEditor(u *UI, id, steamAppID string) *editor {
	return &editor{u: u, id: id, steamAppID: steamAppID, loading: true,
		dev: services.DeviceInput{Paths: map[string]string{}, Env: map[string]string{}, LaunchMode: library.LaunchDirect}}
}

func (e *editor) view() fyne.CanvasObject {
	e.scroll = scrollPage(muted(lang.L("Loading…")))
	u := e.u
	type loaded struct {
		tokens []pathvar.Token
		hasKey bool
		game   *services.GameView
	}
	async(func() (loaded, error) {
		l := loaded{tokens: u.lib.Tokens(e.id), hasKey: u.lib.HasSteamGridKey()}
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
		e.tokens, e.hasGridKey = l.tokens, l.hasKey
		if err != nil {
			e.errText = err.Error()
		}
		if e.id == "" {
			e.slots = []*slotForm{{key: "saves", label: "Saves", isNew: true}}
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
			e.storeAppID, e.originalStoreAppID = g.StoreAppID, g.StoreAppID
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
		}
		e.render()
	})
	return e.scroll
}

func (e *editor) prefillFromSteam(appID string) {
	async(func() ([]services.SteamLibraryGame, error) { return e.u.lib.SteamLibrary(), nil }, func(lib []services.SteamLibraryGame, _ error) {
		for _, a := range lib {
			if a.AppID == appID {
				e.name, e.storeAppID = a.Name, a.AppID
				// A Steam-installed game is started by Steam; Carryover wraps it via launch options.
				e.dev.LaunchMode = library.LaunchSteamWrap
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
	async(func() (services.SuggestResult, error) { return e.u.lib.SuggestSaves(in) }, func(res services.SuggestResult, err error) {
		e.suggesting = false
		if err != nil {
			e.u.toast(lang.L("Save suggestions: {{.Err}}", map[string]any{"Err": err}), toastErr)
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
			e.u.toast(lang.L("Found {{.Game}} saves on this device", map[string]any{"Game": res.Matched}), toastOK)
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
	async(func() ([]steamstore.Match, error) { return e.u.lib.SearchSteam(term) }, func(res []steamstore.Match, err error) {
		e.searching = false
		if err != nil {
			e.u.toast(lang.L("Steam search failed: {{.Err}}", map[string]any{"Err": err}), toastErr)
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
	for _, s := range e.slots {
		s.rememberOpen()
	}
	head := lang.L("Add game")
	if e.id != "" {
		head = lang.L("Edit game")
	}
	e.saveBtn = nil
	left := vbox(e.gameCard(), e.launchCard())
	right := vbox(e.savesCard(), e.artworkCard())
	bottom := vbox()
	if e.errText != "" {
		bottom.Add(colored(e.errText, widget.DangerImportance))
	}
	saveLabel := lang.L("Save")
	if e.saving {
		saveLabel = lang.L("Saving…")
	}
	e.saveBtn = primary(saveLabel, e.save)
	setEnabled(e.saveBtn, !e.saving && strings.TrimSpace(e.name) != "")
	bottom.Add(hbox(widget.NewButton(lang.L("Cancel"), u.back), e.saveBtn))
	replaceScroll(e.scroll, vbox(pageHead(u.back, head), newCols(440, nil, left, right), bottom))
}

func (e *editor) gameCard() fyne.CanvasObject {
	u := e.u
	e.nameEntry = widget.NewEntry()
	e.nameEntry.SetPlaceHolder(lang.L("Game name"))
	e.nameEntry.SetText(e.name)
	findLabel := lang.L("Find on Steam")
	if e.searching {
		findLabel = lang.L("Searching…")
	}
	find := widget.NewButtonWithIcon(findLabel, theme.SearchIcon(), e.searchSteam)
	setEnabled(find, !e.searching && len(strings.TrimSpace(e.name)) >= 2)
	e.nameEntry.OnChanged = func(s string) {
		e.name = s
		setEnabled(find, !e.searching && len(strings.TrimSpace(s)) >= 2)
		if e.saveBtn != nil {
			setEnabled(e.saveBtn, !e.saving && strings.TrimSpace(s) != "")
		}
	}
	e.nameEntry.OnSubmitted = func(string) { e.searchSteam() }
	c := vbox(field(lang.L("Name"), rowFill(e.nameEntry, find)))

	if e.showResults {
		res := vbox()
		if len(e.steamResults) == 0 {
			res.Add(muted(lang.L("No match on Steam. You can still add the game and choose a cover yourself.")))
		}
		for _, m := range e.steamResults {
			res.Add(listRow(m.Name, m.AppID, func() {
				e.name, e.storeAppID, e.showResults = m.Name, m.AppID, false
				e.render()
				e.suggestSaves(true)
			}))
		}
		res.Add(hbox(lowButton(lang.L("Close"), func() { e.showResults = false; e.render() })))
		scroll := container.NewVScroll(res)
		scroll.SetMinSize(fyne.NewSize(0, min(260, res.MinSize().Height)))
		c.Add(card("", scroll))
	}
	if e.storeAppID != "" {
		c.Add(hbox(badge(lang.L("Steam app {{.ID}}", map[string]any{"ID": e.storeAppID}), badgeInfo), lowButton(lang.L("Unlink"), func() {
			e.storeAppID, e.showResults = "", false
			e.render()
		})))
	}

	// Cover.
	var preview fyne.CanvasObject
	size := fyne.NewSize(80, 120)
	switch {
	case e.coverSrc != "":
		preview = coverView(coverSource{path: e.coverSrc, name: filepath.Base(e.coverSrc)}, size)
	case e.gridPick != nil:
		src := e.gridPick.Thumb
		if src == "" {
			src = e.gridPick.URL
		}
		preview = coverView(coverSource{url: src}, size)
	case e.coverPath != "":
		preview = coverView(coverSource{path: e.coverPath, version: e.coverVersion}, size)
	case e.storeAppID != "":
		preview = coverView(coverSource{url: steamCoverURL(e.storeAppID), name: "?"}, size)
	default:
		preview = coverView(coverSource{name: e.name}, size)
	}
	btns := hbox(widget.NewButton(lang.L("Choose file…"), func() {
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
	side := vbox(btns)
	if e.id != "" && e.storeAppID != "" && e.storeAppID == e.originalStoreAppID && e.coverPath != "" {
		side.Add(hbox(lowButton(lang.L("Use Steam cover"), func() {
			async(func() (services.GameView, error) { return u.lib.FetchSteamCover(e.id) }, func(g services.GameView, err error) {
				if err != nil {
					u.toastErr(err)
					return
				}
				e.coverPath, e.coverVersion, e.coverSrc = g.CoverPath, g.CoverVersion, ""
				u.toast(lang.L("Cover updated from Steam"), toastOK)
				e.render()
			})
		})))
	}
	hint := lang.L("Portrait image (600×900 works well). Also used as Steam grid art.")
	switch {
	case e.coverSrc != "":
		hint = lang.L("{{.File}} — copied when you save.", map[string]any{"File": filepath.Base(e.coverSrc)})
	case e.storeAppID != "" && e.coverPath == "" && e.gridPick == nil:
		hint = lang.L("Steam's cover is downloaded when you save, unless you choose your own.")
	case e.gridPick != nil:
		hint = lang.L("SteamGridDB cover — downloaded when you save.")
	}
	side.Add(muted(hint))
	if !e.hasGridKey {
		side.Add(container.NewBorder(nil, nil, nil, lowButton(lang.L("Settings"), func() { u.navigate("/settings") }), muted(lang.L("Add a SteamGridDB API key to search community covers."))))
	}
	c.Add(container.NewBorder(nil, nil, preview, nil, container.NewPadded(side)))
	return card(lang.L("Game"), c)
}

func (e *editor) launchCard() fyne.CanvasObject {
	modes := &choice{}
	modes.add(string(library.LaunchDirect), lang.L("Carryover starts the executable (Windows, native Linux/macOS games)"))
	modes.add(string(library.LaunchSteamWrap), lang.L("Steam starts it via Proton; Carryover wraps %command% (Steam Deck / Linux)"))
	e.exeEntry = monoEntry("", e.dev.Exe, func(s string) { e.dev.Exe = s })
	setExePlaceholder := func() {
		if e.dev.LaunchMode == library.LaunchSteamWrap {
			e.exeEntry.SetPlaceHolder("/home/deck/Games/A/Game.exe")
		} else {
			e.exeEntry.SetPlaceHolder(`C:\Games\A\Game.exe`)
		}
	}
	setExePlaceholder()
	mode := selectOf(modes, string(e.dev.LaunchMode), func(v string) {
		e.dev.LaunchMode = library.LaunchMode(v)
		setExePlaceholder()
	})
	browse := widget.NewButton(lang.L("Browse…"), func() {
		e.pickFile(lang.L("Choose game executable"), nil, func(p string) {
			e.dev.Exe = p
			e.exeEntry.SetText(p)
			if e.name != "" {
				return
			}
			// Name it after the exe if it's a known game, else its folder.
			async(func() (string, error) { return e.u.lib.GuessName(p), nil }, func(name string, _ error) {
				if e.name == "" && name != "" {
					e.nameEntry.SetText(name)
				}
			})
		})
	})
	args := monoEntry("-windowed", e.dev.Args, func(s string) { e.dev.Args = s })
	wait := monoEntry("Game-Win64-Shipping.exe", e.dev.WaitProcess, func(s string) { e.dev.WaitProcess = s })
	env := monoMultiLine("", e.envText, 3, func(s string) { e.envText = s })
	workDir := newPathInput(e.u, e.id, e.tokens, e.dev.WorkDir, "", func(s string) { e.dev.WorkDir = s })
	e.advItem = newAccordionSection(lang.L("Advanced"), vbox(
		field(lang.L("Working directory (default: executable's folder)"), workDir),
		field(lang.L("Wait for process (when the exe is a launcher that exits immediately)"), wait),
		field(lang.L("Environment variables (KEY=value per line)"), env),
	), e.advanced)

	return card("", cardHead(lang.L("Launch on this device"), badge(e.u.settings.DeviceName, badgeNone)),
		field(lang.L("How is the game started?"), mode),
		field(lang.L("Executable"), rowFill(e.exeEntry, browse)),
		field(lang.L("Arguments"), args),
		e.advItem)
}

func (e *editor) savesCard() fyne.CanvasObject {
	suggestLabel := lang.L("Suggest")
	if e.suggesting {
		suggestLabel = lang.L("Looking…")
	}
	sugBtn := widget.NewButtonWithIcon(suggestLabel, theme.SearchIcon(), func() { e.suggestSaves(false) })
	setEnabled(sugBtn, !e.suggesting && (strings.TrimSpace(e.name) != "" || e.storeAppID != ""))
	c := vbox(cardHead(lang.L("Save locations"), sugBtn, widget.NewButtonWithIcon(lang.L("Add"), theme.ContentAddIcon(), e.addSlot)))

	if e.suggest != nil {
		c.Add(card("", newSuggestionList(e.name, e.suggest,
			func(s services.SaveSuggestion) { e.useSuggestion(s, nil); e.render() },
			func(s services.SaveSuggestion) { e.addSuggestion(s); e.render() },
			func() { e.suggest = nil; e.render() })))
	}
	c.Add(muted(lang.L("Each location is synced to its own cloud folder. Every device sets its own path for the same location — e.g. {{.Win}} on Windows and {{.Deck}} on a Steam Deck.",
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
		c.Add(newSlotEditor(e.u, e.id, e.tokens, s, e.others, e.uniqueKey, onRemove))
	}
	return card("", c)
}

func (e *editor) save() {
	u := e.u
	e.errText = ""
	e.saving = true
	e.render()
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
	in := services.GameInput{ID: e.id, Name: e.name, StoreAppID: e.storeAppID, Slots: slots, Device: dev}
	coverSrc, gridPick, coverPath, origStore := e.coverSrc, e.gridPick, e.coverPath, e.originalStoreAppID

	artJobs := e.artworkJobs()
	var coverErr, artErr error
	async(func() (services.GameView, error) {
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
		case in.StoreAppID != "" && (coverPath == "" || in.StoreAppID != origStore):
			// Best effort: a missing cover must not block saving the game.
			_, coverErr = u.lib.FetchSteamCover(g.ID)
		}
		artErr = applyArtwork(u.lib, g.ID, artJobs)
		return g, nil
	}, func(g services.GameView, err error) {
		e.saving = false
		if err != nil {
			e.errText = err.Error()
			e.render()
			return
		}
		if coverErr != nil {
			u.toast(lang.L("Cover: {{.Err}}", map[string]any{"Err": coverErr}), toastErr)
		}
		if artErr != nil {
			u.toast(lang.L("Artwork: {{.Err}}", map[string]any{"Err": artErr}), toastErr)
		}
		u.refresh()
		u.toast(lang.L("Saved"), toastOK)
		u.navigate("/game/" + g.ID)
	})
}
