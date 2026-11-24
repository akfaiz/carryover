package settings

import (
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/akfaiz/carryover/internal/app/appdir"
	"github.com/akfaiz/carryover/internal/app/version"
	"github.com/akfaiz/carryover/internal/cloudsync/syncer"
	"github.com/akfaiz/carryover/services"
	"github.com/akfaiz/carryover/ui/components"
	"github.com/akfaiz/carryover/ui/env"
	"github.com/akfaiz/carryover/ui/flows"
	"github.com/akfaiz/carryover/ui/i18n"
	"github.com/akfaiz/carryover/ui/kit"
)

// Options plugs in what this page shows but does not own.
type Options struct {
	// SteamArt supplies Carryover's own library artwork by kind (services.SelfArtKinds)
	// for the card that adds Carryover to Steam. Without it the card is left out.
	SteamArt func() map[string][]byte
}

// Page is the Settings page. Every setting saves as soon as it changes: the
// page keeps what the fields show, notices a change each frame, and has the
// ViewModel save it (text after a pause, anything else at once).
type Page struct {
	u      env.Env
	opts   Options
	vm     *ViewModel // of the form shown; set once the settings are loaded
	devs   []services.DeviceView
	scroll ui.ScrollState

	// What the fields show.
	f        formState
	seen     Form // the fields as of the last frame
	later    func()
	savedAt  time.Time
	keyTest  *kit.TestButton
	keyDirty bool
	steam    *selfSteamCard
	langs    kit.Choice
}

// formState are the fields of the form.
type formState struct {
	name, id  string
	backups   bool
	retention string
	theme     string // a kit.Theme* setting
	language  string
	share     bool
	covers    bool
	key       string
}

// New returns the Settings page.
func New(u env.Env, opts Options) *Page {
	p := &Page{u: u, opts: opts}
	p.reload()
	return p
}

// Scroll is the page's scroll container, which the pad's right stick scrolls.
func (p *Page) Scroll() *ui.ScrollState { return &p.scroll }

// loaded is what the page shows: the settings and the devices the library knows.
type loaded struct {
	st   appdir.Settings
	devs []services.DeviceView
}

// reload reads the settings and the devices again.
func (p *Page) reload() {
	sync := p.u.Sync()
	kit.Async(func() (loaded, error) { return loaded{sync.GetSettings(), sync.Devices()}, nil }, func(l loaded, _ error) {
		p.load(l.st, l.devs)
	})
}

// AutosaveDelay is how long after the last keystroke a text setting is saved.
// Tests shorten it.
var AutosaveDelay = 600 * time.Millisecond

// words turns an error of the ViewModel into the user's language.
func words(err error) error {
	if errors.Is(err, ErrRetention) {
		return errors.New(i18n.L("backups to keep must be a number from 1 to 100"))
	}
	return err
}

func (p *Page) load(st appdir.Settings, devs []services.DeviceView) {
	u := p.u
	p.devs = devs
	p.vm = NewViewModel(Deps{
		Save: u.Sync().SaveSettings,
		Run:  func(work func() (appdir.Settings, error), done func(appdir.Settings, error)) { kit.Async(work, done) },
		Saved: func() {
			p.savedAt = time.Now()
			time.AfterFunc(2500*time.Millisecond, func() { kit.OnUI(func() {}) }) // draw again to hide the note
		},
		Refresh:      u.Store().Refresh,
		ThemeChanged: u.ApplyTheme,
		Error:        func(err error) { u.ToastErr(words(err)) },
	}, st)
	p.f = formState{
		name: st.DeviceName, id: st.DeviceID,
		backups: st.BackupRetention > 0, retention: strconv.Itoa(p.vm.InitialRetention()),
		theme: st.Theme, language: st.Language,
		share: st.SyncLibrary, covers: !st.CoversLocalOnly, key: st.SteamGridDBKey,
	}
	p.later = kit.Debounce(AutosaveDelay, p.commit)
	p.langs = kit.Choice{}
	p.langs.Add("", i18n.L("System"))
	for _, o := range i18n.Options() {
		p.langs.Add(o.Code, o.Name)
	}
	p.keyTest = kit.NewTestButton(i18n.L("Test"), i18n.L("Key works."), i18n.L("Testing…"), func() func() error {
		k := p.f.key
		return func() error { return u.Lib().TestSteamGridKey(k) }
	})
	if p.opts.SteamArt != nil {
		p.steam = newSelfSteamCard(u, p.opts.SteamArt)
	}
	p.seen = p.read()
}

