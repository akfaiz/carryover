package ui

import (
	"errors"
	"strconv"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/lang"
	"fyne.io/fyne/v2/widget"

	"github.com/akfaiz/carryover/internal/app/appdir"
	"github.com/akfaiz/carryover/internal/app/version"
	"github.com/akfaiz/carryover/internal/cloudsync/syncer"
)

type settingsPage struct {
	u *UI
}

func newSettingsPage(u *UI) *settingsPage { return &settingsPage{u: u} }

func (p *settingsPage) view() fyne.CanvasObject {
	u := p.u
	scroll := scrollPage(muted(lang.L("Loading…")))
	async(func() (appdir.Settings, error) { return u.sync.GetSettings(), nil }, func(st appdir.Settings, _ error) {
		replaceScroll(scroll, p.form(st))
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
	later := debounce(autosaveDelay, changed)

	name := widget.NewEntry()
	name.SetText(st.DeviceName)
	name.OnChanged = func(string) { later() }
	name.OnSubmitted = func(string) { commit() }
	id := monoEntry("", st.DeviceID, nil)
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
	backups := newCheck(lang.L("Back up saves to the cloud before they're overwritten"), st.BackupRetention > 0, func(on bool) {
		setEnabled(retention, on)
		commit()
	})
	setEnabled(retention, backups.Checked)
	themes := []struct{ setting, label string }{
		{themeSystem, lang.L("System")}, {themeDark, lang.L("Dark")}, {themeLight, lang.L("Light")},
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

	// Cover art is part of the library sync, so it can only be on while that is.
	covers := newCheck(lang.L("Upload cover art and artwork to the cloud"), !st.CoversLocalOnly, func(bool) { commit() })
	share := newCheck(lang.L("Share the game library between devices through the cloud"), st.SyncLibrary, func(on bool) {
		setEnabled(covers, on)
		commit()
	})
	setEnabled(covers, share.Checked)

	key := widget.NewPasswordEntry()
	key.SetPlaceHolder(lang.L("Paste your key"))
	key.SetText(st.SteamGridDBKey)
	keyTest := newTestButton(lang.L("Test"), lang.L("Key works."), func() func() error {
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
	keyDirty := func() { setEnabled(keySave, strings.TrimSpace(key.Text) != st.SteamGridDBKey) }
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
		next.SteamGridDBKey = keyToSave
		return next, true, nil
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
		async(func() (appdir.Settings, error) { return u.sync.SaveSettings(next) }, func(saved appdir.Settings, err error) {
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
			u.toast(lang.L("Settings saved"), toastOK)
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

	narrow := func(o fyne.CanvasObject) fyne.CanvasObject { return maxWidth(640, o) }
	return stack(
		pageHead(nil, lang.L("Settings")),
		narrow(card(lang.L("Appearance"),
			field(lang.L("Theme"), themeSel))),
		narrow(card(lang.L("This device"),
			field(lang.L("Device name (shown on other devices and in backups)"), name),
			field(lang.L("Device ID"), id))),
		narrow(card(lang.L("Sync"),
			backups,
			field(lang.L("Backups to keep per game (1–100)"), fixedWidth(120, retention)),
			muted(lang.L("Each backup is a zip uploaded before a sync replaces saves, so syncing gets slower, especially after playing. Older backups are deleted once there are more than this; pinned backups are always kept.")),
			muted(lang.L("When off, only the current save is in the cloud. Before a download replaces this device's saves, a copy is still kept on this device (the last {{.Count}}).",
				map[string]any{"Count": syncer.SafetyKeep})),
			share,
			muted(lang.L("When enabled, games added on one device appear on the others; each device only needs to set its own paths.")),
			covers,
			muted(lang.L("Covers and artwork always stay on this device. When this is on they are also uploaded to the cloud, so every device shows the same images; when off, this device neither uploads nor downloads them.")))),
		narrow(card(lang.L("Cover art"),
			field(lang.L("SteamGridDB API key (optional)"), rowFill(key, keySave, keyTest.Button)),
			keyTest.Result,
			muted(lang.L("Lets you search community covers for any game, including ones not on Steam, and fills in covers Steam doesn't have. Get a free key at steamgriddb.com/profile/preferences/api (log in with Steam). The key is stored only on this device.")))),
		narrow(aboutCard()),
	)
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
		objs = append(objs, muted(commit))
	}
	objs = append(objs, hbox(
		widget.NewHyperlink(notes, mustURL(info.ReleaseURL())),
		widget.NewHyperlink(lang.L("Source code"), mustURL(version.RepoURL)),
	))
	return card(lang.L("About Carryover"), objs...)
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
