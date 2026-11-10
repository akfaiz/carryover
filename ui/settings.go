package ui

import (
	"strconv"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/widget"

	"github.com/akfaiz/playrelay/internal/appdir"
)

type settingsPage struct {
	u *UI
}

func newSettingsPage(u *UI) *settingsPage { return &settingsPage{u: u} }

func (p *settingsPage) view() fyne.CanvasObject {
	u := p.u
	scroll := scrollPage(muted("Loading…"))
	async(func() (appdir.Settings, error) { return u.sync.GetSettings(), nil }, func(st appdir.Settings, _ error) {
		replaceScroll(scroll, p.form(st))
	})
	return scroll
}

func (p *settingsPage) form(st appdir.Settings) fyne.CanvasObject {
	u := p.u
	name := widget.NewEntry()
	name.SetText(st.DeviceName)
	id := widget.NewEntry()
	id.TextStyle.Monospace = true
	id.SetText(st.DeviceID)
	id.Disable()

	retention := widget.NewEntry()
	retention.SetText(strconv.Itoa(st.BackupRetention))
	retention.Validator = func(s string) error {
		_, err := parseRetention(s)
		return err
	}
	share := widget.NewCheck("Share the game library between devices through the cloud", nil)
	share.SetChecked(st.SyncLibrary)

	key := widget.NewPasswordEntry()
	key.SetPlaceHolder("Paste your key")
	key.SetText(st.SteamGridDBKey)
	keyResult := widget.NewLabel("")
	keyResult.Wrapping = fyne.TextWrapWord
	keyResult.Hide()
	var testBtn *widget.Button
	testBtn = widget.NewButton("Test", func() {
		testBtn.Disable()
		testBtn.SetText("Testing…")
		keyResult.Hide()
		k := key.Text
		async0(func() error { return u.lib.TestSteamGridKey(k) }, func(err error) {
			testBtn.Enable()
			testBtn.SetText("Test")
			if err != nil {
				keyResult.Importance = widget.DangerImportance
				keyResult.SetText(err.Error())
			} else {
				keyResult.Importance = widget.SuccessImportance
				keyResult.SetText("Key works.")
			}
			keyResult.Show()
		})
	})
	setEnabled(testBtn, st.SteamGridDBKey != "")
	key.OnChanged = func(s string) { setEnabled(testBtn, strings.TrimSpace(s) != "") }

	save := primary("Save", func() {
		n, err := parseRetention(retention.Text)
		if err != nil {
			u.toastErr(err)
			return
		}
		next := st
		next.DeviceName = name.Text
		next.BackupRetention = n
		next.SyncLibrary = share.Checked
		next.SteamGridDBKey = strings.TrimSpace(key.Text)
		async(func() (appdir.Settings, error) { return u.sync.SaveSettings(next) }, func(saved appdir.Settings, err error) {
			if err != nil {
				u.toastErr(err)
				return
			}
			st = saved
			u.refresh()
			u.toast("Settings saved", toastOK)
		})
	})

	cli := mono("playrelay run <game> [--headless] [-- command…]\nplayrelay sync <game>|--all [--push|--pull]\nplayrelay list")
	narrow := func(o fyne.CanvasObject) fyne.CanvasObject { return maxWidth(640, o) }
	return vbox(
		pageHead(nil, "Settings"),
		narrow(card("This device",
			field("Device name (shown on other devices and in backups)", name),
			field("Device ID", id))),
		narrow(card("Sync",
			field("Backups to keep per game", fixedWidth(120, retention)),
			share,
			muted("When enabled, games added on one device appear on the others; each device only needs to set its own paths."))),
		narrow(card("Cover art",
			field("SteamGridDB API key (optional)", rowFill(key, testBtn)),
			keyResult,
			muted("Lets you search community covers for any game, including ones not on Steam, and fills in covers Steam doesn't have. "+
				"Get a free key at steamgriddb.com/profile/preferences/api (log in with Steam). The key is stored only on this device."))),
		hbox(save),
		narrow(card("Command line",
			muted("Useful for scripts or other launchers (Lutris, Heroic, Playnite…):"),
			cli)),
	)
}

func parseRetention(s string) (int, error) {
	n, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil || n < 0 || n > 100 {
		return 0, errRetention
	}
	return n, nil
}

type settingsError string

func (e settingsError) Error() string { return string(e) }

const errRetention = settingsError("backups to keep must be a number from 0 to 100")
