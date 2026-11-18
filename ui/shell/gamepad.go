package shell

import (
	"log"
	"strings"
	"sync/atomic"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/lang"
	"fyne.io/fyne/v2/layout"

	"github.com/akfaiz/carryover/internal/app/gamemode"
	"github.com/akfaiz/carryover/ui/env"
	"github.com/akfaiz/carryover/ui/kit"
)

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

// newNavigator lets the main window be used with a gamepad (Steam Deck Game
// Mode, where Carryover is a non-Steam game): the D-pad moves between the
// controls, A presses, B goes back, LB/RB switch sections, X searches and Y
// syncs.
func (u *Shell) newNavigator() {
	u.nav = kit.NewNavigator(u.win, kit.NavigatorOptions{
		Back: func() {
			if u.route != "/" {
				u.back()
			}
		},
		Section: u.stepSection,
		Search: func() {
			if s, ok := u.page.(env.Searchable); ok {
				s.FocusSearch()
				u.openKeyboard()
			}
		},
		Sync: func() {
			if s, ok := u.page.(env.Syncable); ok {
				s.SyncNow()
			}
		},
		View: u.toggleFullScreen,
		TextField: func(f fyne.Focusable) {
			u.win.Canvas().Focus(f)
			u.openKeyboard()
		},
		DefaultFocus: func() fyne.Focusable {
			if p, ok := u.page.(env.PadFocuser); ok {
				if f := p.DefaultFocus(); f != nil {
					return f
				}
			}
			// The page's first control, not the window's: that would be the
			// sidebar's Library button, whatever page is open. A page with no
			// controls (an empty trash) starts on its own sidebar entry.
			if f := kit.FirstFocusable(u.content); f != nil {
				return f
			}
			if b := u.navBtns[sectionOf(u.route)]; b != nil {
				return b
			}
			return nil
		},
		PadChanged: func(present bool) {
			kit.SetVisible(u.padHints, present)
			if present && u.win.Canvas().Focused() == nil {
				u.nav.FocusDefault()
			}
		},
	})
	u.readPad(true)
}

// padWanted reports whether Carryover reads the gamepad itself: in Game Mode
// or when Steam started it, where Steam hands the controller over as a
// gamepad. Started by hand on a desktop, Steam's Desktop Layout already turns
// the controller into a mouse and keys (stick: mouse, D-pad: arrows, A:
// Enter); reading it as well would do every press twice.
func padWanted() bool { return gamemode.Active() || gamemode.SteamLaunched() }

// readPad starts (where padWanted) or stops reading the gamepad.
func (u *Shell) readPad(on bool) {
	u.nav.SetActive(on && padWanted())
}

// stepSection goes to the next (dir 1) or previous (-1) sidebar place.
func (u *Shell) stepSection(dir int) {
	cur := 0
	for i, s := range sections {
		if s == sectionOf(u.route) {
			cur = i
		}
	}
	u.navigate(sections[(cur+dir+len(sections))%len(sections)])
}

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

// applyFullScreen opens the main window full screen when that was last chosen
// for how Carryover was started; from Big Picture it starts full screen until
// the user says otherwise. Game Mode makes every window full screen itself.
func (u *Shell) applyFullScreen() {
	if gamemode.Active() {
		return
	}
	def := gamemode.SteamLaunched() && gamemode.BigPicture()
	if u.app.Preferences().BoolWithFallback(fullScreenPref(), def) {
		u.win.SetFullScreen(true)
	}
}

// toggleFullScreen switches full screen (the View button, F11) and remembers
// it.
func (u *Shell) toggleFullScreen() {
	on := !u.win.FullScreen()
	u.win.SetFullScreen(on)
	u.app.Preferences().SetBool(fullScreenPref(), on)
	u.refreshSidebar() // Exit comes and goes with the window's own close button
}

// showExit reports whether the sidebar offers Exit: where the window has no
// close button to reach (Game Mode, full screen) or Steam started Carryover
// (Big Picture, a controller across the room).
func (u *Shell) showExit() bool {
	return gamemode.Active() || gamemode.SteamLaunched() || (u.win != nil && u.win.FullScreen())
}

// openKeyboard opens Steam's on-screen keyboard when Steam started Carryover
// or in Game Mode (Big Picture on Windows, the Deck): a controller there has
// no other way to type. Started by hand, there is a keyboard. The request is
// handed to Steam in the background; the log says how that went (the first
// time, or every time it fails).
func (u *Shell) openKeyboard() {
	if !gamemode.KeyboardWanted() {
		return
	}
	if !gamemode.CanRequestKeyboard {
		// Windows: asking would open Big Picture's keyboard, which takes the
		// controller; Steam's own chord opens the overlay's.
		u.toast(lang.L("Press the Steam or Xbox button + X for Steam's keyboard."), kit.ToastInfo)
		return
	}
	go func() {
		via, err := openKeyboard()
		switch {
		case err != nil:
			log.Printf("open Steam keyboard: %v", err)
		case !keyboardLogged.Swap(true):
			log.Printf("asked Steam for its on-screen keyboard (via %s)", via)
		}
	}()
}

// openKeyboard asks Steam for its keyboard; tests replace it.
var openKeyboard = gamemode.OpenKeyboard

// keyboardLogged is set once a successful request was logged.
var keyboardLogged atomic.Bool

// focusForPad puts the focus where a gamepad starts on the page just shown,
// when a gamepad is connected.
func (u *Shell) focusForPad() {
	if u.nav != nil && u.nav.PadPresent() {
		u.nav.FocusDefault()
	}
}

// padHintBar is the line under the pages that says what the gamepad's
// buttons do; it shows while one is connected.
func (u *Shell) padHintBar() *fyne.Container {
	hint := func(button, what string) fyne.CanvasObject {
		return kit.HBox(container.NewCenter(kit.Badge(button, kit.BadgeInfo)), kit.Small(what))
	}
	bar := container.NewHBox(
		layout.NewSpacer(),
		hint("A", lang.L("Select")),
		hint("B", lang.L("Back")),
		hint("LB / RB", lang.L("Section")),
		hint("X", lang.L("Search")),
		hint("Y", lang.L("Sync")),
		hint("R", lang.L("Scroll")),
	)
	if !gamemode.Active() { // Game Mode is always full screen
		bar.Add(hint("View", lang.L("Full screen")))
	}
	c := container.NewPadded(bar)
	if u.nav == nil || !u.nav.PadPresent() {
		c.Hide()
	}
	return c
}
