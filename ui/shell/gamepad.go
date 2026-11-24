package shell

import (
	"log"

	"github.com/egoist/mygo/ui"

	"github.com/akfaiz/carryover/internal/app/gamemode"
	"github.com/akfaiz/carryover/ui/env"
	"github.com/akfaiz/carryover/ui/i18n"
	"github.com/akfaiz/carryover/ui/kit"
)

// navOptions lets the main window be used with a gamepad (Steam Deck Game
// Mode, where Carryover is a non-Steam game): the D-pad moves between the
// controls, A presses, B goes back, LB/RB switch sections, X searches and Y
// syncs.
func (u *Shell) navOptions() kit.NavigatorOptions {
	return kit.NavigatorOptions{
		Back: func() {
			if u.route != "/" {
				u.back()
			}
		},
		Section: u.stepSection,
		Search: func() {
			if s, ok := u.page.(env.Searchable); ok {
				s.FocusSearch()
			}
		},
		Sync: func() {
			if s, ok := u.page.(env.Syncable); ok {
				s.SyncNow()
			}
		},
		DefaultFocus: func() *ui.Handle {
			if p, ok := u.page.(env.PadFocuser); ok {
				return p.DefaultFocus()
			}
			return nil
		},
		TextField:  u.openKeyboard,
		View:       u.toggleFullScreen,
		PadChanged: func(present bool) { kit.Redraw() },
	}
}

// readPad starts (where padWanted) or stops reading the gamepad.
func (u *Shell) readPad(on bool) {
	if u.stopPad != nil {
		u.stopPad()
		u.stopPad = nil
	}
	if on && (u.wrapper || padWanted()) {
		u.stopPad = u.nav.StartPad(kit.MygoPad, u.win.Update)
	}
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

// applyFullScreen opens the main window full screen when that was last chosen
// for how Carryover was started; from Big Picture it starts full screen until
// the user says otherwise. Game Mode makes every window full screen itself.
func (u *Shell) applyFullScreen() {
	if gamemode.Active() {
		return
	}
	def := gamemode.SteamLaunched() && gamemode.BigPicture()
	if u.prefs.BoolWithFallback(fullScreenPref(), def) {
		u.win.SetFullScreen(true)
	}
}

// toggleFullScreen switches full screen (the View button, F11) and remembers
// it.
func (u *Shell) toggleFullScreen() {
	on := !u.win.IsFullScreen()
	u.win.SetFullScreen(on)
	u.prefs.SetBool(fullScreenPref(), on)
	kit.Redraw() // Exit comes and goes with the window's own close button
}

// showExit reports whether the sidebar offers Exit: where the window has no
// close button to reach (Game Mode, full screen) or Steam started Carryover
// (Big Picture, a controller across the room).
func (u *Shell) showExit() bool {
	return gamemode.Active() || gamemode.SteamLaunched() || (u.win != nil && u.win.IsFullScreen())
}

// focusForPad puts the focus where a gamepad starts on the page just shown,
// when a gamepad is connected: the next frame's first press does it.
func (u *Shell) focusForPad() {
	if u.nav != nil && u.nav.PadPresent() {
		u.nav.Press(kit.PadDown)
	}
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
		u.toast(i18n.L("Press the Steam or Xbox button + X for Steam's keyboard."), kit.ToastInfo)
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