// read is the form as the fields show it now.
func (p *Page) read() Form {
	return Form{
		DeviceName: p.f.name, BackupsOn: p.f.backups, Retention: p.f.retention,
		ShareLibrary: p.f.share, UploadCovers: p.f.covers,
		Language: p.f.language, Theme: p.f.theme, ThemeChosen: true,
	}
}

func (p *Page) commit() { p.vm.Commit(p.read) }

// watch saves a change made since the last frame: a name or a count after a
// pause (they change with every key), any other field at once.
func (p *Page) watch() {
	cur := p.read()
	if cur == p.seen {
		return
	}
	quiet := func(f Form) Form { f.DeviceName, f.Retention = "", ""; return f }
	if quiet(cur) != quiet(p.seen) {
		p.seen = cur
		p.commit()
		return
	}
	p.seen = cur
	p.later()
}

// View draws the page.
func (p *Page) View(c *ui.Context) {
	nav := p.u.Navigator()
	kit.Page(c, kit.PageForm, &p.scroll, func() {
		if p.vm == nil {
			kit.PageHead(c, nav, nil, "", i18n.L("Settings"), nil)
			kit.Muted(c, i18n.L("Loading…"))
			return
		}
		p.watch()
		p.keyDirty = p.vm.KeyDirty(p.f.key)
		p.keyTest.SetAllowed(strings.TrimSpace(p.f.key) != "")
		p.form(c)
	})
}

func (p *Page) field(c *ui.Context, key string, v *string, placeholder string) ui.Element {
	return kit.Field(c, p.u.Navigator(), [2]any{"set", key}, v, placeholder)
}

// check is a setting row with a switch the pad toggles with A.
func (p *Page) check(c *ui.Context, key string, v *bool, label, desc string, disabled bool) {
	kit.ToggleRow(c, p.u.Navigator(), key, label, desc, v, disabled)
}

