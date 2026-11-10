package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/widget"

	"github.com/akfaiz/playrelay/services"
)

// steamFormOpts configures a steamForm. Info is nil while Steam is still
// being detected.
type steamFormOpts struct {
	Game       services.GameView
	Info       *services.SteamInfo
	User, Mode string // selected Steam account and shortcut type
	Busy       bool

	OnUser   func(string)
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
		c.Add(muted("Detecting Steam…"))
	case !o.Info.Found:
		c.Add(muted("Steam was not found on this device."))
	default:
		c.Add(muted("Add " + g.Name + " to Steam as a non-Steam game. Steam launches it through PlayRelay, so saves sync " +
			"automatically before and after every session — also in Steam Deck Game Mode."))
		if len(o.Info.Users) > 1 {
			users := &choice{}
			for _, us := range o.Info.Users {
				users.add(us.ID, us.Name)
			}
			c.Add(field("Steam account", selectOf(users, o.User, o.OnUser)))
		}
		modes := &choice{}
		modes.add("direct", "Run PlayRelay (it starts the game)")
		modes.add("steamwrap", "Run the game exe, wrapped by PlayRelay (pick Proton in Steam)")
		c.Add(field("Shortcut type", selectOf(modes, o.Mode, o.OnMode)))
		if o.Mode == "steamwrap" {
			t := "After adding, open the shortcut's Properties → Compatibility in Steam and force a Proton version. " +
				"Its save prefix is available as {proton:shortcut}"
			if g.Device.SteamAppID != "" {
				t += " (app id " + g.Device.SteamAppID + ")"
			}
			c.Add(muted(t + "."))
		}
		addLabel := "Add to Steam"
		if g.Device.SteamAppID != "" {
			addLabel = "Update Steam shortcut"
		}
		add := primary(addLabel, o.OnAdd)
		setEnabled(add, !o.Busy && o.User != "")
		row := hbox(add)
		if g.Device.SteamAppID != "" {
			rm := widget.NewButton("Remove", o.OnRemove)
			setEnabled(rm, !o.Busy)
			row.Add(rm)
		}
		c.Add(row)
		if o.Info.Running {
			c.Add(colored("Steam is running — restart it after adding so it picks up the change.", widget.WarningImportance))
		}
	}
	f := &steamForm{content: c}
	f.ExtendBaseWidget(f)
	return f
}

// CreateRenderer implements fyne.Widget.
func (f *steamForm) CreateRenderer() fyne.WidgetRenderer { return widget.NewSimpleRenderer(f.content) }
