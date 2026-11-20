package ui

import (
	"strings"

	"github.com/akfaiz/carryover/ui/env"
	"github.com/akfaiz/carryover/ui/pages/cloud"
	"github.com/akfaiz/carryover/ui/pages/editor"
	"github.com/akfaiz/carryover/ui/pages/emulators"
	"github.com/akfaiz/carryover/ui/pages/game"
	"github.com/akfaiz/carryover/ui/pages/library"
	"github.com/akfaiz/carryover/ui/pages/settings"
	"github.com/akfaiz/carryover/ui/pages/trash"
)

// pageFor builds the page of a route. Anything unknown is the library.
//
//	/                      library
//	/game/<id>             a game
//	/edit/<id>             edit a game
//	/new, /new/emulator    add a game; /new/steam/<app id> from Steam,
//	                       /new/lutris/<config id> from Lutris
//	/emulators /cloud /trash /settings
func pageFor(u env.Env, path string) env.Page {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	arg := func(i int) string {
		if i < len(parts) {
			return parts[i]
		}
		return ""
	}
	switch {
	case arg(0) == "game" && arg(1) != "":
		return game.New(u, arg(1))
	case arg(0) == "edit" && arg(1) != "":
		return editor.New(u, arg(1), "")
	case arg(0) == "new" && arg(1) == "steam" && arg(2) != "":
		return editor.New(u, "", arg(2))
	case arg(0) == "new" && arg(1) == "lutris" && arg(2) != "":
		return editor.NewFromLutris(u, arg(2))
	case arg(0) == "new" && arg(1) == "emulator":
		return editor.NewEmulated(u)
	case arg(0) == "new":
		return editor.New(u, "", "")
	case arg(0) == "emulators":
		return emulators.New(u)
	case arg(0) == "cloud":
		return cloud.New(u)
	case arg(0) == "trash":
		return trash.New(u)
	case arg(0) == "settings":
		return settings.New(u, settings.Options{SteamArt: selfSteamArt})
	}
	return library.New(u)
}
