package ui

import (
	"fmt"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/akfaiz/playrelay/internal/library"
	"github.com/akfaiz/playrelay/internal/pathvar"
	"github.com/akfaiz/playrelay/internal/steamgrid"
	"github.com/akfaiz/playrelay/internal/steamstore"
	"github.com/akfaiz/playrelay/services"
)

type slotForm struct {
	key, label, include, exclude, path string
	isNew                              bool
	keyBadge                           *fyne.Container
	filters                            *widget.AccordionItem
	filtersOpen                        bool
}

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
	advItem   *widget.AccordionItem
}

func newEditor(u *UI, id, steamAppID string) *editor {
	return &editor{u: u, id: id, steamAppID: steamAppID, loading: true,
		dev: services.DeviceInput{Paths: map[string]string{}, Env: map[string]string{}, LaunchMode: library.LaunchDirect}}
}

func (e *editor) view() fyne.CanvasObject {
	e.scroll = scrollPage(muted("Loading…"))
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
				// A Steam-installed game is started by Steam; PlayRelay wraps it via launch options.
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
			e.u.toast("Save suggestions: "+err.Error(), toastErr)
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
			e.u.toast("Found "+res.Matched+" saves on this device", toastOK)
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
			e.u.toast("Steam search failed: "+err.Error(), toastErr)
			e.showResults = false
		} else {
			e.steamResults, e.showResults = res, true
		}
		e.render()
	})
}

var exeSuffix = regexp.MustCompile(`(?i)\.(exe|sh|x86_64|AppImage)$`)

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
		e.advanced = e.advItem.Open
	}
	for _, s := range e.slots {
		if s.filters != nil {
			s.filtersOpen = s.filters.Open
		}
	}
	head := "Add game"
	if e.id != "" {
		head = "Edit game"
	}
	e.saveBtn = nil
	left := vbox(e.gameCard(), e.launchCard())
	right := vbox(e.savesCard())
	bottom := vbox()
	if e.errText != "" {
		bottom.Add(colored(e.errText, widget.DangerImportance))
	}
	saveLabel := "Save"
	if e.saving {
		saveLabel = "Saving…"
	}
	e.saveBtn = primary(saveLabel, e.save)
	setEnabled(e.saveBtn, !e.saving && strings.TrimSpace(e.name) != "")
	bottom.Add(hbox(widget.NewButton("Cancel", u.back), e.saveBtn))
	replaceScroll(e.scroll, vbox(pageHead(u.back, head), newCols(440, nil, left, right), bottom))
}

