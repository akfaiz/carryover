// Package ui is Carryover's desktop GUI, built with Fyne.
package ui

import (
	_ "embed"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/lang"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/akfaiz/carryover/internal/app/appdir"
	"github.com/akfaiz/carryover/internal/app/singleinstance"
	"github.com/akfaiz/carryover/internal/cloudsync/syncer"
	"github.com/akfaiz/carryover/internal/core"
	"github.com/akfaiz/carryover/internal/ui/kit"
	"github.com/akfaiz/carryover/services"
)

//go:embed assets/appicon.png
var appIcon []byte

// AppID identifies the app (Fyne preferences, single instance).
const AppID = "com.akfaiz.carryover"

// page is one screen of the main window.
type page interface {
	view() fyne.CanvasObject
}

// guarded pages hold a form: leaving them while dirty asks first.
type guarded interface {
	dirty() bool
}

// searchable pages focus their search field on Ctrl+F.
type searchable interface {
	focusSearch()
}

// syncable pages start a sync on F5.
type syncable interface {
	syncNow()
}

// stateful pages are told when shared state changes.
type stateful interface {
	stateChanged(what change)
}

type change int

const (
	changeLibrary  change = iota // games or settings reloaded
	changeStatus                 // play-session status
	changeProgress               // sync transfer progress
)

// UI is the running GUI. All fields are used on the Fyne UI thread only.
type UI struct {
	app fyne.App
	win fyne.Window
	c   *core.Core

	lib   *services.LibraryService
	sync  *services.SyncService
	cloud *services.CloudService
	steam *services.SteamService
	emus  *services.EmulatorService
	sess  *services.SessionService

	games    []services.GameView
	settings appdir.Settings
	status   map[string]services.Status
	progress map[string]syncer.Event
	// syncAllProg is the latest "Sync all" progress. syncingAll is set while a
	// "Sync all" runs (it outlives the page that started it), cancellingAll once
	// it was asked to stop.
	syncAllProg               services.SyncAllProgress
	syncingAll, cancellingAll bool
	loaded                    bool
	// trashCount: deleted games and emulators in the trash, shown next to Trash
	// in the sidebar.
	trashCount int
	// helpOpen: the help texts unfolded this session, by summary.
	helpOpen map[string]bool

	route   string
	history []string
	page    page
	content *fyne.Container
	sidebar *fyne.Container
	navBtns map[string]*widget.Button
	// activity is the sidebar line that shows a running "Sync all" or game.
	activity *fyne.Container
	toasts   *kit.Toaster
	prompts  *promptQueue

	lastLocalCheck time.Time // last checkLocalChanges run
	// refreshSeq numbers the refreshes: only the newest one's result is shown, so
	// a slow older one can't replace fresher data.
	refreshSeq int
}

// startLanguage is the language setting the translations were loaded with; a
// different setting applies after a restart.
var startLanguage string

// localCheckInterval limits how often the saves are scanned for changes made
// outside Carryover (each time the window regains focus).
const localCheckInterval = 30 * time.Second

// checkLocalChanges flags games whose saves changed since their last sync,
// e.g. played without Carryover. It scans in the background.
func (u *UI) checkLocalChanges() {
	if time.Since(u.lastLocalCheck) < localCheckInterval {
		return
	}
	u.lastLocalCheck = time.Now()
	go u.lib.CheckLocalChanges()
}

func newUI(c *core.Core, a fyne.App, bus *services.Bus) *UI {
	u := &UI{
		app: a, c: c,
		status: map[string]services.Status{}, progress: map[string]syncer.Event{},
		navBtns: map[string]*widget.Button{},
		toasts:  kit.NewToaster(),
	}
	u.sess = services.NewSessionService(c, bus)
	u.lib = services.NewLibraryService(c, bus, u.sess.IsRunning)
	u.sync = services.NewSyncService(c, bus)
	u.cloud = services.NewCloudService(c)
	u.steam = services.NewSteamService(c, bus)
	u.emus = services.NewEmulatorService(c, bus)
	u.prompts = &promptQueue{u: u}

	bus.OnLibrary(func(string) { kit.OnUI(u.refresh) })
	bus.OnStatus(func(st services.Status) {
		kit.OnUI(func() {
			u.status[st.GameID] = st
			if st.Key != "" {
				u.status[st.Key] = st // the session of one emulator profile
			}
			if st.Phase == "error" {
				u.toast(st.Message, kit.ToastErr)
			}
			u.renderActivity()
			u.notify(changeStatus)
		})
	})
	bus.OnSync(func(ev syncer.Event) {
		kit.OnUI(func() {
			u.progress[ev.GameID] = ev
			u.notify(changeProgress)
		})
	})
	bus.OnSyncAll(func(p services.SyncAllProgress) {
		kit.OnUI(func() {
			u.syncAllProg = p
			u.renderActivity()
			u.notify(changeProgress)
		})
	})
	bus.OnPrompt(func(p services.Prompt) { kit.OnUI(func() { u.prompts.add(p) }) })
	bus.OnPromptClosed(func(id string) { kit.OnUI(func() { u.prompts.remove(id) }) })
	return u
}

