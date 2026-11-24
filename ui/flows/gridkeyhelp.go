package flows

import "github.com/akfaiz/carryover/ui/i18n"

// GridKeyHelp explains what the SteamGridDB API key is for and where to get one.
func GridKeyHelp() string {
	return i18n.L("Lets you search community covers and artwork for any game, including ones not on Steam. Get a free key at steamgriddb.com/profile/preferences/api (log in with Steam). The key is stored only on this device.")
}
