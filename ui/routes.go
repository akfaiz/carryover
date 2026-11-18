package ui

import (
	"strings"

	"github.com/akfaiz/carryover/ui/env"
)

// pageFor builds the page of a route. Anything unknown is the library.
//
//	/                      library
//	/game/<id>             a game
//	/edit/<id>             edit a game
//	/new, /new/emulator    add a game; /new/steam/<app id> from Steam
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
		return newGameDetail(u, arg(1))
	case arg(0) == "edit" && arg(1) != "":
		return newEditor(u, arg(1), "")
	case arg(0) == "new" && arg(1) == "steam" && arg(2) != "":
		return newEditor(u, "", arg(2))
	case arg(0) == "new" && arg(1) == "emulator":
		e := newEditor(u, "", "")
		e.kind = kindEmulator
		return e
	case arg(0) == "new":
		return newEditor(u, "", "")
	case arg(0) == "emulators":
		return newEmulatorsPage(u)
	case arg(0) == "cloud":
		return newCloudPage(u)
	case arg(0) == "trash":
		return newTrashPage(u)
	case arg(0) == "settings":
		return newSettingsPage(u)
	}
	return newLibraryPage(u)
}
