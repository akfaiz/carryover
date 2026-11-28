package editor

import (
	"path/filepath"
	"runtime"
	"slices"
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/akfaiz/carryover/services"
	"github.com/akfaiz/carryover/ui/components"
	"github.com/akfaiz/carryover/ui/i18n"
	"github.com/akfaiz/carryover/ui/kit"
	"github.com/akfaiz/carryover/ui/textutil"
)

func (e *Page) gameCard(c *ui.Context) {
	kit.Card(c, i18n.L("Game"), func() {
		kit.Labeled(c, i18n.L("Name"), func() {
			ui.Row(c).Gap(8).Children(func() {
				e.field(c, "name", &e.vm.name, i18n.L("Game name"), false).Grow(1)
				findLabel := i18n.L("Find on Steam")
				if e.vm.searching {
					findLabel = i18n.L("Searching…")
				}
				e.btn(c, "find", kit.BtnSecondary, nil, findLabel, e.vm.searching || len(strings.TrimSpace(e.vm.name)) < 2, e.searchSteam)
			})
		})
		if e.vm.showResults {
			e.steamResults(c)
		}
		if e.vm.storeAppID != "" {
			ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
				kit.Badge(c, i18n.L("Steam app {{.ID}}", map[string]any{"ID": e.vm.storeAppID}), kit.BadgeInfo)
				e.btn(c, "unlink", kit.BtnSecondary, nil, i18n.L("Unlink"), false, func() { e.vm.storeAppID, e.vm.showResults = "", false })
			})
		}
		e.coverRow(c)
	})
}

// steamResults lists the Steam games found for the name.
func (e *Page) steamResults(c *ui.Context) {
	nav := e.u.Navigator()
	kit.Card(c, "", func() {
		if len(e.vm.steamResults) == 0 {
			kit.Muted(c, i18n.L("No match on Steam. You can still add the game."))
		}
		kit.ScrollBox(c, 260, 2, func() {
			for _, m := range e.vm.steamResults {
				pick := func() {
					e.vm.name, e.vm.storeAppID, e.vm.showResults = m.Name, m.AppID, false
					e.suggestSaves(true)
				}
				kit.ListButton(c, nav, [2]any{"steam-result", m.AppID}, m.Name, pick, func() {
					ui.Row(c).Gap(10).Padding(6, 8).Children(func() {
						ui.Text(c, m.Name).Grow(1)
						kit.Small(c, m.AppID)
					})
				})
			}
		})
		e.btn(c, "results-close", kit.BtnSecondary, nil, i18n.L("Close"), false, func() { e.vm.showResults = false })
	})
}

// coverRow is the cover preview with its buttons.
func (e *Page) coverRow(c *ui.Context) {
	var src kit.CoverSource
	switch {
	case e.vm.coverSrc != "":
		src = kit.CoverSource{Path: e.vm.coverSrc, Name: filepath.Base(e.vm.coverSrc)}
	case e.vm.gridPick != nil:
		src.URL = e.vm.gridPick.Thumb
		if src.URL == "" {
			src.URL = e.vm.gridPick.URL
		}
	case e.vm.coverPath != "":
		src = kit.CoverSource{Path: e.vm.coverPath, Version: e.vm.coverVersion}
	default:
		src = kit.CoverSource{Name: e.vm.name}
	}
	hint := i18n.L("Portrait image (600×900 works well). Also used as Steam grid art.")
	switch {
	case e.vm.coverSrc != "":
		hint = i18n.L("{{.File}} — copied when you save.", map[string]any{"File": filepath.Base(e.vm.coverSrc)})
	case e.vm.gridPick != nil:
		hint = i18n.L("SteamGridDB cover — downloaded when you save.")
	}
	ui.Row(c).Gap(12).AlignItems(ui.Start).Children(func() {
		kit.CoverView(c, src, 80, 120)
		ui.Column(c).Gap(6).Grow(1).Children(func() {
			ui.Row(c).Gap(8).Children(func() {
				e.btn(c, "cover-file", kit.BtnSecondary, nil, i18n.L("Choose file…"), false, func() {
					e.pickFile(i18n.L("Choose cover image"), []string{".png", ".jpg", ".jpeg", ".webp"}, "", func(p string) {
						e.vm.coverSrc, e.vm.gridPick = p, nil
					})
				})

				if e.vm.hasGridKey {
					e.btn(c, "cover-find", kit.BtnSecondary, nil, i18n.L("Find cover…"), false, func() { e.openArtPicker(services.ArtCover) })
				}
			})
			kit.Muted(c, hint)
			if !e.vm.hasGridKey {
				ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
					kit.Muted(c, i18n.L("Add a SteamGridDB API key to search community covers.")).Grow(1)
					e.btn(c, "cover-key", kit.BtnSecondary, nil, i18n.L("Add key…"), false, e.askGridKey)
				})
			}
		})
	})
}