// Run opens the GUI and blocks until it closes. With wrapGameID set it runs
// the wrapper session window (carryover run <game>) and returns the game's
// exit code.
func Run(c *core.Core, wrapGameID string, opts core.RunOptions) int {
	if wrapGameID == "" {
		inst, err := singleinstance.Acquire(c.Dirs.StateDir())
		if errors.Is(err, singleinstance.ErrRunning) {
			return 0 // the running instance was asked to show itself
		}
		if err != nil {
			log.Printf("single instance: %v", err)
		}
		if inst != nil {
			defer inst.Close()
		}
		u := start(c)
		u.buildMain()
		if inst != nil {
			inst.OnShow(func() { kit.OnUI(func() { u.win.Show(); u.win.RequestFocus() }) })
		}
		// Coming back to the launcher (e.g. after playing without it) looks
		// for saves that changed since their last sync.
		u.app.Lifecycle().SetOnEnteredForeground(func() { kit.OnUI(u.checkLocalChanges) })
		// Pick up games other devices added while this one was closed.
		u.app.Lifecycle().SetOnStarted(func() {
			kit.OnUI(u.checkLocalChanges)
			// A session killed earlier (e.g. Steam's "Exit Game") may have left
			// its cloud lock behind: release it so other devices stop warning.
			go func() {
				if err := u.sync.RecoverSessions(); err != nil {
					log.Printf("recover sessions: %v", err)
				}
			}()
			kit.Async0(u.sync.SyncLibrary, func(err error) {
				if err != nil {
					u.toast(lang.L("Library sync: {{.Err}}", map[string]any{"Err": err}), kit.ToastErr)
				}
			})
		})
		u.win.ShowAndRun()
		return 0
	}
	return start(c).runWrapper(wrapGameID, opts)
}

func start(c *core.Core) *UI {
	startLanguage = c.Settings.Get().Language
	initTranslations(startLanguage)
	a := app.NewWithID(AppID)
	kit.SystemDark.Store(kit.InitialSystemDark(a))
	kit.SetPalette(kit.PaletteFor(c.Settings.Get().Theme))
	a.Settings().SetTheme(kit.AppTheme{})
	a.SetIcon(fyne.NewStaticResource("appicon.png", appIcon))
	u := newUI(c, a, services.NewBus())
	kit.WatchSystemTheme(a, func(dark bool) {
		if kit.SystemDark.Swap(dark) == dark {
			return
		}
		kit.OnUI(func() { u.applyTheme(c.Settings.Get().Theme) })
	})
	return u
}

// buildMain creates the launcher window.
func (u *UI) buildMain() {
	u.win = u.app.NewWindow("Carryover")
	u.win.Resize(fyne.NewSize(1180, 760))
	u.win.SetMaster()
	u.win.SetCloseIntercept(u.confirmClose)
	u.buildLayout()
	u.addShortcuts()
	u.show("/")
	u.refresh()
	go func() {
		p := u.sess.Prompts()
		kit.OnUI(func() {
			for _, x := range p {
				u.prompts.add(x)
			}
		})
	}()
}

// busy reports whether a play session or a sync is running, i.e. whether
// quitting now would cut off an upload.
func (u *UI) busy() bool {
	return u.syncingAll || u.sess.Active() || u.sync.Active()
}

// confirmClose closes the main window, after asking when a game is being
// played or a sync runs: the post-play upload and the release of the cloud lock
// happen in this process, so closing it would skip both.
func (u *UI) confirmClose() {
	if !u.busy() {
		u.win.Close()
		return
	}
	kit.ConfirmDiscard(u.win, lang.L("Close Carryover?"),
		lang.L("A game is still running or a sync is in progress. If you close Carryover now, its saves won't be uploaded until the next sync, and your other devices may keep showing the game as being played."),
		lang.L("Keep open"), lang.L("Close anyway"), u.win.Close)
}

// buildLayout creates the window content (sidebar + page area); the palette
// colours are read here, so it is rebuilt when the theme changes.
func (u *UI) buildLayout() {
	u.content = container.NewStack()
	u.sidebar = container.NewVBox()
	u.navBtns = map[string]*widget.Button{}
	bg := canvas.NewRectangle(kit.ColBg2)
	sep := canvas.NewRectangle(kit.ColLine)
	sep.SetMinSize(fyne.NewSize(1, 0))
	side := container.NewStack(bg, container.NewPadded(u.sidebar))
	main := container.NewBorder(nil, nil, container.NewHBox(kit.FixedWidth(200, side), sep), nil, u.content)
	u.win.SetContent(container.NewStack(main, u.toasts.Layer))
	u.renderSidebar()
}

