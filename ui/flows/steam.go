package flows

import (
	"net/url"

	"fyne.io/fyne/v2/lang"

	"github.com/akfaiz/carryover/services"
	"github.com/akfaiz/carryover/ui/env"
	"github.com/akfaiz/carryover/ui/kit"
)

// OpenSteam asks Steam to start a game that Steam launches on this device.
func OpenSteam(u env.Env, g services.GameView) {
	link := services.SteamRunURL(g)
	if link == "" {
		u.Toast(lang.L("Steam can't start this game yet: add it to Steam on the game's page."), kit.ToastErr)
		return
	}
	pu, err := url.Parse(link)
	if err == nil {
		err = u.App().OpenURL(pu)
	}
	if err != nil {
		u.ToastErr(err)
		return
	}
	u.Toast(lang.L("Starting {{.Game}} in Steam…", map[string]any{"Game": g.Name}), kit.ToastInfo)
}