func (e *editor) gameCard() fyne.CanvasObject {
	u := e.u
	e.nameEntry = widget.NewEntry()
	e.nameEntry.SetPlaceHolder("Game name")
	e.nameEntry.SetText(e.name)
	findLabel := "Find on Steam"
	if e.searching {
		findLabel = "Searching…"
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
	c := vbox(field("Name", rowFill(e.nameEntry, find)))

	if e.showResults {
		res := vbox()
		if len(e.steamResults) == 0 {
			res.Add(muted("No match on Steam. You can still add the game and choose a cover yourself."))
		}
		for _, m := range e.steamResults {
			res.Add(listRow(m.Name, m.AppID, func() {
				e.name, e.storeAppID, e.showResults = m.Name, m.AppID, false
				e.render()
				e.suggestSaves(true)
			}))
		}
		res.Add(hbox(lowButton("Close", func() { e.showResults = false; e.render() })))
		scroll := container.NewVScroll(res)
		scroll.SetMinSize(fyne.NewSize(0, min(260, res.MinSize().Height)))
		c.Add(card("", scroll))
	}
	if e.storeAppID != "" {
		c.Add(hbox(badge("Steam app "+e.storeAppID, badgeInfo), lowButton("Unlink", func() {
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
	btns := hbox(widget.NewButton("Choose file…", func() {
		e.pickFile("Choose cover image", []string{".png", ".jpg", ".jpeg", ".webp"}, func(p string) {
			e.coverSrc, e.gridPick = p, nil
			e.render()
		})
	}))
	if e.hasGridKey {
		btns.Add(widget.NewButton("Find cover…", func() {
			showCoverPicker(u, e.name, e.storeAppID, func(g steamgrid.Grid) {
				e.gridPick, e.coverSrc = &g, ""
				e.render()
			})
		}))
	}
	side := vbox(btns)
	if e.id != "" && e.storeAppID != "" && e.storeAppID == e.originalStoreAppID && e.coverPath != "" {
		side.Add(hbox(lowButton("Use Steam cover", func() {
			async(func() (services.GameView, error) { return u.lib.FetchSteamCover(e.id) }, func(g services.GameView, err error) {
				if err != nil {
					u.toastErr(err)
					return
				}
				e.coverPath, e.coverVersion, e.coverSrc = g.CoverPath, g.CoverVersion, ""
				u.toast("Cover updated from Steam", toastOK)
				e.render()
			})
		})))
	}
	hint := "Portrait image (600×900 works well). Also used as Steam grid art."
	switch {
	case e.coverSrc != "":
		hint = filepath.Base(e.coverSrc) + " — copied when you save."
	case e.storeAppID != "" && e.coverPath == "" && e.gridPick == nil:
		hint = "Steam's cover is downloaded when you save, unless you choose your own."
	case e.gridPick != nil:
		hint = "SteamGridDB cover — downloaded when you save."
	}
	side.Add(muted(hint))
	if !e.hasGridKey {
		side.Add(container.NewBorder(nil, nil, nil, lowButton("Settings", func() { u.navigate("/settings") }), muted("Add a SteamGridDB API key to search community covers.")))
	}
	c.Add(container.NewBorder(nil, nil, preview, nil, container.NewPadded(side)))
	return card("Game", c)
}

func (e *editor) launchCard() fyne.CanvasObject {
	modes := &choice{}
	modes.add(string(library.LaunchDirect), "PlayRelay starts the executable (Windows, native Linux/macOS games)")
	modes.add(string(library.LaunchSteamWrap), "Steam starts it via Proton; PlayRelay wraps %command% (Steam Deck / Linux)")
	e.exeEntry = widget.NewEntry()
	e.exeEntry.TextStyle.Monospace = true
	e.exeEntry.SetText(e.dev.Exe)
	e.exeEntry.OnChanged = func(s string) { e.dev.Exe = s }
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
	browse := widget.NewButton("Browse…", func() {
		e.pickFile("Choose game executable", nil, func(p string) {
			e.dev.Exe = p
			e.exeEntry.SetText(p)
			if e.name == "" {
				e.nameEntry.SetText(exeSuffix.ReplaceAllString(filepath.Base(p), ""))
			}
		})
	})
	args := widget.NewEntry()
	args.TextStyle.Monospace = true
	args.SetPlaceHolder("-windowed")
	args.SetText(e.dev.Args)
	args.OnChanged = func(s string) { e.dev.Args = s }

	wait := widget.NewEntry()
	wait.TextStyle.Monospace = true
	wait.SetPlaceHolder("Game-Win64-Shipping.exe")
	wait.SetText(e.dev.WaitProcess)
	wait.OnChanged = func(s string) { e.dev.WaitProcess = s }
	env := widget.NewMultiLineEntry()
	env.TextStyle.Monospace = true
	env.SetMinRowsVisible(3)
	env.SetText(e.envText)
	env.OnChanged = func(s string) { e.envText = s }
	workDir := newPathInput(e.u, e.id, e.tokens, e.dev.WorkDir, "", func(s string) { e.dev.WorkDir = s })
	adv := widget.NewAccordionItem("Advanced", vbox(
		field("Working directory (default: executable's folder)", workDir),
		field("Wait for process (when the exe is a launcher that exits immediately)", wait),
		field("Environment variables (KEY=value per line)", env),
	))
	adv.Open = e.advanced
	e.advItem = adv
	acc := widget.NewAccordion(adv)

	return card("", cardHead("Launch on this device", badge(e.u.settings.DeviceName, badgeNone)),
		field("How is the game started?", mode),
		field("Executable", rowFill(e.exeEntry, browse)),
		field("Arguments", args),
		acc)
}

var kindLabel = map[string]string{
	"native":     "Linux/macOS native",
	"proton":     "Proton prefix",
	"install":    "install folder",
	"steamcloud": "Steam Cloud folder",
}

func (e *editor) savesCard() fyne.CanvasObject {
	suggestLabel := "Suggest"
	if e.suggesting {
		suggestLabel = "Looking…"
	}
	sugBtn := widget.NewButtonWithIcon(suggestLabel, theme.SearchIcon(), func() { e.suggestSaves(false) })
	setEnabled(sugBtn, !e.suggesting && (strings.TrimSpace(e.name) != "" || e.storeAppID != ""))
	c := vbox(cardHead("Save locations", sugBtn, widget.NewButtonWithIcon("Add", theme.ContentAddIcon(), e.addSlot)))

	if e.suggest != nil {
		sg := vbox()
		if e.suggest.Matched == "" {
			sg.Add(muted("No known save locations for “" + e.name + "”. Try “Find on Steam” first, or set the path yourself."))
		} else {
			sg.Add(container.NewBorder(nil, nil, nil, lowButton("Hide", func() { e.suggest = nil; e.render() }),
				muted("Known save locations for "+e.suggest.Matched)))
			if len(e.suggest.Suggestions) == 0 {
				sg.Add(muted("None that apply to this device."))
			}
			for _, s := range e.suggest.Suggestions {
				sg.Add(widget.NewSeparator())
				text := s.Path
				var inc []string
				for _, x := range s.Include {
					if !strings.HasSuffix(x, "/**") {
						inc = append(inc, x)
					}
				}
				if len(inc) > 0 {
					text += " · " + strings.Join(inc, ", ")
				}
				pl := mono(text)
				if s.Files > 0 {
					pl.Importance = widget.SuccessImportance
				}
				badges := hbox()
				switch {
				case s.Files > 0:
					badges.Add(badge(fmt.Sprintf("found · %d files · %s · %s", s.Files, formatBytes(s.Bytes), timeAgo(s.Newest)), badgeOK))
				case s.Exists:
					badges.Add(badge("folder exists, no saves", badgeNone))
				default:
					badges.Add(badge("not on this device", badgeNone))
				}
				if l := kindLabel[s.Kind]; l != "" {
					badges.Add(badge(l, badgeNone))
				}
				if s.MSStore {
					badges.Add(badge("Microsoft Store version", badgeWarn))
				}
				actions := hbox(widget.NewButton("Use", func() { e.useSuggestion(s, nil); e.render() }),
					lowButton("＋ Add", func() { e.addSuggestion(s); e.render() }))
				sg.Add(pl)
				sg.Add(container.NewBorder(nil, nil, nil, actions, container.NewHScroll(badges)))
				if s.Kind == "steamcloud" {
					sg.Add(muted("Steam Cloud already syncs this for the Steam version; useful for non-Steam copies."))
				}
			}
			sg.Add(hbox(small("From the"), widget.NewHyperlink("Ludusavi manifest", mustURL("https://github.com/mtkennerly/ludusavi-manifest")), small("(PCGamingWiki).")))
		}
		c.Add(card("", sg))
	}
	c.Add(muted("Each location is synced to its own cloud folder. Every device sets its own path for the same location — " +
		"e.g. {appdata}/GameA on Windows and {proton:shortcut}/AppData/Roaming/GameA on a Steam Deck."))

	for i, s := range e.slots {
		c.Add(widget.NewSeparator())
		label := widget.NewEntry()
		label.SetPlaceHolder("Label")
		label.SetText(s.label)
		s.keyBadge = container.NewCenter(badge(s.key, badgeNone))
		label.OnChanged = func(v string) {
			s.label = v
			if s.isNew {
				s.key = e.uniqueKey(slug(v), s)
				s.keyBadge.Objects = []fyne.CanvasObject{badge(s.key, badgeNone)}
				s.keyBadge.Refresh()
			}
		}
		right := []fyne.CanvasObject{s.keyBadge}
		if len(e.slots) > 1 {
			rm := widget.NewButtonWithIcon("", theme.DeleteIcon(), func() {
				e.slots = slices.Delete(e.slots, i, i+1)
				e.render()
			})
			rm.Importance = widget.DangerImportance
			right = append(right, rm)
		}
		c.Add(rowFill(label, right...))
		c.Add(field("Path on this device", newPathInput(e.u, e.id, e.tokens, s.path, "{appdata}/Game/Saves", func(v string) { s.path = v })))
		for _, o := range e.others {
			if p := o.Paths[s.key]; p != "" {
				l := mono("On " + o.DeviceName + ": " + p)
				l.Importance = widget.LowImportance
				l.SizeName = theme.SizeNameCaptionText
				c.Add(l)
			}
		}
		inc := widget.NewMultiLineEntry()
		inc.TextStyle.Monospace = true
		inc.SetPlaceHolder("*.sav")
		inc.SetMinRowsVisible(2)
		inc.SetText(s.include)
		inc.OnChanged = func(v string) { s.include = v }
		exc := widget.NewMultiLineEntry()
		exc.TextStyle.Monospace = true
		exc.SetPlaceHolder("*.log\nshadercache/**")
		exc.SetMinRowsVisible(2)
		exc.SetText(s.exclude)
		exc.OnChanged = func(v string) { s.exclude = v }
		filters := widget.NewAccordionItem("Filters", container.NewGridWithColumns(2,
			field("Include only (globs, one per line)", inc), field("Exclude (globs)", exc)))
		filters.Open = s.filtersOpen
		s.filters = filters
		c.Add(widget.NewAccordion(filters))
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

	var coverErr error
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
		return g, nil
	}, func(g services.GameView, err error) {
		e.saving = false
		if err != nil {
			e.errText = err.Error()
			e.render()
			return
		}
		if coverErr != nil {
			u.toast("Cover: "+coverErr.Error(), toastErr)
		}
		u.refresh()
		u.toast("Saved", toastOK)
		u.navigate("/game/" + g.ID)
	})
}
