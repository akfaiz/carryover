package ui

import (
	"log"
	"strings"
	"sync/atomic"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/lang"
	"fyne.io/fyne/v2/layout"

	"github.com/akfaiz/carryover/internal/app/gamemode"
	"github.com/akfaiz/carryover/internal/ui/kit"
)

// padFocuser pages say where a gamepad starts on them (nil: the top left
// control).
type padFocuser interface {
	defaultFocus() fyne.Focusable
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

// newNavigator lets the main window be used with a gamepad (Steam Deck Game
// Mode, where Carryover is a non-Steam game): the D-pad moves between the
// controls, A presses, B goes back, LB/RB switch sections, X searches and Y
// syncs.
func (u *UI) newNavigator() {
	u.nav = kit.NewNavigator(u.win, kit.NavigatorOptions{
		Back: func() {
			if u.route != "/" {
				u.back()
			}
		},
		Section: u.stepSection,
		Search: func() {
			if s, ok := u.page.(searchable); ok {
				s.focusSearch()
				u.openKeyboard()
			}
		},
		Sync: func() {
			if s, ok := u.page.(syncable); ok {
				s.syncNow()
			}
		},
		TextField: func(f fyne.Focusable) {
			u.win.Canvas().Focus(f)
			u.openKeyboard()
		},
		DefaultFocus: func() fyne.Focusable {
			if p, ok := u.page.(padFocuser); ok {
				if f := p.defaultFocus(); f != nil {
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
	u.nav.SetActive(true)
}

// stepSection goes to the next (dir 1) or previous (-1) sidebar place.
func (u *UI) stepSection(dir int) {
	cur := 0
	for i, s := range sections {
		if s == sectionOf(u.route) {
			cur = i
		}
	}
	u.navigate(sections[(cur+dir+len(sections))%len(sections)])
}

// openKeyboard opens Steam's on-screen keyboard in Game Mode, where there is
// no other way to type. Elsewhere there is a keyboard (or Steam + X). The
// request is handed to Steam in the background; the log says how that went
// (the first time, or every time it fails).
func (u *UI) openKeyboard() {
	if !gamemode.Active() {
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
func (u *UI) focusForPad() {
	if u.nav != nil && u.nav.PadPresent() {
		u.nav.FocusDefault()
	}
}

// padHintBar is the line under the pages that says what the gamepad's
// buttons do; it shows while one is connected.
func (u *UI) padHintBar() *fyne.Container {
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
	c := container.NewPadded(bar)
	if u.nav == nil || !u.nav.PadPresent() {
		c.Hide()
	}
	return c
}
