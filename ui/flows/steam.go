package flows

import (
	"net/url"

	"github.com/akfaiz/carryover/ui/i18n"

	"github.com/akfaiz/carryover/services"
	"github.com/akfaiz/carryover/ui/env"
	"github.com/akfaiz/carryover/ui/kit"
)

// OpenSteam asks Steam to start a game that Steam launches on this device.
func OpenSteam(u env.Env, g services.GameView) {
	link := services.SteamRunURL(g)
	if link == "" {
		u.Toast(i18n.L("Steam can't start this game yet: add it to Steam on the game's page."), kit.ToastErr)
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
	u.Toast(i18n.L("Starting {{.Game}} in Steam…", map[string]any{"Game": g.Name}), kit.ToastInfo)
}
