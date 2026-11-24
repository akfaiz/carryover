package settings

import (
	"github.com/egoist/mygo/ui"

	"github.com/akfaiz/carryover/services"
	"github.com/akfaiz/carryover/ui/components"
	"github.com/akfaiz/carryover/ui/i18n"
	"github.com/akfaiz/carryover/ui/kit"
)

// devicesCard lists the other devices the library holds settings of: forget
// one that is gone, or make this device an earlier install of itself again
// (reinstalling Carryover or clearing its data gives a device a new ID).
func (p *Page) devicesCard(c *ui.Context) {
	kit.Card(c, i18n.L("Other devices"), func() {
		kit.Muted(c, i18n.L("Devices with games or emulators set up in your library. Reinstalling Carryover or clearing its data makes a device start over as a new one: choose \"This is me\" on its earlier entry to take its settings back, and forget devices you no longer use."))
		others := 0
		for _, d := range p.devs {
			if d.This {
				continue
			}
			others++
			p.deviceRow(c, d)
		}
		if others == 0 {
			kit.Muted(c, i18n.L("No other devices."))
		}
	})
}

// deviceName is a device's name, or a stand-in when it has none.
func deviceName(d services.DeviceView) string {
	if d.Name == "" {
		return i18n.L("Unnamed device")
	}
	return d.Name
}

func (p *Page) deviceRow(c *ui.Context, d services.DeviceView) {
	nav := p.u.Navigator()
	name := deviceName(d)
	button := func(key, label string, danger bool, action func()) {
		k := [3]any{"device", d.ID, key}
		b := ui.Button(c.Key(k), label)
		if danger {
			b = b.TextColor(c.Theme().Danger)
		}
		if nav.Item(nav.Handle(k), b, action).Clicked() {
			action()
		}
	}
	ui.Column(c).Gap(2).Children(func() {
		kit.CardHead(c, name, func() {
			button("reuse", i18n.L("This is me…"), false, func() {
				components.ConfirmDanger(p.u, components.ConfirmOpts{
					Title: i18n.L("Use the settings of {{.Name}} here", map[string]any{"Name": name}),
					Message: i18n.L("This device takes over the game and emulator settings of {{.Name}} (where both have settings for a game, the newer ones stay), and {{.Name}} is removed from the list on every device. Saves are not touched.",
						map[string]any{"Name": name}),
					Action: i18n.L("This is me"),
					OnConfirm: func(bool) {
						p.deviceAction(func() error { return p.u.Sync().ReuseDevice(d.ID) }, i18n.L("This device now has the settings of {{.Name}}.", map[string]any{"Name": name}))
					},
				})
			})
			button("forget", i18n.L("Forget…"), true, func() {
				components.ConfirmDanger(p.u, components.ConfirmOpts{
					Title: i18n.L("Forget device"),
					Message: i18n.L("Remove {{.Name}} and its game and emulator settings from the library on every device? Its cloud saves and backups are kept. Only forget a device you no longer use: one still in use loses its setup too.",
						map[string]any{"Name": name}),
					Action: i18n.L("Forget"),
					OnConfirm: func(bool) {
						p.deviceAction(func() error { return p.u.Sync().ForgetDevice(d.ID) }, i18n.L("Device forgotten"))
					},
				})
			})
		})
		if d.SameName {
			kit.Badge(c, i18n.L("Same name as this device"), kit.BadgeInfo)
		}
		kit.Small(c, i18n.L("Last change {{.When}} · games: {{.Games}} · emulators: {{.Emulators}}",
			map[string]any{"When": components.FmtDate(d.LastSeen), "Games": d.Games, "Emulators": d.Emulators}))
	})
}

// deviceAction runs a device change off the UI thread, then tells how it went
// and reads the page again (the device ID may have changed).
func (p *Page) deviceAction(run func() error, done string) {
	kit.Async(func() (struct{}, error) { return struct{}{}, run() }, func(_ struct{}, err error) {
		if err != nil {
			p.u.ToastErr(err)
			return
		}
		p.u.Toast(done, kit.ToastOK)
		p.u.Store().Refresh()
		p.reload()
	})
}
