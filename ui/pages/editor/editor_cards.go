package editor

import (
	"path/filepath"
	"runtime"
	"slices"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/lang"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/akfaiz/carryover/services"
	"github.com/akfaiz/carryover/ui/components"
	"github.com/akfaiz/carryover/ui/kit"
)

func (e *Page) gameCard() fyne.CanvasObject {
	e.nameEntry = widget.NewEntry()
	e.nameEntry.SetPlaceHolder(lang.L("Game name"))
	e.nameEntry.SetText(e.vm.name)
	findLabel := lang.L("Find on Steam")
	if e.vm.searching {
		findLabel = lang.L("Searching…")
	}
	find := widget.NewButtonWithIcon(findLabel, theme.SearchIcon(), e.searchSteam)
	kit.SetEnabled(find, !e.vm.searching && len(strings.TrimSpace(e.vm.name)) >= 2)
	e.nameEntry.OnChanged = func(s string) {
		e.vm.name = s
		kit.SetEnabled(find, !e.vm.searching && len(strings.TrimSpace(s)) >= 2)
		e.renderBar()
	}
	e.nameEntry.OnSubmitted = func(string) { e.searchSteam() }
	c := kit.VBox(kit.Field(lang.L("Name"), kit.RowFill(e.nameEntry, find)))

	if e.vm.showResults {
		res := kit.VBox()
		if len(e.vm.steamResults) == 0 {
			res.Add(kit.Muted(lang.L("No match on Steam. You can still add the game.")))
		}
		for _, m := range e.vm.steamResults {
			res.Add(kit.ListRow(m.Name, m.AppID, func() {
				e.vm.name, e.vm.storeAppID, e.vm.showResults = m.Name, m.AppID, false
				e.render()
				e.suggestSaves(true)
			}))
		}
		res.Add(kit.HBox(kit.LowButton(lang.L("Close"), func() { e.vm.showResults = false; e.render() })))
		scroll := container.NewVScroll(res)
		scroll.SetMinSize(fyne.NewSize(0, min(260, res.MinSize().Height)))
		c.Add(kit.Card("", scroll))
	}
	if e.vm.storeAppID != "" {
		c.Add(kit.HBox(kit.Badge(lang.L("Steam app {{.ID}}", map[string]any{"ID": e.vm.storeAppID}), kit.BadgeInfo), kit.LowButton(lang.L("Unlink"), func() {
			e.vm.storeAppID, e.vm.showResults = "", false
			e.render()
		})))
	}

	// Cover.
	var preview fyne.CanvasObject
	size := fyne.NewSize(80, 120)
	switch {
	case e.vm.coverSrc != "":
		preview = kit.CoverView(kit.CoverSource{Path: e.vm.coverSrc, Name: filepath.Base(e.vm.coverSrc)}, size)
	case e.vm.gridPick != nil:
		src := e.vm.gridPick.Thumb
		if src == "" {
			src = e.vm.gridPick.URL
		}
		preview = kit.CoverView(kit.CoverSource{URL: src}, size)
	case e.vm.coverPath != "":
		preview = kit.CoverView(kit.CoverSource{Path: e.vm.coverPath, Version: e.vm.coverVersion}, size)
	default:
		preview = kit.CoverView(kit.CoverSource{Name: e.vm.name}, size)
	}
	btns := kit.HBox(widget.NewButton(lang.L("Choose file…"), func() {
		e.pickFile(lang.L("Choose cover image"), []string{".png", ".jpg", ".jpeg", ".webp"}, func(p string) {
			e.vm.coverSrc, e.vm.gridPick = p, nil
			e.render()
		})
	}))
	if e.vm.hasGridKey {
		btns.Add(widget.NewButton(lang.L("Find cover…"), func() {
			e.openArtPicker(services.ArtCover)
		}))
	}
	side := kit.VBox(btns)
	hint := lang.L("Portrait image (600×900 works well). Also used as Steam grid art.")
	switch {
	case e.vm.coverSrc != "":
		hint = lang.L("{{.File}} — copied when you save.", map[string]any{"File": filepath.Base(e.vm.coverSrc)})
	case e.vm.gridPick != nil:
		hint = lang.L("SteamGridDB cover — downloaded when you save.")
	}
	side.Add(kit.Muted(hint))
	if !e.vm.hasGridKey {
		side.Add(container.NewBorder(nil, nil, nil, kit.LowButton(lang.L("Add key…"), e.askGridKey), kit.Muted(lang.L("Add a SteamGridDB API key to search community covers."))))
	}
	c.Add(container.NewBorder(nil, nil, preview, nil, container.NewPadded(side)))
	return kit.Card(lang.L("Game"), c)
}

