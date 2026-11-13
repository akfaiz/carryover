package ui

import (
	"errors"
	"strconv"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/lang"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"

	"github.com/akfaiz/carryover/internal/app/appdir"
	"github.com/akfaiz/carryover/internal/app/version"
	"github.com/akfaiz/carryover/internal/cloudsync/syncer"
	"github.com/akfaiz/carryover/internal/ui/kit"
)

type settingsPage struct {
	u *UI
}

func newSettingsPage(u *UI) *settingsPage { return &settingsPage{u: u} }

func (p *settingsPage) view() fyne.CanvasObject {
	u := p.u
	scroll := kit.ScrollPage(kit.Muted(lang.L("Loading…")))
	kit.Async(func() (appdir.Settings, error) { return u.sync.GetSettings(), nil }, func(st appdir.Settings, _ error) {
		kit.ReplaceScroll(scroll, p.form(st))
	})
	return scroll
}

// autosaveDelay is how long after the last keystroke a text setting is saved.
var autosaveDelay = 600 * time.Millisecond

// form is the settings page. Every setting saves as soon as it changes (text
// fields shortly after the last keystroke), except the SteamGridDB key, which
// has a Save button of its own: it is a secret, and half a pasted key is
// worthless.
func (p *settingsPage) form(st appdir.Settings) fyne.CanvasObject {
	u := p.u
	var commit func()
	changed := func() { commit() }
	later := kit.Debounce(autosaveDelay, changed)

	name := widget.NewEntry()
	name.SetText(st.DeviceName)
	name.OnChanged = func(string) { later() }
	name.OnSubmitted = func(string) { commit() }
	id := kit.MonoEntry("", st.DeviceID, nil)
	id.Disable()

	// Cloud backups: a checkbox turns them on, then a count of 1-100. Off is
	// stored as 0; turning them back on starts from the default count.
	keep := st.BackupRetention
	if keep <= 0 {
		keep = defaultRetention
	}
	retention := widget.NewEntry()
	retention.SetText(strconv.Itoa(keep))
	retention.Validator = func(s string) error {
		_, err := parseRetention(s)
		return err
	}
	retention.OnChanged = func(string) { later() }
	retention.OnSubmitted = func(string) { commit() }
	backups := kit.NewCheck(lang.L("Back up saves to the cloud before they're overwritten"), st.BackupRetention > 0, func(on bool) {
		kit.SetEnabled(retention, on)
		commit()
	})
	kit.SetEnabled(retention, backups.Checked)
	themes := []struct{ setting, label string }{
		{kit.ThemeSystem, lang.L("System")}, {kit.ThemeDark, lang.L("Dark")}, {kit.ThemeLight, lang.L("Light")},
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
	langs.Add("", lang.L("System"))
	for _, o := range languageOptions() {
		langs.Add(o.code, o.name)
	}
	langNote := kit.Muted(lang.L("Restart Carryover to apply the language."))
	showLangNote := func(code string) { kit.SetVisible(langNote, code != startLanguage) }
	showLangNote(st.Language)
	langSel := kit.SelectOf(langs, st.Language, func(v string) {
		showLangNote(v)
		commit()
	})

	// Cover art is part of the library sync, so it can only be on while that is.
	covers := kit.NewCheck(lang.L("Upload cover art and artwork to the cloud"), !st.CoversLocalOnly, func(bool) { commit() })
	share := kit.NewCheck(lang.L("Share the game library between devices through the cloud"), st.SyncLibrary, func(on bool) {
		kit.SetEnabled(covers, on)
		commit()
	})
	kit.SetEnabled(covers, share.Checked)

	key := widget.NewPasswordEntry()
	key.SetPlaceHolder(lang.L("Paste your key"))
	key.SetText(st.SteamGridDBKey)
	keyTest := kit.NewTestButton(lang.L("Test"), lang.L("Key works."), lang.L("Testing…"), func() func() error {
		k := key.Text
		return func() error { return u.lib.TestSteamGridKey(k) }
	})
	keyTest.SetAllowed(st.SteamGridDBKey != "")
	keyToSave := st.SteamGridDBKey
	keySave := widget.NewButton(lang.L("Save"), func() {
		keyToSave = strings.TrimSpace(key.Text)
		commit()
	})
	keySave.Importance = widget.HighImportance
	keyDirty := func() { kit.SetEnabled(keySave, strings.TrimSpace(key.Text) != st.SteamGridDBKey) }
	keyDirty()
	key.OnChanged = func(s string) {
		keyTest.SetAllowed(strings.TrimSpace(s) != "")
		keyDirty()
	}

	// collect returns the settings the form shows. The key is the saved one
	// until its own button is pressed. ok is false when a value can't be saved
	// yet; err says why when the user should hear about it.
	collect := func() (next appdir.Settings, ok bool, err error) {
		next = st
		n := 0
		if backups.Checked {
			if n, err = parseRetention(retention.Text); err != nil {
				return st, false, err
			}
		}
		if dev := strings.TrimSpace(name.Text); dev != "" {
			next.DeviceName = dev
		}
		next.BackupRetention = n
		next.SyncLibrary = share.Checked
		next.CoversLocalOnly = !covers.Checked
		for _, t := range themes {
			if t.label == themeSel.Selected {
				next.Theme = t.setting
			}
		}
		next.Language = langs.Value(langSel.Selected)
		next.SteamGridDBKey = keyToSave
		return next, true, nil
	}
	// A quiet note next to the title says a change was saved; a toast for
	// every change would pile up while typing.
	savedNote := kit.Small("")
	var hideNote *time.Timer
	showSaved := func() {
		savedNote.SetText("✓ " + lang.L("Saved"))
		if hideNote != nil {
			hideNote.Stop()
		}
		hideNote = time.AfterFunc(2500*time.Millisecond, func() { kit.OnUI(func() { savedNote.SetText("") }) })
	}

	// Saves are one at a time: a change made while one is running is saved
	// right after it, from whatever the form shows then.
	var saving, dirty bool
	commit = func() {
		if saving {
			dirty = true
			return
		}
		next, ok, err := collect()
		if err != nil {
			u.toastErr(err)
		}
		if !ok || next == st {
			return
		}
		saving = true
		kit.Async(func() (appdir.Settings, error) { return u.sync.SaveSettings(next) }, func(saved appdir.Settings, err error) {
			saving = false
			if err != nil {
				u.toastErr(err)
				return
			}
			themeChanged := saved.Theme != st.Theme
			st = saved
			keyToSave = saved.SteamGridDBKey
			keyDirty()
			u.refresh()
			showSaved()
			if themeChanged {
				u.applyTheme(saved.Theme)
				return // the page is built again
			}
			if dirty {
				dirty = false
				commit()
			}
		})
	}

	narrow := func(o fyne.CanvasObject) fyne.CanvasObject { return kit.MaxWidth(640, o) }
	return kit.Stack(
		kit.PageHead(nil, lang.L("Settings"), container.NewCenter(savedNote)),
		narrow(kit.Card(lang.L("Appearance"),
			kit.Field(lang.L("Theme"), themeSel),
			kit.Field(lang.L("Language"), langSel),
			langNote)),
		narrow(kit.Card(lang.L("This device"),
			kit.Field(lang.L("Device name (shown on other devices and in backups)"), name),
			kit.Field(lang.L("Device ID"), id))),
		narrow(kit.Card(lang.L("Sync"),
			backups,
			kit.Field(lang.L("Backups to keep per game (1–100)"), kit.FixedWidth(120, retention)),
			kit.Muted(lang.L("Each backup is a zip uploaded before a sync replaces saves, so syncing gets slower, especially after playing. Older backups are deleted once there are more than this; pinned backups are always kept.")),
			kit.Muted(lang.L("When off, only the current save is in the cloud. Before a download replaces this device's saves, a copy is still kept on this device (the last {{.Count}}).",
				map[string]any{"Count": syncer.SafetyKeep})),
			share,
			kit.Muted(lang.L("When enabled, games added on one device appear on the others; each device only needs to set its own paths.")),
			covers,
			kit.Muted(lang.L("Covers and artwork always stay on this device. When this is on they are also uploaded to the cloud, so every device shows the same images; when off, this device neither uploads nor downloads them.")))),
		narrow(kit.Card(lang.L("Cover art"),
			kit.Field(lang.L("SteamGridDB API key (optional)"), kit.RowFill(key, keySave, keyTest.Button)),
			keyTest.Result,
			kit.Muted(gridKeyHelp()))),
		narrow(aboutCard()),
	)
}

func gridKeyHelp() string {
	return lang.L("Lets you search community covers for any game, including ones not on Steam, and fills in covers Steam doesn't have. Get a free key at steamgriddb.com/profile/preferences/api (log in with Steam). The key is stored only on this device.")
}

// showGridKeyDialog asks for the SteamGridDB key in a dialog, for pages that
// shouldn't be left to go to Settings (the game editor). onSaved runs once it
// is stored.
func showGridKeyDialog(u *UI, onSaved func()) {
	key := widget.NewPasswordEntry()
	key.SetPlaceHolder(lang.L("Paste your key"))
	keyTest := kit.NewTestButton(lang.L("Test"), lang.L("Key works."), lang.L("Testing…"), func() func() error {
		k := strings.TrimSpace(key.Text)
		return func() error { return u.lib.TestSteamGridKey(k) }
	})
	keyTest.SetAllowed(false)
	var d dialogHider
	save := kit.Primary(lang.L("Save"), nil)
	save.Disable()
	save.OnTapped = func() {
		k := strings.TrimSpace(key.Text)
		save.Disable()
		kit.Async(func() (appdir.Settings, error) {
			st := u.sync.GetSettings()
			st.SteamGridDBKey = k
			return u.sync.SaveSettings(st)
		}, func(_ appdir.Settings, err error) {
			if err != nil {
				u.toastErr(err)
				save.Enable()
				return
			}
			d.Hide()
			u.refresh()
			u.toast(lang.L("SteamGridDB key saved"), kit.ToastOK)
			onSaved()
		})
	}
	key.OnChanged = func(s string) {
		ok := strings.TrimSpace(s) != ""
		keyTest.SetAllowed(ok)
		kit.SetEnabled(save, ok)
	}
	body := kit.VBox(kit.Muted(gridKeyHelp()), kit.RowFill(key, keyTest.Button), keyTest.Result,
		kit.HBox(layout.NewSpacer(), widget.NewButton(lang.L("Cancel"), func() { d.Hide() }), save))
	d = showCustom(u, lang.L("SteamGridDB API key"), body, 540)
}

// aboutCard shows which build is running, with links to its release notes
// and the source code.
func aboutCard() fyne.CanvasObject {
	info := version.Get()
	title := lang.L("Development build")
	notes := lang.L("All releases")
	if info.IsRelease() {
		title = lang.L("Version {{.Version}}", map[string]any{"Version": info.Version})
		notes = lang.L("Release notes")
	}
	objs := []fyne.CanvasObject{widget.NewLabelWithStyle(title, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})}
	if info.Commit != "" {
		commit := lang.L("Commit {{.Commit}}", map[string]any{"Commit": info.Commit})
		if info.Modified {
			commit = lang.L("Commit {{.Commit}}, with uncommitted changes", map[string]any{"Commit": info.Commit})
		}
		objs = append(objs, kit.Muted(commit))
	}
	objs = append(objs, kit.HBox(
		widget.NewHyperlink(notes, kit.MustURL(info.ReleaseURL())),
		widget.NewHyperlink(lang.L("Source code"), kit.MustURL(version.RepoURL)),
	))
	return kit.Card(lang.L("About Carryover"), objs...)
}

// defaultRetention is the backup count offered when cloud backups are turned on.
const defaultRetention = 5

// parseRetention parses the number of cloud backups to keep (1-100).
func parseRetention(s string) (int, error) {
	n, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil || n < 1 || n > 100 {
		return 0, errors.New(lang.L("backups to keep must be a number from 1 to 100"))
	}
	return n, nil
}