// applyTheme switches to the palette for a stored theme setting and rebuilds the UI.
func (u *UI) applyTheme(setting string) {
	name := kit.PaletteFor(setting)
	if (name == kit.ThemeLight) == kit.LightOn {
		return
	}
	kit.SetPalette(name)
	u.app.Settings().SetTheme(kit.AppTheme{})
	u.toasts.Recolor()
	if u.win == nil {
		return
	}
	u.buildLayout()
	u.show(u.route)
}

func (u *UI) renderSidebar() {
	logo := canvas.NewImageFromResource(fyne.NewStaticResource("appicon.png", appIcon))
	logo.FillMode = canvas.ImageFillContain
	logo.SetMinSize(fyne.NewSize(30, 30))
	brand := widget.NewLabelWithStyle("Carryover", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	brand.SizeName = theme.SizeNameSubHeadingText
	objs := []fyne.CanvasObject{container.NewHBox(logo, brand), widget.NewSeparator()}
	nav := func(path, label string, icon fyne.Resource) {
		b := widget.NewButtonWithIcon(label, icon, func() { u.navigate(path) })
		b.Alignment = widget.ButtonAlignLeading
		u.navBtns[path] = b
		objs = append(objs, b)
	}
	nav("/", lang.L("Library"), theme.GridIcon())
	nav("/emulators", lang.L("Emulators"), theme.ComputerIcon())
	nav("/cloud", lang.L("Cloud"), theme.StorageIcon())
	objs = append(objs, layout.NewSpacer())
	// Secondary places, out of the way of the main ones.
	nav("/trash", u.trashLabel(), theme.DeleteIcon())
	nav("/settings", lang.L("Settings"), theme.SettingsIcon())
	u.activity = kit.VBox()
	objs = append(objs, widget.NewSeparator(), u.activity)
	u.renderActivity()
	dev := kit.Muted(u.settings.DeviceName)
	objs = append(objs, dev)
	if u.settings.Remote != "" {
		r := kit.Mono(u.settings.Remote + ":" + u.settings.RemoteRoot)
		r.SizeName = theme.SizeNameCaptionText
		r.Importance = widget.LowImportance
		objs = append(objs, r)
	} else if u.loaded {
		b := widget.NewButtonWithIcon(lang.L("No cloud configured"), theme.WarningIcon(), func() { u.navigate("/cloud") })
		b.Importance = widget.WarningImportance
		objs = append(objs, b)
	}
	u.sidebar.Objects = objs
	u.sidebar.Refresh()
	u.highlightNav()
}

func (u *UI) highlightNav() {
	section, _, _ := strings.Cut(strings.TrimPrefix(u.route, "/"), "/")
	for p, b := range u.navBtns {
		active := false
		switch p {
		case "/":
			active = section == "" || section == "game" || section == "edit" || section == "new"
		default:
			active = "/"+section == p
		}
		if active {
			b.Importance = widget.MediumImportance
		} else {
			b.Importance = widget.LowImportance
		}
		b.Refresh()
	}
}

// navigate goes to a route, adding it to the history. Leaving a form with
// unsaved changes asks first.
func (u *UI) navigate(path string) {
	u.leave(func() {
		if u.route != "" && u.route != path {
			u.history = append(u.history, u.route)
			if len(u.history) > 50 {
				u.history = u.history[1:]
			}
		}
		u.show(path)
	})
}

// replace goes to a route in place of the current one, so Back skips it: the
// editor once it saved, a page whose game is gone. Going back to the page the
// current one was opened from also takes it off the history.
func (u *UI) replace(path string) {
	if n := len(u.history); n > 0 && u.history[n-1] == path {
		u.history = u.history[:n-1]
	}
	u.show(path)
}

// back returns to the previous route (Library if none).
func (u *UI) back() {
	u.leave(func() {
		if n := len(u.history); n > 0 {
			p := u.history[n-1]
			u.history = u.history[:n-1]
			u.show(p)
			return
		}
		u.show("/")
	})
}

// leave runs next, after asking when the current page has unsaved changes.
func (u *UI) leave(next func()) {
	if g, ok := u.page.(guarded); ok && g.dirty() {
		confirmDiscard(u, next)
		return
	}
	next()
}

func (u *UI) show(path string) {
	u.route = path
	parts := strings.Split(strings.Trim(path, "/"), "/")
	arg := func(i int) string {
		if i < len(parts) {
			return parts[i]
		}
		return ""
	}
	switch {
	case arg(0) == "game" && arg(1) != "":
		u.page = newGameDetail(u, arg(1))
	case arg(0) == "edit" && arg(1) != "":
		u.page = newEditor(u, arg(1), "")
	case arg(0) == "new" && arg(1) == "steam" && arg(2) != "":
		u.page = newEditor(u, "", arg(2))
	case arg(0) == "new" && arg(1) == "emulator":
		e := newEditor(u, "", "")
		e.kind = kindEmulator
		u.page = e
	case arg(0) == "new":
		u.page = newEditor(u, "", "")
	case arg(0) == "emulators":
		u.page = newEmulatorsPage(u)
	case arg(0) == "cloud":
		u.page = newCloudPage(u)
	case arg(0) == "trash":
		u.page = newTrashPage(u)
	case arg(0) == "settings":
		u.page = newSettingsPage(u)
	default:
		u.page = newLibraryPage(u)
	}
	u.content.Objects = []fyne.CanvasObject{u.page.view()}
	u.content.Refresh()
	u.highlightNav()
	u.renderActivity() // what it shows depends on the page
}

// refresh reloads games and settings.
func (u *UI) refresh() {
	u.refreshSeq++
	seq := u.refreshSeq
	go func() {
		games := u.lib.List()
		st := u.sync.GetSettings()
		trash := u.lib.TrashCount()
		kit.OnUI(func() {
			if seq != u.refreshSeq {
				return // a newer refresh is on its way; this result may be older
			}
			u.games, u.settings, u.loaded, u.trashCount = games, st, true, trash
			if u.sidebar != nil {
				u.renderSidebar()
			}
			u.notify(changeLibrary)
		})
	}()
}

// trashLabel is the sidebar text of the Trash entry: it counts the games and
// emulators that can be restored. (Leftover cloud saves are counted on the
// page itself: they are not something deleted from here.)
func (u *UI) trashLabel() string {
	if n := u.trashCount; n > 0 {
		return fmt.Sprintf("%s (%d)", lang.L("Trash"), n)
	}
	return lang.L("Trash")
}

// renderActivity shows what runs in the background at the bottom of the
// sidebar, whatever page is open: a "Sync all" and the games being played.
// Each entry opens the page it belongs to.
func (u *UI) renderActivity() {
	if u.activity == nil {
		return
	}
	var objs []fyne.CanvasObject
	// The library shows "Sync all" progress itself; elsewhere it shows here.
	if u.syncingAll && u.route != "/" {
		text := lang.L("Syncing…")
		if p := u.syncAllProg; p.Total > 0 {
			text = lang.L("Syncing {{.Done}} of {{.Total}}", map[string]any{"Done": p.Done, "Total": p.Total})
		}
		b := widget.NewButtonWithIcon(text, theme.ViewRefreshIcon(), func() { u.navigate("/") })
		b.Alignment, b.Importance = widget.ButtonAlignLeading, widget.LowImportance
		objs = append(objs, b)
	}
	for _, g := range u.games {
		if st, ok := u.status[g.ID]; ok && st.Running {
			id := g.ID
			b := widget.NewButtonWithIcon(lang.L("Playing {{.Game}}", map[string]any{"Game": g.Name}), theme.MediaPlayIcon(),
				func() { u.navigate("/game/" + id) })
			b.Alignment, b.Importance = widget.ButtonAlignLeading, widget.LowImportance
			objs = append(objs, b)
		}
	}
	grew := len(objs) != len(u.activity.Objects)
	u.activity.Objects = objs
	u.activity.Refresh()
	if grew && u.sidebar != nil {
		u.sidebar.Refresh() // its height changed: lay the sidebar out again
	}
}

func (u *UI) refreshSidebar() {
	if u.sidebar != nil {
		u.renderSidebar()
	}
}

func (u *UI) notify(what change) {
	if s, ok := u.page.(stateful); ok {
		s.stateChanged(what)
	}
}

func (u *UI) game(id string) (services.GameView, bool) {
	for _, g := range u.games {
		if g.ID == id {
			return g, true
		}
	}
	return services.GameView{}, false
}

// copyField is a read-only text with a Copy button that tells it copied.
func (u *UI) copyField(text string) fyne.CanvasObject {
	return kit.NewCopyField(lang.L("Copy"), text, u.app.Clipboard(), func() { u.toast(lang.L("Copied"), kit.ToastOK) })
}

func (u *UI) toast(text string, kind kit.ToastKind) {
	if kind == kit.ToastErr {
		// The log is what a bug report carries: record every error the user saw.
		log.Printf("ui error: %s", text)
	}
	u.toasts.Show(text, kind)
}

func (u *UI) toastErr(err error) {
	if err != nil {
		u.toast(err.Error(), kit.ToastErr)
	}
}

// openFolder opens a folder in the system file manager.
func (u *UI) openFolder(p string) {
	if err := u.lib.CheckFolder(p); err != nil {
		u.toastErr(err)
		return
	}
	u.toastErr(kit.OpenFolder(p))
}
