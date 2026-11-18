// Package shell is the main window of Carryover: the sidebar, the router that
// opens pages, the keyboard and gamepad handling, the toasts and the session
// prompts. It implements env.Env for the pages and knows no page itself: the
// application hands it the pages through Config.
package shell

import (
	"fmt"
	"log"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/lang"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/akfaiz/carryover/internal/core"
	"github.com/akfaiz/carryover/services"
	"github.com/akfaiz/carryover/ui/components"
	"github.com/akfaiz/carryover/ui/env"
	"github.com/akfaiz/carryover/ui/kit"
	"github.com/akfaiz/carryover/ui/prompts"
	"github.com/akfaiz/carryover/ui/store"
)

// Config is what the application plugs into the shell.
type Config struct {
	// Icon is the application icon (PNG), shown in the sidebar.
	Icon []byte
	// Page builds the page of a route ("/", "/game/<id>", ...).
	Page func(e env.Env, route string) env.Page
	// AddGame opens the "add a game" dialog (Ctrl+N).
	AddGame func(e env.Env)
	// SyncProblems lists the games a "Sync all" could not sync.
	SyncProblems func(e env.Env, total int, problems []services.SyncResult)
}

// Shell is the running main window. All fields are used on the Fyne UI thread
// only.
type Shell struct {
	cfg Config

	app fyne.App
	win fyne.Window
	c   *core.Core

	lib   *services.LibraryService
	sync  *services.SyncService
	cloud *services.CloudService
	steam *services.SteamService
	emus  *services.EmulatorService
	sess  *services.SessionService

	// store is the state shared by every page (see package store).
	store *store.Store
	// helpOpen: the help texts unfolded this session, by summary.
	helpOpen map[string]bool

	route   string
	history []string
	page    env.Page
	content *fyne.Container
	sidebar *fyne.Container
	navBtns map[string]*widget.Button
	// activity is the sidebar line that shows a running "Sync all" or game.
	activity *fyne.Container
	toasts   *kit.Toaster
	prompts  *prompts.Queue
	// nav drives the window with a gamepad; padHints says how, while one is
	// connected.
	nav      *kit.Navigator
	padHints *fyne.Container

	lastLocalCheck time.Time // last checkLocalChanges run
}

// localCheckInterval limits how often the saves are scanned for changes made
// outside Carryover (each time the window regains focus).
const localCheckInterval = 30 * time.Second

// checkLocalChanges flags games whose saves changed since their last sync,
// e.g. played without Carryover. It scans in the background.
func (u *Shell) checkLocalChanges() {
	if time.Since(u.lastLocalCheck) < localCheckInterval {
		return
	}
	u.lastLocalCheck = time.Now()
	go u.lib.CheckLocalChanges()
}

// New builds the shell around the services of c; bus carries their events.
func New(c *core.Core, a fyne.App, bus *services.Bus, cfg Config) *Shell {
	u := &Shell{
		cfg: cfg,
		app: a, c: c,
		navBtns: map[string]*widget.Button{},
		toasts:  kit.NewToaster(),
	}
	u.sess = services.NewSessionService(c, bus)
	u.lib = services.NewLibraryService(c, bus, u.sess.IsRunning)
	u.sync = services.NewSyncService(c, bus)
	u.cloud = services.NewCloudService(c)
	u.steam = services.NewSteamService(c, bus)
	u.emus = services.NewEmulatorService(c, bus)
	u.prompts = prompts.New(u)

	// A failed session is an event, not state: it toasts once, before the store
	// (registered next) tells the pages about the new status.
	bus.OnStatus(func(st services.Status) {
		if st.Phase == "error" {
			kit.OnUI(func() { u.toast(st.Message, kit.ToastErr) })
		}
	})
	u.store = store.New(store.Loader{Games: u.lib.List, Settings: u.sync.GetSettings, TrashCount: u.lib.TrashCount})
	u.store.Attach(bus)
	u.store.Subscribe(u.onStore)
	bus.OnPrompt(func(p services.Prompt) { kit.OnUI(func() { u.prompts.Add(p) }) })
	bus.OnPromptClosed(func(id string) { kit.OnUI(func() { u.prompts.Remove(id) }) })
	return u
}

// onStore keeps the window's own parts (sidebar, activity line) up to date and
// hands the change on to the open page. Pages see "Sync all" as transfer
// progress.
func (u *Shell) onStore(t store.Topic) {
	switch t {
	case store.Library:
		if u.sidebar != nil {
			u.renderSidebar()
		}
	case store.Status, store.SyncAll:
		u.renderActivity()
	}
	if t == store.SyncAll {
		t = store.Progress
	}
	u.notify(t)
}

// BuildMain creates the launcher window.
func (u *Shell) BuildMain() {
	u.win = u.app.NewWindow("Carryover")
	u.win.Resize(fyne.NewSize(1180, 760))
	u.win.SetMaster()
	u.win.SetCloseIntercept(u.confirmClose)
	u.applyFullScreen()
	u.newNavigator()
	u.buildLayout()
	u.addShortcuts()
	u.show("/")
	u.store.Refresh()
	go func() {
		p := u.sess.Prompts()
		kit.OnUI(func() {
			for _, x := range p {
				u.prompts.Add(x)
			}
		})
	}()
}

// busy reports whether a play session or a sync is running, i.e. whether
// quitting now would cut off an upload.
func (u *Shell) busy() bool {
	return u.store.SyncAll.Running || u.sess.Active() || u.sync.Active()
}

