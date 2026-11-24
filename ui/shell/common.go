package shell

import (
	"log"
	"strings"
	"sync/atomic"

	"github.com/akfaiz/carryover/internal/app/gamemode"
)

// setWindowIcon gives the window the application icon in its title bar and
// taskbar button (Linux, Windows). The taskbar can find the icon through the
// desktop file, but a title bar can't: an AppImage installs no icon theme, so
// without this its top-left corner stays empty.
func (u *Shell) setWindowIcon() {
	if len(u.cfg.Icon) == 0 {
		return
	}
	if err := u.win.SetIcon(u.cfg.Icon); err != nil {
		log.Printf("window icon: %v", err)
	}
}

// sections are the sidebar's places, in order, for LB and RB.
var sections = []string{"/", "/emulators", "/cloud", "/trash", "/settings"}

// sectionOf is the sidebar place a route belongs to.
func sectionOf(route string) string {
	s, _, _ := strings.Cut(strings.TrimPrefix(route, "/"), "/")
	switch s {
	case "", "game", "edit", "new":
		return "/"
	}
	return "/" + s
}

// padWanted reports whether Carryover reads the gamepad itself: in Game Mode
// or when Steam started it, where Steam hands the controller over as a
// gamepad. Started by hand on a desktop, Steam's Desktop Layout already turns
// the controller into a mouse and keys (stick: mouse, D-pad: arrows, A:
// Enter); reading it as well would do every press twice.
func padWanted() bool { return gamemode.Active() || gamemode.SteamLaunched() }

// Full screen is remembered apart for Carryover started by Steam (Big
// Picture, a controller across the room) and started by hand (a window on
// the desktop).
const (
	// PrefFullScreen and PrefFullScreenSteam are the preference keys of the
	// full screen choice.
	PrefFullScreen      = "window.fullscreen"
	PrefFullScreenSteam = "window.fullscreen.steam"
)

func fullScreenPref() string {
	if gamemode.SteamLaunched() {
		return PrefFullScreenSteam
	}
	return PrefFullScreen
}

// openKeyboard asks Steam for its keyboard; tests replace it.
var openKeyboard = gamemode.OpenKeyboard

// keyboardLogged is set once a successful request was logged.
var keyboardLogged atomic.Bool