func (p *Page) form(c *ui.Context) {
	nav, f := p.u.Navigator(), &p.f
	kit.PageHead(c, nav, nil, "", i18n.L("Settings"), func() {
		if time.Since(p.savedAt) < 2500*time.Millisecond && !p.savedAt.IsZero() {
			kit.IconLine(c, kit.IconCheck, i18n.L("Saved"), c.Theme().Success)
		}
	})

	kit.Card(c, i18n.L("Appearance"), func() {
		kit.Labeled(c, i18n.L("Theme"), func() {
			kit.Chips(c, nav, "theme", []kit.ChipOption{
				{Value: kit.ThemeSystem, Label: i18n.L("System")},
				{Value: kit.ThemeDark, Label: i18n.L("Dark")},
				{Value: kit.ThemeLight, Label: i18n.L("Light")},
			}, f.theme, func(v string) { f.theme = v })
		})
		kit.Labeled(c, i18n.L("Language"), func() {
			kit.SelectOf(c, nav, "language", i18n.L("Language"), &p.langs, f.language, func(v string) { f.language = v })
		})
		if f.language != i18n.Started() {
			kit.Muted(c, i18n.L("Restart Carryover to apply the language."))
		}
	})

	kit.Card(c, i18n.L("This device"), func() {
		kit.Labeled(c, i18n.L("Device name (shown on other devices and in backups)"), func() { p.field(c, "name", &f.name, "") })
		kit.Labeled(c, i18n.L("Device ID"), func() { kit.Mono(c, f.id) })
	})

	p.devicesCard(c)

	kit.Card(c, i18n.L("Backups"), func() {
		// Cloud backups: a switch turns them on, then a count of 1-100. Off is
		// stored as 0; turning them back on starts from the default count.
		p.check(c, "backups", &f.backups, i18n.L("Back up saves to the cloud before they're overwritten"),
			i18n.L("Keeps older versions of your saves in the cloud, so a sync that went wrong can be undone."), false)
		if f.backups {
			kit.Labeled(c, i18n.L("Backups to keep per game (1–100)"), func() {
				p.field(c, "retention", &f.retention, "").Width(120)
				if _, err := ParseRetention(f.retention); err != nil {
					kit.Danger(c, words(err).Error())
				}
			})
		}
		components.Help(c, p.u, i18n.L("Backups make syncing a little slower."),
			i18n.L("Each backup is a zip uploaded before a sync replaces saves, so syncing gets slower, especially after playing. Older backups are deleted once there are more than this; pinned backups are always kept."),
			i18n.L("When off, only the current save is in the cloud. Before a download replaces this device's saves, a copy is still kept on this device (the last {{.Count}}).",
				map[string]any{"Count": syncer.SafetyKeep}))
	})

	kit.Card(c, i18n.L("Library sharing"), func() {
		p.check(c, "share", &f.share, i18n.L("Share the game library between devices through the cloud"),
			i18n.L("When enabled, games added on one device appear on the others; each device only needs to set its own paths."), false)
		ui.Divider(c)
		// Cover art is part of the library sync, so it can only be on while that is.
		p.check(c, "covers", &f.covers, i18n.L("Upload cover art and artwork to the cloud"),
			i18n.L("Covers and artwork always stay on this device. When this is on they are also uploaded to the cloud, so every device shows the same images; when off, this device neither uploads nor downloads them."), !f.share)
	})

	kit.Card(c, i18n.L("Cover art"), func() {
		kit.Labeled(c, i18n.L("SteamGridDB API key (optional)"), func() {
			ui.Row(c).Gap(kit.S2).Children(func() {
				p.field(c, "key", &f.key, i18n.L("Paste your key")).Password().Grow(1)
				kit.Btn(c, nav, "key-save", kit.BtnPrimary, nil, i18n.L("Save"), func() { p.vm.SaveKey(f.key, p.read) }, kit.BtnDisabled(!p.keyDirty))
				p.keyTest.Button(c, nav, "grid")
			})
		})
		p.keyTest.Result(c)
		kit.Muted(c, flows.GridKeyHelp())
	})

	if p.steam != nil {
		p.steam.view(c)
	}
	p.about(c)
}

// about shows which build is running, with links to its release notes and the
// source code, and the folder of the log to attach to bug reports.
func (p *Page) about(c *ui.Context) {
	u, nav := p.u, p.u.Navigator()
	info := version.Get()
	title := i18n.L("Development build")
	notes := i18n.L("All releases")
	if info.IsRelease() {
		title = i18n.L("Version {{.Version}}", map[string]any{"Version": info.Version})
		notes = i18n.L("Release notes")
	}
	kit.Card(c, i18n.L("About Carryover"), func() {
		ui.Text(c, title).FontWeight(600)
		if info.Commit != "" {
			commit := i18n.L("Commit {{.Commit}}", map[string]any{"Commit": info.Commit})
			if info.Modified {
				commit = i18n.L("Commit {{.Commit}}, with uncommitted changes", map[string]any{"Commit": info.Commit})
			}
			kit.Muted(c, commit)
		}
		ui.Row(c).Gap(kit.S4).Children(func() {
			p.link(c, "notes", notes, info.ReleaseURL())
			p.link(c, "source", i18n.L("Source code"), version.RepoURL)
		})
		logs := u.Sync().LogsDir()
		kit.Btn(c, nav, "open-logs", kit.BtnSecondary, kit.IconFolderOpen, i18n.L("Open logs folder"), func() { u.OpenFolder(logs) }).AlignSelf(ui.Start)
		kit.Muted(c, i18n.L("When reporting a problem, attach carryover.log from this folder (and carryover.1.log if it happened a while ago)."))
	})
}

// link is a link the pad can press: it opens the address in the browser.
func (p *Page) link(c *ui.Context, key, label, url string) {
	nav := p.u.Navigator()
	k := [2]any{"link", key}
	open := func() { p.u.ToastErr(p.u.OpenURL(url)) }
	nav.Item(nav.Handle(k), ui.Link(c.Key(k), label, url), open)
}