// confirmClose closes the main window, after asking when a game is being
// played or a sync runs: the post-play upload and the release of the cloud lock
// happen in this process, so closing it would skip both.
func (u *Shell) confirmClose() {
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
func (u *Shell) buildLayout() {
	u.content = container.NewStack()
	u.sidebar = container.NewVBox()
	u.navBtns = map[string]*widget.Button{}
	bg := canvas.NewRectangle(kit.ColBg2)
	sep := canvas.NewRectangle(kit.ColLine)
	sep.SetMinSize(fyne.NewSize(1, 0))
	// One group for the gamepad: up and down stay in the sidebar.
	side := container.NewStack(bg, container.NewPadded(kit.NewNavGroup(u.sidebar)))
	u.padHints = u.padHintBar()
	main := container.NewBorder(nil, u.padHints, container.NewHBox(kit.FixedWidth(200, side), sep), nil, u.content)
	u.win.SetContent(container.NewStack(main, u.toasts.Layer, u.nav.RingLayer()))
	u.renderSidebar()
}

// applyTheme switches to the palette for a stored theme setting and rebuilds the UI.
func (u *Shell) applyTheme(setting string) {
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

func (u *Shell) renderSidebar() {
	logo := canvas.NewImageFromResource(fyne.NewStaticResource("appicon.png", u.cfg.Icon))
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
	if u.showExit() {
		exit := widget.NewButtonWithIcon(lang.L("Exit"), theme.LogoutIcon(), u.confirmClose)
		exit.Alignment, exit.Importance = widget.ButtonAlignLeading, widget.LowImportance
		objs = append(objs, exit)
	}
	u.activity = kit.VBox()
	objs = append(objs, widget.NewSeparator(), u.activity)
	u.renderActivity()
	dev := kit.Muted(u.store.Settings.DeviceName)
	objs = append(objs, dev)
	if u.store.Settings.Remote != "" {
		objs = append(objs, kit.Muted(u.store.Settings.Remote+":"+u.store.Settings.RemoteRoot))
	} else if u.store.Loaded {
		b := widget.NewButtonWithIcon(lang.L("No cloud configured"), theme.WarningIcon(), func() { u.navigate("/cloud") })
		b.Importance = widget.WarningImportance
		objs = append(objs, b)
	}
	u.sidebar.Objects = objs
	u.sidebar.Refresh()
	u.highlightNav()
}

func (u *Shell) highlightNav() {
	section := sectionOf(u.route)
	for p, b := range u.navBtns {
		if p == section {
			b.Importance = widget.MediumImportance
		} else {
			b.Importance = widget.LowImportance
		}
		b.Refresh()
	}
}

// navigate goes to a route, adding it to the history. Leaving a form with
// unsaved changes asks first.
func (u *Shell) navigate(path string) {
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
func (u *Shell) replace(path string) {
	if n := len(u.history); n > 0 && u.history[n-1] == path {
		u.history = u.history[:n-1]
	}
	u.show(path)
}

// back returns to the previous route (Library if none).
func (u *Shell) back() {
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
func (u *Shell) leave(next func()) {
	if g, ok := u.page.(env.Guarded); ok && g.Dirty() {
		components.ConfirmDiscard(u, next)
		return
	}
	next()
}

func (u *Shell) show(path string) {
	u.route = path
	u.page = u.cfg.Page(u, path)
	u.content.Objects = []fyne.CanvasObject{u.page.View()}
	u.content.Refresh()
	u.highlightNav()
	u.renderActivity() // what it shows depends on the page
	u.focusForPad()
}

// trashLabel is the sidebar text of the Trash entry: it counts the games and
// emulators that can be restored. (Leftover cloud saves are counted on the
// page itself: they are not something deleted from here.)
func (u *Shell) trashLabel() string {
	if n := u.store.TrashCount; n > 0 {
		return fmt.Sprintf("%s (%d)", lang.L("Trash"), n)
	}
	return lang.L("Trash")
}

// renderActivity shows what runs in the background at the bottom of the
// sidebar, whatever page is open: a "Sync all" and the games being played.
// Each entry opens the page it belongs to.
func (u *Shell) renderActivity() {
	if u.activity == nil {
		return
	}
	var objs []fyne.CanvasObject
	// The library shows "Sync all" progress itself; elsewhere it shows here.
	if u.store.SyncAll.Running && u.route != "/" {
		text := lang.L("Syncing…")
		if p := u.store.SyncAll.Progress; p.Total > 0 {
			text = lang.L("Syncing {{.Done}} of {{.Total}}", map[string]any{"Done": p.Done, "Total": p.Total})
		}
		b := widget.NewButtonWithIcon(text, theme.ViewRefreshIcon(), func() { u.navigate("/") })
		b.Alignment, b.Importance = widget.ButtonAlignLeading, widget.LowImportance
		objs = append(objs, b)
	}
	for _, g := range u.store.Games {
		if st, ok := u.store.Status[g.ID]; ok && st.Running {
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

func (u *Shell) refreshSidebar() {
	if u.sidebar != nil {
		u.renderSidebar()
	}
}

func (u *Shell) notify(what store.Topic) {
	if s, ok := u.page.(env.Stateful); ok {
		s.StateChanged(what)
	}
}

func (u *Shell) toast(text string, kind kit.ToastKind) {
	if kind == kit.ToastErr {
		// The log is what a bug report carries: record every error the user saw.
		log.Printf("ui error: %s", text)
	}
	u.toasts.Show(text, kind)
}

func (u *Shell) toastErr(err error) {
	if err != nil {
		u.toast(err.Error(), kit.ToastErr)
	}
}

// openFolder opens a folder in the system file manager.
func (u *Shell) openFolder(p string) {
	if err := u.lib.CheckFolder(p); err != nil {
		u.toastErr(err)
		return
	}
	u.toastErr(kit.OpenFolder(p))
}
