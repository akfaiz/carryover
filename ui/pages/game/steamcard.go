package game

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/lang"

	"github.com/akfaiz/carryover/services"
	"github.com/akfaiz/carryover/ui/components"
	"github.com/akfaiz/carryover/ui/kit"
)

// steamCard is the Steam shortcut form. A game with emulators gets one
// shortcut for the whole game by default; it asks which emulator to use when it
// starts. The shortcut of one emulator is the other choice.
func (p *Page) steamCard(orig services.GameView, busy bool) fyne.CanvasObject {
	u := p.u
	sc := p.vm.steamChoice(orig)
	key, form, target, asks, hideMode := sc.key, sc.form, sc.target, sc.asks, sc.hideMode
	c := kit.VBox(newSteamForm(steamFormOpts{
		Game: form, Info: p.vm.steam, User: p.vm.steamUser, Mode: p.vm.steamMode, Busy: busy,
		Profiles: orig.Emulators, Target: target, AsksAtLaunch: asks, HideMode: hideMode,
		Expanded: p.vm.steamExpanded,
		OnExpand: func() { p.vm.steamExpanded = true; p.render() },
		OnTarget: func(v string) { p.vm.steamTarget = v; p.render() },
		OnUser:   func(v string) { p.vm.steamUser = v },
		OnMode:   func(v string) { p.vm.steamMode = v; p.render() },
		OnAdd: func() {
			var res services.AddToSteamResult
			p.run("steam", func() error {
				var err error
				res, err = p.vm.AddToSteam(key, hideMode)
				return err
			}, func(err error) {
				if err != nil {
					return
				}
				p.vm.steamExpanded = false
				if res.SteamRunning {
					u.Toast(lang.L("Added to Steam. Restart Steam to see it."), kit.ToastOK)
				} else {
					u.Toast(lang.L("Added to Steam"), kit.ToastOK)
				}
			})
		},
		OnRemove: func() {
			var n int
			p.run("steam", func() error {
				var err error
				n, err = p.vm.RemoveFromSteam(key)
				return err
			}, func(err error) {
				if err == nil && n > 0 {
					u.Toast(lang.L("Removed from Steam"), kit.ToastOK)
				} else if err == nil {
					u.Toast(lang.L("No shortcut found"), kit.ToastOK)
				}
			})
		},
	}))
	if p.vm.busy == "steam" {
		c.Add(kit.BusyLine(lang.L("Updating Steam…")))
	}
	if p.vm.wrapOpts != "" {
		p.wrapSection = kit.NewAccordionSection(lang.L("Already own it on Steam?"), kit.VBox(
			kit.Muted(lang.L("Paste this into the game's Steam launch options to sync around Steam's own launch:")),
			components.CopyField(u, p.vm.wrapOpts)), p.vm.wrapOpen)
		c.Add(p.wrapSection)
	}
	return kit.Card(lang.L("Steam"), c)
}
