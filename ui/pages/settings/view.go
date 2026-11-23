package settings

import (
	"errors"
	"strconv"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/akfaiz/carryover/internal/app/appdir"
	"github.com/akfaiz/carryover/internal/app/version"
	"github.com/akfaiz/carryover/internal/cloudsync/syncer"
	"github.com/akfaiz/carryover/services"
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

// Page is the Settings page.
type Page struct {
	u      env.Env
	opts   Options
	vm     *ViewModel // of the form shown; set once the settings are loaded
	scroll *container.Scroll
}

// New returns the Settings page.
func New(u env.Env, opts Options) *Page { return &Page{u: u, opts: opts} }

func (p *Page) View() fyne.CanvasObject {
	p.scroll = kit.ScrollPage(kit.Muted(i18n.L("Loading…")))
	p.reload()
	return p.scroll
}

// loaded is what the page shows: the settings and the devices the library knows.
type loaded struct {
	st   appdir.Settings
	devs []services.DeviceView
}

// reload reads the settings and the devices again and draws the page anew.
func (p *Page) reload() {
	sync := p.u.Sync()
	kit.Async(func() (loaded, error) { return loaded{sync.GetSettings(), sync.Devices()}, nil }, func(l loaded, _ error) {
		kit.ReplaceScroll(p.scroll, p.form(l.st, l.devs))
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

func (p *Page) form(st appdir.Settings, devs []services.DeviceView) fyne.CanvasObject {
	u := p.u
	// A quiet note next to the title says a change was saved; a toast for
	// every change would pile up while typing.
	savedNote := kit.Small("")
	var hideNote *time.Timer
	var keyDirty func()
	p.vm = NewViewModel(Deps{
		Save: u.Sync().SaveSettings,
		Run:  func(work func() (appdir.Settings, error), done func(appdir.Settings, error)) { kit.Async(work, done) },
		Saved: func() {
			keyDirty()
			savedNote.SetText("✓ " + i18n.L("Saved"))
			if hideNote != nil {
				hideNote.Stop()
			}
			hideNote = time.AfterFunc(2500*time.Millisecond, func() { kit.OnUI(func() { savedNote.SetText("") }) })
		},
		Refresh:      u.Store().Refresh,
		ThemeChanged: u.ApplyTheme,
		Error:        func(err error) { u.ToastErr(words(err)) },
	}, st)
	vm := p.vm

	var read func() Form
	commit := func() { vm.Commit(read) }
	later := kit.Debounce(AutosaveDelay, commit)

	name := widget.NewEntry()
	name.SetText(st.DeviceName)
	name.OnChanged = func(string) { later() }
	name.OnSubmitted = func(string) { commit() }
	id := kit.MonoEntry("", st.DeviceID, nil)
	id.Disable()

	// Cloud backups: a checkbox turns them on, then a count of 1-100. Off is
	// stored as 0; turning them back on starts from the default count.
	retention := widget.NewEntry()
	retention.SetText(strconv.Itoa(vm.InitialRetention()))
	retention.Validator = func(s string) error {
		_, err := ParseRetention(s)
		return words(err)
	}
	retention.OnChanged = func(string) { later() }
	retention.OnSubmitted = func(string) { commit() }
	backups := kit.NewCheck(i18n.L("Back up saves to the cloud before they're overwritten"), st.BackupRetention > 0, func(on bool) {
		kit.SetEnabled(retention, on)
		commit()
	})
	kit.SetEnabled(retention, backups.Checked)
	themes := []struct{ setting, label string }{
		{kit.ThemeSystem, i18n.L("System")}, {kit.ThemeDark, i18n.L("Dark")}, {kit.ThemeLight, i18n.L("Light")},
	}
	labels := make([]string, len(themes))
	for i, t := range themes {
		labels[i] = t.label
	}
	themeSel := widget.NewRadioGroup(labels, nil)
	themeSel.Horizontal = true
	themeSel.Required = true
	for _, t := range themes {
		if t.setting == st.Theme {
			themeSel.SetSelected(t.label)
		}
	}
	themeSel.OnChanged = func(string) { commit() }

	// The language applies at the next start (Fyne picks its translations once).
	langs := &kit.Choice{}
	langs.Add("", i18n.L("System"))
	for _, o := range i18n.Options() {
		langs.Add(o.Code, o.Name)
	}
	langNote := kit.Muted(i18n.L("Restart Carryover to apply the language."))
	showLangNote := func(code string) { kit.SetVisible(langNote, code != i18n.Started()) }
	showLangNote(st.Language)
	langSel := kit.SelectOf(langs, st.Language, func(v string) {
		showLangNote(v)
		commit()
	})

	// Cover art is part of the library sync, so it can only be on while that is.
	covers := kit.NewCheck(i18n.L("Upload cover art and artwork to the cloud"), !st.CoversLocalOnly, func(bool) { commit() })
	share := kit.NewCheck(i18n.L("Share the game library between devices through the cloud"), st.SyncLibrary, func(on bool) {
		kit.SetEnabled(covers, on)
		commit()
	})
	kit.SetEnabled(covers, share.Checked)

	key := widget.NewPasswordEntry()
	key.SetPlaceHolder(i18n.L("Paste your key"))
	key.SetText(st.SteamGridDBKey)
	keyTest := kit.NewTestButton(i18n.L("Test"), i18n.L("Key works."), i18n.L("Testing…"), func() func() error {
		k := key.Text
		return func() error { return u.Lib().TestSteamGridKey(k) }
	})
	keyTest.SetAllowed(st.SteamGridDBKey != "")
	keySave := widget.NewButton(i18n.L("Save"), func() { vm.SaveKey(key.Text, read) })
	keySave.Importance = widget.HighImportance
	keyDirty = func() { kit.SetEnabled(keySave, vm.KeyDirty(key.Text)) }
	keyDirty()
	key.OnChanged = func(s string) {
		keyTest.SetAllowed(strings.TrimSpace(s) != "")
		keyDirty()
	}

	read = func() Form {
		f := Form{
			DeviceName: name.Text, BackupsOn: backups.Checked, Retention: retention.Text,
			ShareLibrary: share.Checked, UploadCovers: covers.Checked,
			Language: langs.Value(langSel.Selected),
		}
		for _, t := range themes {
			if t.label == themeSel.Selected {
				f.Theme, f.ThemeChosen = t.setting, true
			}
		}
		return f
	}

	narrow := func(o fyne.CanvasObject) fyne.CanvasObject { return kit.MaxWidth(640, o) }
	objs := []fyne.CanvasObject{
		kit.PageHead(nil, i18n.L("Settings"), container.NewCenter(savedNote)),
		narrow(kit.Card(i18n.L("Appearance"),
			kit.Field(i18n.L("Theme"), themeSel),
			kit.Field(i18n.L("Language"), kit.Outlined(langSel)),
			langNote)),
		narrow(kit.Card(i18n.L("This device"),
			kit.Field(i18n.L("Device name (shown on other devices and in backups)"), name),
			kit.Field(i18n.L("Device ID"), id))),
		narrow(p.devicesCard(devs)),
		narrow(kit.Card(i18n.L("Sync"),
			backups,
			kit.Field(i18n.L("Backups to keep per game (1–100)"), kit.FixedWidth(120, retention)),
			kit.Muted(i18n.L("Each backup is a zip uploaded before a sync replaces saves, so syncing gets slower, especially after playing. Older backups are deleted once there are more than this; pinned backups are always kept.")),
			kit.Muted(i18n.L("When off, only the current save is in the cloud. Before a download replaces this device's saves, a copy is still kept on this device (the last {{.Count}}).",
				map[string]any{"Count": syncer.SafetyKeep})),
			share,
			kit.Muted(i18n.L("When enabled, games added on one device appear on the others; each device only needs to set its own paths.")),
			covers,
			kit.Muted(i18n.L("Covers and artwork always stay on this device. When this is on they are also uploaded to the cloud, so every device shows the same images; when off, this device neither uploads nor downloads them.")))),
		narrow(kit.Card(i18n.L("Cover art"),
			kit.Field(i18n.L("SteamGridDB API key (optional)"), kit.RowFill(key, keySave, keyTest.Button)),
			keyTest.Result,
			kit.Muted(flows.GridKeyHelp()))),
	}
	if p.opts.SteamArt != nil {
		objs = append(objs, narrow(newSelfSteamCard(u, p.opts.SteamArt)))
	}
	objs = append(objs, narrow(aboutCard(u)))
	return kit.Stack(objs...)
}

// aboutCard shows which build is running, with links to its release notes
// and the source code, and the folder of the log to attach to bug reports.
func aboutCard(u env.Env) fyne.CanvasObject {
	info := version.Get()
	title := i18n.L("Development build")
	notes := i18n.L("All releases")
	if info.IsRelease() {
		title = i18n.L("Version {{.Version}}", map[string]any{"Version": info.Version})
		notes = i18n.L("Release notes")
	}
	objs := []fyne.CanvasObject{widget.NewLabelWithStyle(title, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})}
	if info.Commit != "" {
		commit := i18n.L("Commit {{.Commit}}", map[string]any{"Commit": info.Commit})
		if info.Modified {
			commit = i18n.L("Commit {{.Commit}}, with uncommitted changes", map[string]any{"Commit": info.Commit})
		}
		objs = append(objs, kit.Muted(commit))
	}
	objs = append(objs, kit.HBox(
		widget.NewHyperlink(notes, kit.MustURL(info.ReleaseURL())),
		widget.NewHyperlink(i18n.L("Source code"), kit.MustURL(version.RepoURL)),
	))
	logs := u.Sync().LogsDir()
	objs = append(objs,
		kit.HBox(widget.NewButtonWithIcon(i18n.L("Open logs folder"), theme.FolderOpenIcon(), func() { u.OpenFolder(logs) })),
		kit.Muted(i18n.L("When reporting a problem, attach carryover.log from this folder (and carryover.1.log if it happened a while ago).")))
	return kit.Card(i18n.L("About Carryover"), objs...)
}
