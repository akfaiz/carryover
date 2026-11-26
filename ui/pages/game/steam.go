package game

import (
	"runtime"

	"github.com/egoist/mygo/ui"

	"github.com/akfaiz/carryover/services"
	"github.com/akfaiz/carryover/ui/components"
	"github.com/akfaiz/carryover/ui/i18n"
	"github.com/akfaiz/carryover/ui/kit"
)

// steamCard is the Steam shortcut form. A game with emulators gets one
// shortcut for the whole game by default; it asks which emulator to use when it
// starts. The shortcut of one emulator is the other choice.
func (p *Page) steamCard(c *ui.Context, orig services.GameView, busy bool) {
	u := p.u
	sc := p.vm.steamChoice(orig)
	key, form, target, asks, hideMode := sc.key, sc.form, sc.target, sc.asks, sc.hideMode
	kit.Card(c, i18n.L("Steam"), func() {
		p.steamForm(c, steamFormOpts{
			Game: form, Info: p.vm.steam, User: p.vm.steamUser, Mode: p.vm.steamMode, Busy: busy,
			Profiles: runnable(orig.Emulators), Target: target, AsksAtLaunch: asks, HideMode: hideMode,
			Expanded: p.vm.steamExpanded,
			OnExpand: func() { p.vm.steamExpanded = true },
			OnTarget: func(v string) { p.vm.steamTarget = v },
			OnUser:   func(v string) { p.vm.steamUser = v },
			OnMode:   func(v string) { p.vm.steamMode = v },
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
						u.Toast(i18n.L("Added to Steam. Restart Steam to see it."), kit.ToastOK)
					} else {
						u.Toast(i18n.L("Added to Steam"), kit.ToastOK)
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
						u.Toast(i18n.L("Removed from Steam"), kit.ToastOK)
					} else if err == nil {
						u.Toast(i18n.L("No shortcut found"), kit.ToastOK)
					}
				})
			},
		})
		if p.vm.busy == "steam" {
			kit.BusyLine(c, i18n.L("Updating Steam…"))
		}
		if p.vm.wrapOpts != "" {
			kit.Disclosure(c, p.u.Navigator(), "wrap-opts", i18n.L("Already own it on Steam?"), &p.vm.wrapOpen, func() {
				kit.Muted(c, i18n.L("Paste this into the game's Steam launch options to sync around Steam's own launch:"))
				components.CopyField(c, u, p.vm.wrapOpts)
			})
		}
	})
}

// steamFormOpts configures the Steam form. Info is nil while Steam is still
// being detected.
type steamFormOpts struct {
	Game       services.GameView
	Info       *services.SteamInfo
	User, Mode string // selected Steam account and shortcut type
	Busy       bool

	// For a game with emulators. Profiles are its emulators; Target is "game"
	// (one shortcut for the whole game, which asks which emulator to use when it
	// starts) or the ID of the emulator the shortcut is for. AsksAtLaunch is set
	// when the shortcut is the whole-game one. HideMode hides the shortcut type:
	// a game played only through emulators has just the direct one.
	Profiles     []services.ProfileView
	Target       string
	AsksAtLaunch bool
	HideMode     bool

	// Once the shortcut exists the form folds into one line with Change and
	// Remove; Expanded shows it in full again (OnExpand asks for that).
	Expanded bool
	OnExpand func()

	OnUser   func(string)
	OnTarget func(string)
	OnMode   func(string)
	OnAdd    func()
	OnRemove func()
}