func (e *Page) launchCard() fyne.CanvasObject {
	modes := &kit.Choice{}
	modes.Add(string(services.LaunchDirect), lang.L("Carryover"))
	if runtime.GOOS == "linux" || runtime.GOOS == "windows" || e.vm.dev.LaunchMode == services.LaunchHeroic {
		modes.Add(string(services.LaunchHeroic), lang.L("Heroic Games Launcher"))
	}
	if runtime.GOOS == "linux" || e.vm.dev.LaunchMode == services.LaunchLutris {
		modes.Add(string(services.LaunchLutris), lang.L("Lutris"))
	}
	modes.Add(string(services.LaunchSteamWrap), lang.L("Steam"))
	e.exeEntry = kit.MonoEntry("", e.vm.dev.Exe, func(s string) { e.vm.dev.Exe = s })
	hint := kit.Muted("")
	args := kit.MonoEntry("-windowed", e.vm.dev.Args, func(s string) { e.vm.dev.Args = s })
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
		switch e.vm.dev.LaunchMode {
		case services.LaunchSteamWrap:
			hint.SetText(lang.L("Steam starts the game, and Carryover syncs before and after through the game's Steam launch options (shown on the game page). On Linux, Steam chooses the Proton version."))
			e.exeEntry.SetPlaceHolder(exeExample(""))
		case services.LaunchHeroic:
			hint.SetText(lang.L("Heroic starts the game with its own settings (and, on Linux, its Wine prefix). Pick the game's executable inside its Heroic install folder; Carryover waits for it to close, then syncs."))
			e.exeEntry.SetPlaceHolder(exeExample(`Heroic\`))
		case services.LaunchLutris:
			hint.SetText(lang.L("Lutris starts the game with its own runner and Wine prefix. Pick the game's executable as set in Lutris; Carryover waits for it to close, then syncs."))
			e.exeEntry.SetPlaceHolder(exeExample(`Lutris\`))
		default:
			hint.SetText(lang.L("Carryover runs the executable below. It syncs saves before the game starts and after it closes."))
			e.exeEntry.SetPlaceHolder(exeExample(""))
		}
		// Heroic and Lutris launch the game with their own settings, so
		// arguments set here are unused.
		if e.vm.dev.LaunchMode == services.LaunchHeroic || e.vm.dev.LaunchMode == services.LaunchLutris {
			argsField.Hide()
		} else {
			argsField.Show()
		}
	}
	applyMode()
	mode := kit.SelectOf(modes, string(e.vm.dev.LaunchMode), func(v string) {
		e.vm.dev.LaunchMode = services.LaunchMode(v)
		applyMode()
	})
	browse := widget.NewButton(lang.L("Browse…"), func() {
		kit.PickProgram(e.u.Window(), lang.L("Choose game executable"), func(p string) {
			p = services.AppBundle(p) // a program picked inside a macOS .app
			e.vm.dev.Exe = p
			e.exeEntry.SetText(p)
			e.detectHeroic(p)
			e.detectLutris(p)
			if e.vm.name != "" {
				return
			}
			// Name it after the exe if it's a known game, else its folder.
			kit.Async(func() (string, error) { return e.vm.be.GuessName(p), nil }, func(name string, _ error) {
				if e.vm.name == "" && name != "" {
					e.nameEntry.SetText(name)
				}
			})
		})
	})
	wait := kit.MonoEntry("Game-Win64-Shipping.exe", e.vm.dev.WaitProcess, func(s string) { e.vm.dev.WaitProcess = s })
	env := kit.MonoMultiLine("", e.vm.envText, 3, func(s string) { e.vm.envText = s })
	workDir := components.NewPathInput(e.u, e.vm.id, e.vm.tokens, e.vm.dev.WorkDir, "", func(s string) { e.vm.dev.WorkDir = s })
	e.advItem = kit.NewAccordionSection(lang.L("Advanced"), kit.VBox(
		kit.Field(lang.L("Working directory (default: executable's folder)"), workDir),
		kit.Field(lang.L("Wait for process (when the exe is a launcher that exits immediately)"), wait),
		kit.Field(lang.L("Environment variables (KEY=value per line)"), env),
	), e.vm.advanced)

	launchTitle := lang.L("Launch on this device")
	if e.vm.kind == KindBoth {
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
	if e.vm.dev.LaunchMode == services.LaunchHeroic {
		return
	}
	type found struct {
		title string
		ok    bool
	}
	kit.Async(func() (found, error) {
		title, ok := e.vm.be.HeroicGame(exe)
		return found{title, ok}, nil
	}, func(f found, _ error) {
		if !f.ok || e.vm.dev.Exe != exe {
			return
		}
		e.vm.dev.LaunchMode = services.LaunchHeroic
		e.u.Toast(lang.L("{{.Game}} is installed by Heroic — it will be started through Heroic.", map[string]any{"Game": f.title}), kit.ToastOK)
		e.render()
	})
}

// detectLutris switches the launch mode to Lutris when exe belongs to a game
// Lutris has configured, so the game keeps Lutris's runner and Wine prefix
// instead of getting a new one from Steam.
func (e *Page) detectLutris(exe string) {
	if e.vm.dev.LaunchMode == services.LaunchLutris || e.vm.dev.LaunchMode == services.LaunchHeroic {
		return
	}
	type found struct {
		title string
		ok    bool
	}
	kit.Async(func() (found, error) {
		title, ok := e.vm.be.LutrisGame(exe)
		return found{title, ok}, nil
	}, func(f found, _ error) {
		if !f.ok || e.vm.dev.Exe != exe || e.vm.dev.LaunchMode == services.LaunchHeroic {
			return
		}
		e.vm.dev.LaunchMode = services.LaunchLutris
		e.u.Toast(lang.L("{{.Game}} is installed by Lutris — it will be started through Lutris.", map[string]any{"Game": f.title}), kit.ToastOK)
		e.render()
	})
}

func (e *Page) savesCard() fyne.CanvasObject {
	suggestLabel := lang.L("Suggest")
	if e.vm.suggesting {
		suggestLabel = lang.L("Looking…")
	}
	sugBtn := widget.NewButtonWithIcon(suggestLabel, theme.SearchIcon(), func() { e.suggestSaves(false) })
	kit.SetEnabled(sugBtn, !e.vm.suggesting && (strings.TrimSpace(e.vm.name) != "" || e.vm.storeAppID != ""))
	savesTitle := lang.L("Save locations")
	if e.vm.kind == KindBoth {
		savesTitle = lang.L("Save locations (without an emulator)")
	}
	c := kit.VBox(kit.CardHead(savesTitle, sugBtn, widget.NewButtonWithIcon(lang.L("Add"), theme.ContentAddIcon(), e.addSlot)))

	if e.vm.suggest != nil {
		c.Add(kit.Card("", newSuggestionList(e.vm.name, e.vm.suggest,
			func(s services.SaveSuggestion) { e.vm.useSuggestion(s, nil); e.render() },
			func(s services.SaveSuggestion) { e.vm.addSuggestion(s); e.render() },
			func() { e.vm.suggest = nil; e.render() })))
	}
	c.Add(components.Help(e.u, lang.L("Where the game keeps its saves on this device. Each device sets its own path."),
		lang.L("Each location is synced to its own cloud folder. Every device sets its own path for the same location — e.g. {{.Win}} on Windows and {{.Deck}} on a Steam Deck.",
			map[string]any{"Win": "{appdata}/GameA", "Deck": "{proton:shortcut}/AppData/Roaming/GameA"})))

	for i, s := range e.vm.slots {
		c.Add(widget.NewSeparator())
		var onRemove func()
		if len(e.vm.slots) > 1 {
			onRemove = func() {
				e.vm.slots = slices.Delete(e.vm.slots, i, i+1)
				e.render()
			}
		}
		c.Add(newSlotEditor(e.u, e.vm.id, e.vm.tokens, s, e.slotViewOf(s), e.vm.others, e.vm.uniqueKey, onRemove, e.renderBar))
	}
	return kit.Card("", c)
}
