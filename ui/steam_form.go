package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/lang"
	"fyne.io/fyne/v2/widget"

	"github.com/akfaiz/carryover/services"
)

// steamFormOpts configures a steamForm. Info is nil while Steam is still
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

	OnUser   func(string)
	OnTarget func(string)
	OnMode   func(string)
	OnAdd    func()
	OnRemove func()
}

// steamForm is the "add to Steam as a non-Steam game" form: account,
// shortcut type and the Add / Update / Remove buttons.
type steamForm struct {
	widget.BaseWidget
	content fyne.CanvasObject
}

func newSteamForm(o steamFormOpts) *steamForm {
	c := vbox()
	g := o.Game
	switch {
	case o.Info == nil:
		c.Add(muted(lang.L("Detecting Steam…")))
	case !o.Info.Found:
		c.Add(muted(lang.L("Steam was not found on this device.")))
	default:
		c.Add(muted(lang.L("Add {{.Game}} to Steam as a non-Steam game. Steam launches it through Carryover, so saves sync automatically before and after every session — also in Steam Deck Game Mode.",
			map[string]any{"Game": g.Name})))
		if len(o.Info.Users) > 1 {
			users := &choice{}
			for _, us := range o.Info.Users {
				users.add(us.ID, us.Name)
			}
			c.Add(field(lang.L("Steam account"), selectOf(users, o.User, o.OnUser)))
		}
		if len(o.Profiles) > 0 {
			targets := &choice{}
			targets.add("game", lang.L("The whole game: asks which emulator to use when it starts"))
			for _, pv := range o.Profiles {
				targets.add(pv.EmulatorID, lang.L("Only {{.Emulator}}", map[string]any{"Emulator": pv.Name}))
			}
			c.Add(field(lang.L("Shortcut for"), selectOf(targets, o.Target, o.OnTarget)))
		}
		if o.AsksAtLaunch {
			c.Add(muted(lang.L("Steam shows one game. When it starts, Carryover asks which emulator to use and, after a few seconds, starts the one you played last.")))
		}
		if o.HideMode {
			// Only the direct shortcut exists here.
		} else if g.Device.LaunchMode == "heroic" {
			c.Add(muted(lang.L("Steam runs Carryover, which starts the game through Heroic Games Launcher, so it keeps Heroic's settings (and, on Linux, its Wine prefix). Don't force a Proton version on this shortcut.")))
		} else {
			modes := &choice{}
			modes.add("direct", lang.L("Run Carryover (it starts the game)"))
			modes.add("steamwrap", lang.L("Run the game exe, wrapped by Carryover (pick Proton in Steam)"))
			c.Add(field(lang.L("Shortcut type"), selectOf(modes, o.Mode, o.OnMode)))
		}
		if o.Mode == "steamwrap" && g.Device.LaunchMode != "heroic" {
			t := lang.L("After adding, open the shortcut's Properties → Compatibility in Steam and force a Proton version. Its save prefix is available as {{.Token}}.",
				map[string]any{"Token": "{proton:shortcut}"})
			if g.Device.SteamAppID != "" {
				t = lang.L("After adding, open the shortcut's Properties → Compatibility in Steam and force a Proton version. Its save prefix is available as {{.Token}} (app id {{.AppID}}).",
					map[string]any{"Token": "{proton:shortcut}", "AppID": g.Device.SteamAppID})
			}
			c.Add(muted(t))
		}
		addLabel := lang.L("Add to Steam")
		if g.Device.SteamAppID != "" {
			addLabel = lang.L("Update Steam shortcut")
		}
		add := primary(addLabel, o.OnAdd)
		setEnabled(add, !o.Busy && o.User != "")
		row := hbox(add)
		if g.Device.SteamAppID != "" {
			rm := widget.NewButton(lang.L("Remove"), o.OnRemove)
			setEnabled(rm, !o.Busy)
			row.Add(rm)
		}
		c.Add(row)
		if o.Info.Running {
			c.Add(colored(lang.L("Steam is running — restart it after adding so it picks up the change."), widget.WarningImportance))
		}
	}
	f := &steamForm{content: c}
	f.ExtendBaseWidget(f)
	return f
}

// CreateRenderer implements fyne.Widget.
func (f *steamForm) CreateRenderer() fyne.WidgetRenderer { return widget.NewSimpleRenderer(f.content) }
