// Package flows holds the multi-step actions that open dialogs, call services
// and navigate: add a game, import from Steam, set the SteamGridDB key, open a
// game in Steam. This is the MyGo build.
package flows

import (
	"github.com/akfaiz/carryover/services"
	"github.com/akfaiz/carryover/ui/env"
	"github.com/akfaiz/carryover/ui/i18n"
	"github.com/akfaiz/carryover/ui/kit"
)

// OpenSteam asks Steam to start a game that Steam launches on this device.
func OpenSteam(u env.Env, g services.GameView) {
	link := services.SteamRunURL(g)
	if link == "" {
		u.Toast(i18n.L("Steam can't start this game yet: add it to Steam on the game's page."), kit.ToastErr)
		return
	}
	if err := u.OpenURL(link); err != nil {
		u.ToastErr(err)
		return
	}
	u.Toast(i18n.L("Starting {{.Game}} in Steam…", map[string]any{"Game": g.Name}), kit.ToastInfo)
}
