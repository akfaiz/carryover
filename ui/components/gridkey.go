package components

import (
	"strings"

	"fyne.io/fyne/v2/lang"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"

	"github.com/akfaiz/carryover/internal/app/appdir"
	"github.com/akfaiz/carryover/ui/env"
	"github.com/akfaiz/carryover/ui/kit"
)

// GridKeyHelp explains what the SteamGridDB API key is for and where to get one.
func GridKeyHelp() string {
	return lang.L("Lets you search community covers and artwork for any game, including ones not on Steam. Get a free key at steamgriddb.com/profile/preferences/api (log in with Steam). The key is stored only on this device.")
}

// ShowGridKeyDialog asks for the SteamGridDB key in a dialog, for pages that
// shouldn't be left to go to Settings (the game editor). onSaved runs once it
// is stored.
func ShowGridKeyDialog(u env.Env, onSaved func()) {
	key := widget.NewPasswordEntry()
	key.SetPlaceHolder(lang.L("Paste your key"))
	keyTest := kit.NewTestButton(lang.L("Test"), lang.L("Key works."), lang.L("Testing…"), func() func() error {
		k := strings.TrimSpace(key.Text)
		return func() error { return u.Lib().TestSteamGridKey(k) }
	})
	keyTest.SetAllowed(false)
	var d DialogHider
	save := kit.Primary(lang.L("Save"), nil)
	save.Disable()
	save.OnTapped = func() {
		k := strings.TrimSpace(key.Text)
		save.Disable()
		kit.Async(func() (appdir.Settings, error) {
			st := u.Sync().GetSettings()
			st.SteamGridDBKey = k
			return u.Sync().SaveSettings(st)
		}, func(_ appdir.Settings, err error) {
			if err != nil {
				u.ToastErr(err)
				save.Enable()
				return
			}
			d.Hide()
			u.Store().Refresh()
			u.Toast(lang.L("SteamGridDB key saved"), kit.ToastOK)
			onSaved()
		})
	}
	key.OnChanged = func(s string) {
		ok := strings.TrimSpace(s) != ""
		keyTest.SetAllowed(ok)
		kit.SetEnabled(save, ok)
	}
	body := kit.VBox(kit.Muted(GridKeyHelp()), kit.RowFill(key, keyTest.Button), keyTest.Result,
		kit.HBox(layout.NewSpacer(), widget.NewButton(lang.L("Cancel"), func() { d.Hide() }), save))
	d = ShowCustom(u, lang.L("SteamGridDB API key"), body, 540)
}
