// Package shell is the main window of Carryover: the sidebar, the router that
// opens pages, the keyboard and gamepad handling, the toasts and the session
// prompts. It implements env.Env for the pages and knows no page itself: the
// application hands it the pages through Config.
//
// This is the MyGo build. MyGo draws the window again from this code every
// frame, so the shell keeps plain state (route, history, the dialog open) and
// the view reads it; nothing here builds or updates widgets.
package shell

import (
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"github.com/akfaiz/carryover/internal/app/gamemode"
	"github.com/akfaiz/carryover/internal/core"
	"github.com/akfaiz/carryover/services"
	"github.com/akfaiz/carryover/ui/env"
	"github.com/akfaiz/carryover/ui/i18n"
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

// Shell is the running main window. All fields are used on the UI thread only.
type Shell struct {
	cfg Config

	win *mygo.Window
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

	nav     *kit.Navigator
	confirm kit.Confirm
	dialogs kit.Dialogs
	prompts *prompts.Queue
	prefs   *prefs
	logo    *ui.Bitmap
	// toastQ holds toasts asked for outside a frame; the view shows them.
	toastQ []toastReq
	// themeDirty asks the next frame to apply the palette again.
	themeDirty bool
	closing    bool
	wrapper    bool // the window is the wrapper of `carryover run`
	stopPad    func()

	sideH     [5]ui.Handle // the places of the sidebar
	exitH     ui.Handle
	cloudH    ui.Handle
	activityH [8]ui.Handle // running games, "Sync all"

	lastLocalCheck time.Time // last checkLocalChanges run
}

type toastReq struct {
	text string
	kind kit.ToastKind
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
func New(c *core.Core, bus *services.Bus, cfg Config) *Shell {
	u := &Shell{cfg: cfg, c: c, prefs: loadPrefs(c.Dirs.StateDir())}
	if cfg.Icon != nil {
		u.logo, _ = ui.DecodeBitmap(cfg.Icon)
	}
	u.sess = services.NewSessionService(c, bus)
	u.lib = services.NewLibraryService(c, bus, u.sess.IsRunning)
	u.sync = services.NewSyncService(c, bus)
	u.cloud = services.NewCloudService(c)
	u.steam = services.NewSteamService(c, bus)
	u.emus = services.NewEmulatorService(c, bus)
	u.nav = kit.NewNavigator(u.navOptions())

	// A failed session is an event, not state: it toasts once, before the store
	// (registered next) tells the pages about the new status.
	bus.OnStatus(func(st services.Status) {
		if st.Phase == "error" {
			kit.OnUI(func() { u.toast(st.Message, kit.ToastErr) })
		}
	})
	u.store = store.New(store.Loader{Games: u.lib.List, Settings: u.sync.GetSettings, TrashCount: u.lib.TrashCount},
		func(f func()) { kit.OnUI(f) }) // a closure: tests swap kit.OnUI
	u.store.Attach(bus)
	u.store.Subscribe(u.onStore)
	u.prompts = prompts.New(u)
	bus.OnPrompt(func(p services.Prompt) { kit.OnUI(func() { u.prompts.Add(p) }) })
	bus.OnPromptClosed(func(id string) { kit.OnUI(func() { u.prompts.Remove(id) }) })
	return u
}

// onStore hands a change to the open page. Pages see "Sync all" as transfer
// progress. The window redraws by itself: kit.OnUI invalidates it.
func (u *Shell) onStore(t store.Topic) {
	if t == store.SyncAll {
		t = store.Progress
	}
	if s, ok := u.page.(env.Stateful); ok {
		s.StateChanged(t)
	}
}

// BuildMain creates the launcher window. Call it once MyGo is ready.
func (u *Shell) BuildMain() {
	kit.SystemDark.Store(mygo.Theme.IsDark())
	mygo.Theme.OnUpdated(func() {
		kit.OnUI(func() {
			kit.SystemDark.Store(mygo.Theme.IsDark())
			u.ApplyTheme(u.store.Settings.Theme)
		})
	})
	u.win = mygo.NewWindow(mygo.WindowOptions{
		Title:   "Carryover",
		Width:   1180,
		Height:  760,
		Content: ui.View(u.view),
	})
	u.win.OnClose(u.onClose)
	u.applyFullScreen()
	u.Init()
	u.readPad(true)
}

// Init shows the first page and loads the state. BuildMain calls it once the
// window exists; tests call it on a shell with no window.
func (u *Shell) Init() {
	u.show("/")
	u.store.Refresh()
	go func() { // prompts a session raised before this window existed
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

// onClose lets the window close, after asking when a game is being played or a
// sync runs: the post-play upload and the release of the cloud lock happen in
// this process, so closing it would skip both.
func (u *Shell) onClose(e *mygo.CloseEvent) {
	if u.closing || !u.busy() {
		return
	}
	e.PreventDefault()
	u.askClose()
}

func (u *Shell) askClose() {
	if !u.busy() {
		u.closing = true
		u.win.Close()
		return
	}
	u.confirm.Show(kit.ConfirmOptions{
		Title:  i18n.L("Close Carryover?"),
		Body:   i18n.L("A game is still running or a sync is in progress. If you close Carryover now, its saves won't be uploaded until the next sync, and your other devices may keep showing the game as being played."),
		Cancel: i18n.L("Keep open"), OK: i18n.L("Close anyway"), Danger: true,
	}, func() { u.closing = true; u.win.Close() })
	kit.Redraw()
}

// frame is what every window of the shell shares around its content: the
// theme, the gamepad's registry, toasts and the yes/no dialog.
func (u *Shell) frame(c *ui.Context, shortcuts bool, content func()) {
	if u.themeDirty || c.Theme().Background != kitTheme().Background {
		c.SetTheme(kitTheme())
		u.themeDirty = false
	}
	u.nav.Begin(c)
	u.showToasts(c)
	if shortcuts {
		u.shortcuts(c)
	}
	c.Root().Background(c.Theme().Background)
	content()
	u.dialogs.View(c, u.nav)
	u.confirm.View(c, u.nav)
	u.nav.Keys(c)
	u.nav.End(c)
}

// view is the main window's content, built every frame.
func (u *Shell) view(c *ui.Context) {
	u.frame(c, true, func() {
		ui.Column(c).Fill().Children(func() {
			ui.Row(c).Grow(1).AlignItems(ui.Stretch).Children(func() {
				u.sidebar(c)
				ui.Column(c).Grow(1).Padding(0, 0, 0, 16).Children(func() {
					if u.page != nil {
						u.nav.MarkPage()
						u.page.View(c)
					}
				})
			})
			u.padHints(c)
		})
	})
}

// wrapperView is the wrapper window's content: the page alone.
func (u *Shell) wrapperView(c *ui.Context) {
	u.frame(c, false, func() {
		ui.Column(c).Fill().Children(func() {
			u.nav.MarkPage()
			u.page.View(c)
		})
	})
}

// kitTheme is the palette as a theme (a func so tests can stand in).
var kitTheme = kit.AppTheme

func (u *Shell) shortcuts(c *ui.Context) {
	if u.confirm.Open() || u.dialogs.Open() {
		return
	}
	c.OnShortcut(ui.Cmd, ui.KeyF, func() {
		if s, ok := u.page.(env.Searchable); ok {
			s.FocusSearch()
		}
	})
	c.OnShortcut(ui.Cmd, ui.KeyN, func() { u.cfg.AddGame(u) })
	if c.Shortcut(0, ui.KeyEscape) && u.route != "/" {
		u.back()
	}
	if c.Shortcut(0, ui.KeyF5) {
		if s, ok := u.page.(env.Syncable); ok {
			s.SyncNow()
		}
	}
	if c.Shortcut(0, ui.KeyF11) {
		u.toggleFullScreen()
	}
}

// showToasts hands the toasts asked for since the last frame to MyGo.
func (u *Shell) showToasts(c *ui.Context) {
	for _, t := range u.toastQ {
		typ := "info"
		switch t.kind {
		case kit.ToastOK:
			typ = "success"
		case kit.ToastErr:
			typ = "error"
		}
		c.AddToast(ui.Toast{Description: t.text, Type: typ})
	}
	u.toastQ = u.toastQ[:0]
}

// sidebarPlaces are the sidebar's entries in order; the labels are looked up
// when drawn, since text goes through i18n.
func (u *Shell) sidebarPlaces() []struct{ path, label string } {
	return []struct{ path, label string }{
		{"/", i18n.L("Library")},
		{"/emulators", i18n.L("Emulators")},
		{"/cloud", i18n.L("Cloud")},
		{"/trash", u.trashLabel()},
		{"/settings", i18n.L("Settings")},
	}
}

func (u *Shell) sidebar(c *ui.Context) {
	t := c.Theme()
	ui.Column(c).Width(216).Padding(12).Gap(4).Background(t.Surface).Children(func() {
		ui.Row(c).Gap(8).AlignItems(ui.Center).Padding(0, 0, 8, 0).Children(func() {
			if u.logo != nil {
				ui.Image(c, u.logo).Size(30, 30)
			}
			ui.Text(c, "Carryover").FontSize(16).Bold()
		})
		ui.Divider(c)
		places := u.sidebarPlaces()
		section := sectionOf(u.route)
		item := func(i int) {
			p := places[i]
			var b ui.Element
			if p.path == section {
				b = ui.PrimaryButton(c.Key([2]any{"side", i}), p.label)
			} else {
				b = ui.Button(c.Key([2]any{"side", i}), p.label)
			}
			go1 := func() { u.navigate(p.path) }
			if u.nav.Item(&u.sideH[i], b.AlignSelf(ui.Stretch), go1).Clicked() {
				go1()
			}
		}
		for i := range 3 {
			item(i)
		}
		ui.Spacer(c)
		for i := 3; i < len(places); i++ {
			item(i)
		}
		if u.showExit() {
			exit := u.askClose
			if u.nav.Item(&u.exitH, ui.Button(c.Key("exit"), i18n.L("Exit")).AlignSelf(ui.Stretch), exit).Clicked() {
				exit()
			}
		}
		ui.Divider(c)
		u.activity(c)
		s := &u.store.Settings
		ui.Text(c, s.DeviceName).FontSize(12).TextColor(t.TextMuted)
		switch {
		case s.Remote != "":
			ui.Text(c, s.Remote+":"+s.RemoteRoot).FontSize(12).TextColor(t.TextMuted)
		case u.store.Loaded:
			cloud := func() { u.navigate("/cloud") }
			if u.nav.Item(&u.cloudH, ui.Button(c.Key("nocloud"), i18n.L("No cloud configured")).TextColor(t.Warning).AlignSelf(ui.Stretch), cloud).Clicked() {
				cloud()
			}
		}
	})
}

// activity shows what runs in the background at the bottom of the sidebar,
// whatever page is open: a "Sync all" and the games being played. Each entry
// opens the page it belongs to.
func (u *Shell) activity(c *ui.Context) {
	n := 0
	entry := func(label string, to string) {
		if n >= len(u.activityH) {
			return
		}
		h := &u.activityH[n]
		n++
		open := func() { u.navigate(to) }
		if u.nav.Item(h, ui.Button(c.Key([2]any{"act", n}), label).AlignSelf(ui.Stretch), open).Clicked() {
			open()
		}
	}
	// The library shows "Sync all" progress itself; elsewhere it shows here.
	if u.store.SyncAll.Running && u.route != "/" {
		text := i18n.L("Syncing…")
		if p := u.store.SyncAll.Progress; p.Total > 0 {
			text = i18n.L("Syncing {{.Done}} of {{.Total}}", map[string]any{"Done": p.Done, "Total": p.Total})
		}
		entry(text, "/")
	}
	for _, g := range u.store.Games {
		if st, ok := u.store.Status[g.ID]; ok && st.Running {
			entry(i18n.L("Playing {{.Game}}", map[string]any{"Game": g.Name}), "/game/"+g.ID)
		}
	}
}

// padHints says what the gamepad's buttons do; it shows while one is connected.
func (u *Shell) padHints(c *ui.Context) {
	if !u.nav.PadPresent() {
		return
	}
	t := c.Theme()
	hints := []struct{ button, what string }{
		{"A", i18n.L("Select")}, {"B", i18n.L("Back")}, {"LB / RB", i18n.L("Section")},
		{"X", i18n.L("Search")}, {"Y", i18n.L("Sync")}, {"R", i18n.L("Scroll")},
	}
	if !gamemode.Active() { // Game Mode is always full screen
		hints = append(hints, struct{ button, what string }{"View", i18n.L("Full screen")})
	}
	ui.Row(c).Gap(14).Padding(8, 16).Justify(ui.End).Children(func() {
		for _, h := range hints {
			ui.Row(c).Gap(4).AlignItems(ui.Center).Children(func() {
				ui.Badge(c, h.button)
				ui.Text(c, h.what).FontSize(12).TextColor(t.TextMuted)
			})
		}
	})
}

// ApplyTheme switches to the palette for a stored theme setting.
func (u *Shell) ApplyTheme(setting string) {
	name := kit.PaletteFor(setting)
	if (name == kit.ThemeLight) == kit.LightOn {
		return
	}
	kit.SetPalette(name)
	u.themeDirty = true
	kit.Redraw()
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
		u.confirm.Show(kit.ConfirmOptions{
			Title:  i18n.L("Discard changes?"),
			Body:   i18n.L("Your changes on this page haven't been saved."),
			Cancel: i18n.L("Keep editing"), OK: i18n.L("Discard changes"), Danger: true,
		}, next)
		return
	}
	next()
}

func (u *Shell) show(path string) {
	u.route = path
	u.page = u.cfg.Page(u, path)
	if s, ok := u.page.(env.Scrollable); ok {
		u.nav.SetScroll(s.Scroll())
	} else {
		u.nav.SetScroll(nil)
	}
	kit.Redraw()
}

// trashLabel is the sidebar text of the Trash entry: it counts the games and
// emulators that can be restored. (Leftover cloud saves are counted on the
// page itself: they are not something deleted from here.)
func (u *Shell) trashLabel() string {
	if n := u.store.TrashCount; n > 0 {
		return fmt.Sprintf("%s (%d)", i18n.L("Trash"), n)
	}
	return i18n.L("Trash")
}

func (u *Shell) toast(text string, kind kit.ToastKind) {
	if kind == kit.ToastErr {
		// The log is what a bug report carries: record every error the user saw.
		log.Printf("ui error: %s", text)
	}
	u.toastQ = append(u.toastQ, toastReq{text, kind})
	kit.Redraw()
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

// syncAll syncs every game that is set up on this device; called again while
// it runs, it cancels. It belongs to the window, not to the library page, so
// it keeps running (and is shown in the sidebar) on any page.
func (u *Shell) syncAll() {
	if u.store.SyncAll.Running {
		if !u.store.SyncAll.Cancelling && u.sync.Cancel("") {
			u.store.SyncAllCancelling()
		}
		return
	}
	u.store.BeginSyncAll()
	kit.Async(u.sync.SyncAll, func(res []services.SyncResult, err error) {
		u.store.EndSyncAll()
		defer u.store.Refresh()
		if errors.Is(err, services.ErrSyncCancelled) {
			u.toast(i18n.N("Sync cancelled after {{.Count}} game", len(res), map[string]any{"Count": len(res)}), kit.ToastInfo)
			return
		}
		if services.IsOffline(err) {
			u.toast(i18n.L("Offline — nothing synced"), kit.ToastInfo)
			return
		}
		if err != nil {
			u.toastErr(err)
			return
		}
		var problems []services.SyncResult
		for _, r := range res {
			if r.Error != "" || r.Action == "conflict" {
				problems = append(problems, r)
			}
		}
		if len(problems) > 0 {
			u.cfg.SyncProblems(u, len(res), problems)
			return
		}
		u.toast(i18n.N("Synced {{.Count}} game", len(res), map[string]any{"Count": len(res)}), kit.ToastOK)
	})
}

// showNotices shows, once, what session watchers did while no notification
// could be shown (e.g. in Steam Deck Game Mode).
func (u *Shell) showNotices() {
	go func() {
		notices := u.sync.TakeNotices()
		if len(notices) == 0 {
			return
		}
		kit.OnUI(func() {
			for _, n := range notices {
				u.toast(noticeBody(n), noticeToastKind(n.Kind))
			}
		})
	}()
}

func noticeToastKind(k core.NoticeKind) kit.ToastKind {
	switch k {
	case core.NoticeUploaded:
		return kit.ToastOK
	case core.NoticeConflict, core.NoticeOffline:
		return kit.ToastInfo
	}
	return kit.ToastErr
}