func (e *Page) launchCard(c *ui.Context) {
	modes := &kit.Choice{}
	modes.Add(string(services.LaunchDirect), i18n.L("Carryover"))
	if runtime.GOOS == "linux" || runtime.GOOS == "windows" || e.vm.dev.LaunchMode == services.LaunchHeroic {
		modes.Add(string(services.LaunchHeroic), i18n.L("Heroic Games Launcher"))
	}
	if runtime.GOOS == "linux" || e.vm.dev.LaunchMode == services.LaunchLutris {
		modes.Add(string(services.LaunchLutris), i18n.L("Lutris"))
	}
	if runtime.GOOS == "linux" || e.vm.dev.LaunchMode == services.LaunchBottles {
		modes.Add(string(services.LaunchBottles), i18n.L("Bottles"))
	}
	modes.Add(string(services.LaunchSteamWrap), i18n.L("Steam"))
	exeExample := func(dir string) string {
		switch runtime.GOOS {
		case "windows":
			return `C:\Games\` + dir + `A\Game.exe`
		case "darwin":
			return "/Applications/GameA.app"
		}
		return "/home/deck/Games/" + strings.ReplaceAll(dir, `\`, "/") + "A/Game.exe"
	}
	var hint, placeholder string
	switch e.vm.dev.LaunchMode {
	case services.LaunchSteamWrap:
		hint = i18n.L("Steam starts the game, and Carryover syncs before and after through the game's Steam launch options (shown on the game page). On Linux, Steam chooses the Proton version.")
		placeholder = exeExample("")
	case services.LaunchHeroic:
		hint = i18n.L("Heroic starts the game with its own settings (and, on Linux, its Wine prefix). Pick the game's executable inside its Heroic install folder; Carryover waits for it to close, then syncs.")
		placeholder = exeExample(`Heroic\`)
	case services.LaunchLutris:
		hint = i18n.L("Lutris starts the game with its own runner and Wine prefix. Pick the game's executable as set in Lutris; Carryover waits for it to close, then syncs.")
		placeholder = exeExample(`Lutris\`)
	case services.LaunchBottles:
		hint = i18n.L("Bottles starts the game in its bottle. Pick the game's executable (a program added to a bottle, or any .exe inside one); Carryover waits for it to close, then syncs.")
		placeholder = "/home/deck/.local/share/bottles/bottles/Games/drive_c/GameA/Game.exe"
	default:
		hint = i18n.L("Carryover runs the executable below. It syncs saves before the game starts and after it closes.")
		placeholder = exeExample("")
	}
	title := i18n.L("Launch on this device")
	if e.vm.kind == KindBoth {
		title = i18n.L("Launch without an emulator")
	}
	kit.Card(c, "", func() {
		kit.CardHead(c, title, func() { kit.Badge(c, e.u.Store().Settings.DeviceName, kit.BadgeNone) })
		kit.Labeled(c, i18n.L("Launch with"), func() {
			kit.SelectOf(c, e.u.Navigator(), "launch-mode", i18n.L("Launch with"), modes, string(e.vm.dev.LaunchMode), func(v string) {
				e.vm.dev.LaunchMode = services.LaunchMode(v)
			})
			kit.Muted(c, hint)
		})
		kit.Labeled(c, i18n.L("Executable"), func() {
			ui.Row(c).Gap(8).Children(func() {
				e.field(c, "exe", &e.vm.dev.Exe, placeholder, true).Grow(1)
				e.btn(c, "exe-browse", kit.BtnSecondary, nil, i18n.L("Browse…"), false, func() {
					components.BrowseFrom(e.u, e.vm.id, e.vm.dev.Exe, e.pickExe)
				})

			})
		})
		// Heroic, Lutris and Bottles launch the game with their own settings,
		// so arguments set here are unused.
		if !otherLauncher(e.vm.dev.LaunchMode) {
			kit.Labeled(c, i18n.L("Arguments"), func() { e.field(c, "args", &e.vm.dev.Args, "-windowed", true) })
		}
		kit.Disclosure(c, e.u.Navigator(), "adv-open", i18n.L("Advanced"), &e.vm.advanced, func() {
			kit.Labeled(c, i18n.L("Working directory (default: executable's folder)"), func() {
				e.pathInput(&e.vm.dev.WorkDir, e.vm.id, "", true).View(c, "workdir")
			})
			kit.Labeled(c, i18n.L("Wait for process (when the exe is a launcher that exits immediately)"), func() {
				e.field(c, "wait", &e.vm.dev.WaitProcess, "Game-Win64-Shipping.exe", true)
			})
			kit.Labeled(c, i18n.L("Environment variables (KEY=value per line)"), func() { e.area(c, "env", &e.vm.envText, "", 70) })
		})
	})
}

// pickExe asks for the game's executable, starting in dir (the folder of the
// one set, "" for the dialog's default).
func (e *Page) pickExe(dir string) {
	kit.PickProgram(e.u.Window(), i18n.L("Choose game executable"), dir, func(p string) {
		p = services.AppBundle(p) // a program picked inside a macOS .app
		e.vm.dev.Exe = p
		e.detectHeroic(p)
		e.detectLutris(p)
		e.detectBottles(p)
		if e.vm.name != "" {
			return
		}
		// Name it after the exe if it's a known game, else its folder.
		kit.Async(func() (string, error) { return e.vm.be.GuessName(p), nil }, func(name string, _ error) {
			if e.vm.name == "" && name != "" {
				e.vm.name = name
			}
		})
	})
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
		e.u.Toast(i18n.L("{{.Game}} is installed by Heroic — it will be started through Heroic.", map[string]any{"Game": f.title}), kit.ToastOK)
	})
}

// detectLutris switches the launch mode to Lutris when exe belongs to a game
// Lutris has configured, so the game keeps Lutris's runner and Wine prefix
// instead of getting a new one from Steam.
func (e *Page) detectLutris(exe string) {
	if otherLauncher(e.vm.dev.LaunchMode) {
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
		e.u.Toast(i18n.L("{{.Game}} is installed by Lutris — it will be started through Lutris.", map[string]any{"Game": f.title}), kit.ToastOK)
	})
}

// detectBottles switches the launch mode to Bottles when exe is in a bottle,
// so the game runs there instead of in a new prefix from Steam. Heroic and
// Lutris win: their games may keep their prefix among the bottles.
func (e *Page) detectBottles(exe string) {
	if otherLauncher(e.vm.dev.LaunchMode) {
		return
	}
	type found struct {
		name string
		ok   bool
	}
	kit.Async(func() (found, error) {
		name, ok := e.vm.be.BottlesGame(exe)
		return found{name, ok}, nil
	}, func(f found, _ error) {
		if !f.ok || e.vm.dev.Exe != exe || otherLauncher(e.vm.dev.LaunchMode) {
			return
		}
		e.vm.dev.LaunchMode = services.LaunchBottles
		e.u.Toast(i18n.L("{{.Game}} is in Bottles — it will be started through Bottles.", map[string]any{"Game": f.name}), kit.ToastOK)
	})
}

// otherLauncher reports whether another launcher starts the game in this mode.
func otherLauncher(m services.LaunchMode) bool {
	return m == services.LaunchHeroic || m == services.LaunchLutris || m == services.LaunchBottles
}

func (e *Page) savesCard(c *ui.Context) {
	title := i18n.L("Save locations")
	if e.vm.kind == KindBoth {
		title = i18n.L("Save locations (without an emulator)")
	}
	kit.Card(c, "", func() {
		kit.CardHead(c, title, func() {
			label := i18n.L("Suggest")
			if e.vm.suggesting {
				label = i18n.L("Looking…")
			}
			e.btn(c, "suggest", kit.BtnSecondary, nil, label, e.vm.suggesting || (strings.TrimSpace(e.vm.name) == "" && e.vm.storeAppID == ""), func() { e.suggestSaves(false) })
			e.btn(c, "slot-add", kit.BtnSecondary, nil, i18n.L("Add"), false, func() { e.vm.addSlot() })
		})
		if e.vm.suggest != nil {
			e.suggestionList(c)
		}
		components.Help(c, e.u, i18n.L("Where the game keeps its saves on this device. Each device sets its own path."),
			i18n.L("Each location is synced to its own cloud folder. Every device sets its own path for the same location — e.g. {{.Win}} on Windows and {{.Deck}} on a Steam Deck.",
				map[string]any{"Win": "{appdata}/GameA", "Deck": "{proton:shortcut}/AppData/Roaming/GameA"}))
		for i, s := range e.vm.slots {
			ui.Divider(c)
			var onRemove func()
			if len(e.vm.slots) > 1 {
				onRemove = func() { e.vm.slots = slices.Delete(e.vm.slots, i, i+1) }
			}
			e.slotEditor(c, s, e.vm.others, e.vm.uniqueKey, onRemove)
		}
	})
}

// slotEditor edits one save location: its label, its path on this device and
// the include/exclude filters.
func (e *Page) slotEditor(c *ui.Context, s *slotForm, others []services.DevicePaths, uniqueKey func(base string, self *slotForm) string, onRemove func()) {
	// A new location's key follows its label until it is saved.
	if prev, ok := e.slotLabel[s]; ok && prev != s.label && s.isNew {
		s.key = uniqueKey(textutil.Slug(s.label), s)
	}
	e.slotLabel[s] = s.label
	open := e.slotOpen[s]
	if open == nil {
		open = new(bool)
		e.slotOpen[s] = open
	}
	ui.Column(c).Gap(6).Children(func() {
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			e.field(c, [2]any{"slot-label", s}, &s.label, i18n.L("Label"), false).Grow(1)
			if onRemove != nil {
				kit.IconBtn(c, e.u.Navigator(), [2]any{"slot-remove", s}, kit.BtnDanger, kit.IconTrash, i18n.L("Remove"), onRemove)
			}
		})
		kit.Labeled(c, i18n.L("Path on this device"), func() {
			e.pathInput(&s.path, e.vm.id, "{appdata}/Game/Saves", true).View(c, s)
		})
		for _, o := range others {
			if p := o.Paths[s.key]; p != "" {
				kit.Small(c, i18n.L("On {{.Device}}: {{.Path}}", map[string]any{"Device": o.DeviceName, "Path": textutil.BreakablePath(p)})).
					Font("monospace").TextColor(c.Theme().TextMuted).Wrap()
			}
		}
		kit.Disclosure(c, e.u.Navigator(), [2]any{"slot-filters", s}, i18n.L("Filters"), open, func() {
			ui.Row(c).Gap(10).Children(func() {
				ui.Column(c).Grow(1).Children(func() {
					kit.Labeled(c, i18n.L("Include only (globs, one per line)"), func() { e.area(c, [2]any{"slot-inc", s}, &s.include, "*.sav", 50) })
				})
				ui.Column(c).Grow(1).Children(func() {
					kit.Labeled(c, i18n.L("Exclude (globs)"), func() { e.area(c, [2]any{"slot-exc", s}, &s.exclude, "*.log\nshadercache/**", 50) })
				})
			})
		})
	})
}

// suggestionList is the known save locations of a game, each with Use and Add.
func (e *Page) suggestionList(c *ui.Context) {
	res := e.vm.suggest
	nav := e.u.Navigator()
	kit.Card(c, "", func() {
		if res.Matched == "" {
			kit.Muted(c, i18n.L("No known save locations for “{{.Game}}”. Try “Find on Steam” first, or set the path yourself.", map[string]any{"Game": e.vm.name}))
			return
		}
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			kit.Muted(c, i18n.L("Known save locations for {{.Game}}", map[string]any{"Game": res.Matched})).Grow(1)
			e.btn(c, "sug-hide", kit.BtnSecondary, nil, i18n.L("Hide"), false, func() { e.vm.suggest = nil })
		})
		if len(res.Suggestions) == 0 {
			kit.Muted(c, i18n.L("None that apply to this device."))
		}
		for i, s := range res.Suggestions {
			ui.Divider(c)
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
			path := kit.Mono(c, text)
			if s.Files > 0 {
				path.TextColor(c.Theme().Success)
			}
			ui.Row(c).Gap(6).Wrap().AlignItems(ui.Center).Children(func() {
				switch {
				case s.Files > 0:
					kit.Badge(c, i18n.N("found · {{.Count}} file · {{.Size}} · {{.Age}}", s.Files, map[string]any{"Count": s.Files, "Size": kit.FormatBytes(s.Bytes), "Age": components.TimeAgo(s.Newest)}), kit.BadgeOK)
				case s.Exists:
					kit.Badge(c, i18n.L("Folder exists, no saves"), kit.BadgeNone)
				default:
					kit.Badge(c, i18n.L("Not on this device"), kit.BadgeNone)
				}
				if l := kindLabel(s.Kind); l != "" {
					if s.Prefix != "" {
						l = s.Prefix // which launcher's prefix, e.g. "Heroic: Hades"
					}
					kit.Badge(c, l, kit.BadgeNone)
				}
				if s.MSStore {
					kit.Badge(c, i18n.L("Microsoft Store version"), kit.BadgeWarn)
				}
				ui.Spacer(c)
				kit.Btn(c, nav, [2]any{"sug-use", i}, kit.BtnSecondary, nil, i18n.L("Use"), func() {
					if !e.vm.useSuggestion(s, nil) {
						e.u.Toast(noMatchingLocation(), kit.ToastErr)
					}
				})
				kit.Btn(c, nav, [2]any{"sug-add", i}, kit.BtnSecondary, kit.IconPlus, i18n.L("Add"), func() { e.vm.addSuggestion(s) })
			})
			if s.Kind == "steamcloud" {
				kit.Muted(c, i18n.L("Steam Cloud already syncs this for the Steam version; useful for non-Steam copies."))
			}
		}
		ui.Row(c).Gap(4).Children(func() {
			kit.Small(c, i18n.L("From the"))
			// The pad can press it too: A opens the address in the browser.
			const manifest = "https://github.com/mtkennerly/ludusavi-manifest"
			k := "ludusavi-link"
			nav.Item(nav.Handle(k), ui.Link(c.Key(k), i18n.L("Ludusavi manifest"), manifest), func() { e.u.ToastErr(e.u.OpenURL(manifest)) })
			kit.Small(c, i18n.L("(PCGamingWiki)."))
		})
	})
}