// steamForm is the "add to Steam as a non-Steam game" form: account, shortcut
// type and the Add / Update / Remove buttons.
func (p *Page) steamForm(c *ui.Context, o steamFormOpts) {
	nav, th := p.u.Navigator(), c.Theme()
	g := o.Game
	targets := func() {
		if len(o.Profiles) == 0 {
			return
		}
		ts := &kit.Choice{}
		ts.Add("game", i18n.L("The whole game: asks which emulator to use when it starts"))
		for _, pv := range o.Profiles {
			ts.Add(pv.EmulatorID, i18n.L("Only {{.Emulator}}", map[string]any{"Emulator": pv.Name}))
		}
		kit.Labeled(c, i18n.L("Shortcut for"), func() {
			kit.SelectOf(c, nav, "steam-target", i18n.L("Shortcut for"), ts, o.Target, o.OnTarget)
		})
	}
	remove := func() { p.btn(c, "steam-remove", kit.BtnDanger, kit.IconTrash, i18n.L("Remove"), o.Busy, o.OnRemove) }
	switch {
	case o.Info == nil:
		kit.Muted(c, i18n.L("Detecting Steam…"))
	case !o.Info.Found:
		kit.Muted(c, i18n.L("Steam was not found on this device."))
	case g.Device.SteamAppID != "" && !o.Expanded:
		kit.IconLine(c, kit.IconCheck, i18n.L("In Steam on this device"), th.Success)
		ui.Row(c).Gap(kit.S2).Children(func() {
			p.btn(c, "steam-change", kit.BtnSecondary, nil, i18n.L("Change…"), o.Busy, o.OnExpand)
			remove()
		})
		targets()
	default:
		kit.Muted(c, i18n.L("Add {{.Game}} to Steam as a non-Steam game. Steam launches it through Carryover, so saves sync automatically before and after every session — also in Steam Deck Game Mode.",
			map[string]any{"Game": g.Name}))
		if len(o.Info.Users) > 1 {
			users := &kit.Choice{}
			for _, us := range o.Info.Users {
				users.Add(us.ID, us.Name)
			}
			kit.Labeled(c, i18n.L("Steam account"), func() {
				kit.SelectOf(c, nav, "steam-user", i18n.L("Steam account"), users, o.User, o.OnUser)
			})
		}
		targets()
		if o.AsksAtLaunch {
			kit.Muted(c, i18n.L("Steam shows one game. When it starts, Carryover asks which emulator to use and, after a few seconds, starts the one you played last."))
		}
		switch {
		case o.HideMode: // only the direct shortcut exists here
		case g.Device.LaunchMode == "heroic":
			kit.Muted(c, i18n.L("Steam runs Carryover, which starts the game through Heroic Games Launcher, so it keeps Heroic's settings (and, on Linux, its Wine prefix). Don't force a Proton version on this shortcut."))
		case g.Device.LaunchMode == "lutris":
			kit.Muted(c, i18n.L("Steam runs Carryover, which starts the game through Lutris, so it keeps Lutris's runner and Wine prefix. Don't force a Proton version on this shortcut."))
		case g.Device.LaunchMode == "bottles":
			kit.Muted(c, i18n.L("Steam runs Carryover, which starts the game through Bottles, in the game's bottle. Don't force a Proton version on this shortcut."))
		default:
			modes := &kit.Choice{}
			modes.Add("direct", i18n.L("Run Carryover (it starts the game)"))
			if runtime.GOOS == "darwin" { // no Proton on macOS
				modes.Add("steamwrap", i18n.L("Run the game, wrapped by Carryover"))
			} else {
				modes.Add("steamwrap", i18n.L("Run the game exe, wrapped by Carryover (pick Proton in Steam)"))
			}
			kit.Labeled(c, i18n.L("Shortcut type"), func() {
				kit.SelectOf(c, nav, "steam-mode", i18n.L("Shortcut type"), modes, o.Mode, o.OnMode)
			})
		}
		throughLauncher := g.Device.LaunchMode == "heroic" || g.Device.LaunchMode == "lutris" || g.Device.LaunchMode == "bottles"
		if o.Mode == "steamwrap" && !throughLauncher && runtime.GOOS != "darwin" {
			t := i18n.L("After adding, open the shortcut's Properties → Compatibility in Steam and force a Proton version. Its save prefix is available as {{.Token}}.",
				map[string]any{"Token": "{proton:shortcut}"})
			if g.Device.SteamAppID != "" {
				t = i18n.L("After adding, open the shortcut's Properties → Compatibility in Steam and force a Proton version. Its save prefix is available as {{.Token}} (app id {{.AppID}}).",
					map[string]any{"Token": "{proton:shortcut}", "AppID": g.Device.SteamAppID})
			}
			kit.Muted(c, t)
		}
		addLabel := i18n.L("Add to Steam")
		if g.Device.SteamAppID != "" {
			addLabel = i18n.L("Update Steam shortcut")
		}
		ui.Row(c).Gap(kit.S2).Children(func() {
			p.btn(c, "steam-add", kit.BtnPrimary, nil, addLabel, o.Busy || o.User == "", o.OnAdd)
			if g.Device.SteamAppID != "" {
				remove()
			}
		})
		if o.Info.Running {
			kit.Colored(c, i18n.L("Steam is running — restart it after adding so it picks up the change."), th.Warning)
		}
	}
}
