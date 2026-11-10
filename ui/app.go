// Package ui is Carryover's desktop GUI, built with Fyne.
package ui

import (
	_ "embed"
	"errors"
	"log"
	"os/exec"
	"runtime"
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

	"github.com/akfaiz/carryover/internal/appdir"
	"github.com/akfaiz/carryover/internal/core"
	"github.com/akfaiz/carryover/internal/singleinstance"
	"github.com/akfaiz/carryover/internal/syncer"
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
	sess  *services.SessionService

	games    []services.GameView
	settings appdir.Settings
	status   map[string]services.Status
	progress map[string]syncer.Event
	loaded   bool

	route   string
	history []string
	page    page
	content *fyne.Container
	sidebar *fyne.Container
	navBtns map[string]*widget.Button
	toasts  *toaster
	prompts *promptQueue
}

func newUI(c *core.Core, a fyne.App, bus *services.Bus) *UI {
	u := &UI{
		app: a, c: c,
		status: map[string]services.Status{}, progress: map[string]syncer.Event{},
		navBtns: map[string]*widget.Button{},
		toasts:  newToaster(),
	}
	u.sess = services.NewSessionService(c, bus)
	u.lib = services.NewLibraryService(c, bus, u.sess.IsRunning)
	u.sync = services.NewSyncService(c, bus)
	u.cloud = services.NewCloudService(c)
	u.steam = services.NewSteamService(c, bus)
	u.prompts = &promptQueue{u: u}

	bus.OnLibrary(func(string) { onUI(u.refresh) })
	bus.OnStatus(func(st services.Status) {
		onUI(func() {
			u.status[st.GameID] = st
			if st.Phase == "error" {
				u.toast(st.Message, toastErr)
			}
			u.notify(changeStatus)
		})
	})
	bus.OnSync(func(ev syncer.Event) {
		onUI(func() {
			u.progress[ev.GameID] = ev
			u.notify(changeProgress)
		})
	})
	bus.OnPrompt(func(p services.Prompt) { onUI(func() { u.prompts.add(p) }) })
	bus.OnPromptClosed(func(id string) { onUI(func() { u.prompts.remove(id) }) })
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
			inst.OnShow(func() { onUI(func() { u.win.Show(); u.win.RequestFocus() }) })
		}
		// Pick up games other devices added while this one was closed.
		u.app.Lifecycle().SetOnStarted(func() {
			async0(u.sync.SyncLibrary, func(err error) {
				if err != nil {
					u.toast(lang.L("Library sync: {{.Err}}", map[string]any{"Err": err}), toastErr)
				}
			})
		})
		u.win.ShowAndRun()
		return 0
	}
	return start(c).runWrapper(wrapGameID, opts)
}

func start(c *core.Core) *UI {
	initTranslations()
	a := app.NewWithID(AppID)
	setPalette(c.Settings.Get().Theme)
	a.Settings().SetTheme(appTheme{})
	a.SetIcon(fyne.NewStaticResource("appicon.png", appIcon))
	return newUI(c, a, services.NewBus())
}

// buildMain creates the launcher window.
func (u *UI) buildMain() {
	u.win = u.app.NewWindow("Carryover")
	u.win.Resize(fyne.NewSize(1180, 760))
	u.win.SetMaster()
	u.buildLayout()
	u.show("/")
	u.refresh()
	go func() {
		p := u.sess.Prompts()
		onUI(func() {
			for _, x := range p {
				u.prompts.add(x)
			}
		})
	}()
}

// buildLayout creates the window content (sidebar + page area); the palette
// colours are read here, so it is rebuilt when the theme changes.
func (u *UI) buildLayout() {
	u.content = container.NewStack()
	u.sidebar = container.NewVBox()
	u.navBtns = map[string]*widget.Button{}
	bg := canvas.NewRectangle(colBg2)
	sep := canvas.NewRectangle(colLine)
	sep.SetMinSize(fyne.NewSize(1, 0))
	side := container.NewStack(bg, container.NewPadded(u.sidebar))
	main := container.NewBorder(nil, nil, container.NewHBox(fixedWidth(200, side), sep), nil, u.content)
	u.win.SetContent(container.NewStack(main, u.toasts.layer))
	u.renderSidebar()
}

