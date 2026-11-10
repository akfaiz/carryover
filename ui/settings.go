package ui

import (
	"errors"
	"strconv"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/lang"
	"fyne.io/fyne/v2/widget"

	"github.com/akfaiz/carryover/internal/appdir"
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

func (p *settingsPage) form(st appdir.Settings) fyne.CanvasObject {
	u := p.u
	name := widget.NewEntry()
	name.SetText(st.DeviceName)
	id := monoEntry("", st.DeviceID, nil)
	id.Disable()

	retention := widget.NewEntry()
	retention.SetText(strconv.Itoa(st.BackupRetention))
	retention.Validator = func(s string) error {
		_, err := parseRetention(s)
		return err
	}
	themeSel := widget.NewRadioGroup([]string{lang.L("Dark"), lang.L("Light")}, nil)
	themeSel.Horizontal = true
	themeSel.Required = true
	if st.Theme == themeLight {
		themeSel.SetSelected(lang.L("Light"))
	} else {
		themeSel.SetSelected(lang.L("Dark"))
	}
	share := widget.NewCheck(lang.L("Share the game library between devices through the cloud"), nil)
	share.SetChecked(st.SyncLibrary)

	key := widget.NewPasswordEntry()
	key.SetPlaceHolder(lang.L("Paste your key"))
	key.SetText(st.SteamGridDBKey)
	keyTest := newTestButton(lang.L("Test"), lang.L("Key works."), func() func() error {
		k := key.Text
		return func() error { return u.lib.TestSteamGridKey(k) }
	})
	keyTest.SetAllowed(st.SteamGridDBKey != "")
	key.OnChanged = func(s string) { keyTest.SetAllowed(strings.TrimSpace(s) != "") }

	save := primary(lang.L("Save"), func() {
		n, err := parseRetention(retention.Text)
		if err != nil {
			u.toastErr(err)
			return
		}
		next := st
		next.DeviceName = name.Text
		next.BackupRetention = n
		next.SyncLibrary = share.Checked
		next.Theme = themeDark
		if themeSel.Selected == lang.L("Light") {
			next.Theme = themeLight
		}
		next.SteamGridDBKey = strings.TrimSpace(key.Text)
		async(func() (appdir.Settings, error) { return u.sync.SaveSettings(next) }, func(saved appdir.Settings, err error) {
			if err != nil {
				u.toastErr(err)
				return
			}
			st = saved
			u.applyTheme(saved.Theme)
			u.refresh()
			u.toast(lang.L("Settings saved"), toastOK)
		})
	})

	cli := mono("carryover run <game> [--headless] [-- command…]\ncarryover sync <game>|--all [--push|--pull]\ncarryover list")
	narrow := func(o fyne.CanvasObject) fyne.CanvasObject { return maxWidth(640, o) }
	return stack(
		pageHead(nil, lang.L("Settings")),
		narrow(card(lang.L("Appearance"),
			field(lang.L("Theme"), themeSel))),
		narrow(card(lang.L("This device"),
			field(lang.L("Device name (shown on other devices and in backups)"), name),
			field(lang.L("Device ID"), id))),
		narrow(card(lang.L("Sync"),
			field(lang.L("Backups to keep per game (0 = off)"), fixedWidth(120, retention)),
			muted(lang.L("Off by default: only the current save is synced. Backups are uploaded as a zip before saves are overwritten, which makes syncing slower, especially after playing.")),
			share,
			muted(lang.L("When enabled, games added on one device appear on the others; each device only needs to set its own paths.")))),
		narrow(card(lang.L("Cover art"),
			field(lang.L("SteamGridDB API key (optional)"), rowFill(key, keyTest.Button)),
			keyTest.Result,
			muted(lang.L("Lets you search community covers for any game, including ones not on Steam, and fills in covers Steam doesn't have. Get a free key at steamgriddb.com/profile/preferences/api (log in with Steam). The key is stored only on this device.")))),
		hbox(save),
		narrow(card(lang.L("Command line"),
			muted(lang.L("Useful for scripts or other launchers (Lutris, Heroic, Playnite…):")),
			cli)),
	)
}

func parseRetention(s string) (int, error) {
	n, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil || n < 0 || n > 100 {
		return 0, errors.New(lang.L("backups to keep must be a number from 0 to 100"))
	}
	return n, nil
}
