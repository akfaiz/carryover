package flows

import (
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/akfaiz/carryover/internal/app/appdir"
	"github.com/akfaiz/carryover/ui/env"
	"github.com/akfaiz/carryover/ui/i18n"
	"github.com/akfaiz/carryover/ui/kit"
)

// ShowGridKeyDialog asks for the SteamGridDB key in a dialog, for pages that
// shouldn't be left to go to Settings (the game editor). onSaved runs once it
// is stored.
func ShowGridKeyDialog(u env.Env, onSaved func()) {
	var key string
	saving := false
	test := kit.NewTestButton(i18n.L("Test"), i18n.L("Key works."), i18n.L("Testing…"), func() func() error {
		k := strings.TrimSpace(key)
		return func() error { return u.Lib().TestSteamGridKey(k) }
	})
	var d *kit.Dialog
	save := func() {
		k := strings.TrimSpace(key)
		saving = true
		kit.Async(func() (appdir.Settings, error) {
			st := u.Sync().GetSettings()
			st.SteamGridDBKey = k
			return u.Sync().SaveSettings(st)
		}, func(_ appdir.Settings, err error) {
			saving = false
			if err != nil {
				u.ToastErr(err)
				return
			}
			d.Close()
			u.Store().Refresh()
			u.Toast(i18n.L("SteamGridDB key saved"), kit.ToastOK)
			onSaved()
		})
	}
	d = u.Dialogs().Show(&kit.Dialog{Title: i18n.L("SteamGridDB API key"), Width: 540, Body: func(c *ui.Context, nav *kit.Navigator) {
		ok := strings.TrimSpace(key) != ""
		test.SetAllowed(ok)
		kit.Muted(c, GridKeyHelp())
		ui.Row(c).Gap(kit.S2).Children(func() {
			kit.Field(c, nav, "grid-key", &key, i18n.L("Paste your key")).Password().Grow(1)
			test.Button(c, nav, "gridkey")
		})
		test.Result(c)
		kit.Footer(c, func() {
			kit.Btn(c, nav, "grid-cancel", kit.BtnSecondary, nil, i18n.L("Cancel"), func() { d.Close() }, kit.BtnItem(kit.FocusSafe))
			kit.Btn(c, nav, "grid-save", kit.BtnPrimary, nil, i18n.L("Save"), save, kit.BtnDisabled(!ok || saving))
		})
	}})
}