// applyTheme switches between the dark and light palette and rebuilds the UI.
func (u *UI) applyTheme(name string) {
	if (name == themeLight) == lightOn {
		return
	}
	setPalette(name)
	u.app.Settings().SetTheme(appTheme{})
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
	for _, n := range []struct{ path, label string }{{"/", lang.L("Library")}, {"/cloud", lang.L("Cloud")}, {"/settings", lang.L("Settings")}} {
		b := widget.NewButton(n.label, func() { u.navigate(n.path) })
		b.Alignment = widget.ButtonAlignLeading
		u.navBtns[n.path] = b
		objs = append(objs, b)
	}
	u.navBtns["/"].SetIcon(theme.GridIcon())
	u.navBtns["/cloud"].SetIcon(theme.StorageIcon())
	u.navBtns["/settings"].SetIcon(theme.SettingsIcon())
	objs = append(objs, layout.NewSpacer(), widget.NewSeparator())
	dev := muted(u.settings.DeviceName)
	objs = append(objs, dev)
	if u.settings.Remote != "" {
		r := mono(u.settings.Remote + ":" + u.settings.RemoteRoot)
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

// navigate goes to a route, adding it to the history.
func (u *UI) navigate(path string) {
	if u.route != "" && u.route != path {
		u.history = append(u.history, u.route)
		if len(u.history) > 50 {
			u.history = u.history[1:]
		}
	}
	u.show(path)
}

// back returns to the previous route (Library if none).
func (u *UI) back() {
	if n := len(u.history); n > 0 {
		p := u.history[n-1]
		u.history = u.history[:n-1]
		u.show(p)
		return
	}
	u.show("/")
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
	case arg(0) == "new":
		u.page = newEditor(u, "", "")
	case arg(0) == "cloud":
		u.page = newCloudPage(u)
	case arg(0) == "settings":
		u.page = newSettingsPage(u)
	default:
		u.page = newLibraryPage(u)
	}
	u.content.Objects = []fyne.CanvasObject{u.page.view()}
	u.content.Refresh()
	u.highlightNav()
}

// refresh reloads games and settings.
func (u *UI) refresh() {
	go func() {
		games := u.lib.List()
		st := u.sync.GetSettings()
		onUI(func() {
			u.games, u.settings, u.loaded = games, st, true
			if u.sidebar != nil {
				u.renderSidebar()
			}
			u.notify(changeLibrary)
		})
	}()
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

func (u *UI) toast(text string, kind toastKind) {
	u.toasts.show(text, kind)
}

func (u *UI) toastErr(err error) {
	if err != nil {
		u.toast(err.Error(), toastErr)
	}
}

// async runs work off the UI thread and then done on it.
func async[T any](work func() (T, error), done func(T, error)) {
	go func() {
		v, err := work()
		onUI(func() { done(v, err) })
	}()
}

// async0 is async for work without a result.
func async0(work func() error, done func(error)) {
	go func() {
		err := work()
		onUI(func() { done(err) })
	}()
}

// openFolder opens a folder in the system file manager.
func (u *UI) openFolder(p string) {
	if err := u.lib.CheckFolder(p); err != nil {
		u.toastErr(err)
		return
	}
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("explorer", p)
	case "darwin":
		cmd = exec.Command("open", p)
	default:
		cmd = exec.Command("xdg-open", p)
	}
	if err := cmd.Start(); err != nil {
		u.toastErr(err)
		return
	}
	go func() { _ = cmd.Wait() }()
}

// pagePadded gives page content the same margin on every side.
func pagePadded(content fyne.CanvasObject) fyne.CanvasObject {
	const m = 28
	return container.New(layout.NewCustomPaddedLayout(m, m, m, m), content)
}

// scrollPage wraps page content in a padded vertical scroll.
func scrollPage(content fyne.CanvasObject) *container.Scroll {
	return container.NewVScroll(pagePadded(content))
}

// replaceScroll swaps a scroll's content, keeping its position.
func replaceScroll(s *container.Scroll, content fyne.CanvasObject) {
	off := s.Offset
	s.Content = pagePadded(content)
	s.Refresh()
	s.Offset = off
	s.Refresh()
}

// debounce returns a function that runs f on the UI thread d after its last call.
func debounce(d time.Duration, f func()) func() {
	var t *time.Timer
	return func() {
		if t != nil {
			t.Stop()
		}
		t = time.AfterFunc(d, func() { onUI(f) })
	}
}

// pageHead is a page title with an optional back button and actions.
func pageHead(back func(), text string, actions ...fyne.CanvasObject) fyne.CanvasObject {
	var objs []fyne.CanvasObject
	if back != nil {
		b := widget.NewButtonWithIcon("", theme.NavigateBackIcon(), back)
		b.Importance = widget.LowImportance
		objs = append(objs, b)
	}
	objs = append(objs, title(text), layout.NewSpacer())
	objs = append(objs, actions...)
	return container.NewHBox(objs...)
}

// onUI runs f on the Fyne UI goroutine. It is a variable so tests can
// serialize UI work (Fyne's test driver runs fyne.Do inline on the caller).
var onUI = fyne.Do
