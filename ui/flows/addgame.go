package flows

import (
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/akfaiz/carryover/services"
	"github.com/akfaiz/carryover/ui/env"
	"github.com/akfaiz/carryover/ui/i18n"
	"github.com/akfaiz/carryover/ui/kit"
)

// ShowAddGame asks what kind of game to add: one installed by Steam (or by
// Heroic or Lutris, when installed on this device), another PC game, or a
// game played through an emulator. Each opens the editor set up for it.
func ShowAddGame(u env.Env) {
	kit.Async(func() (services.Launchers, error) { return u.Lib().Launchers(), nil }, func(l services.Launchers, _ error) {
		showAddGame(u, l)
	})
}

type addTile struct {
	name  string
	icon  *ui.SVG
	about string
	do    func()
}

func showAddGame(u env.Env, l services.Launchers) {
	var d *kit.Dialog
	heroic, lutris := l.Heroic, l.Lutris
	tiles := []addTile{{i18n.L("From Steam"), kit.IconDownload, i18n.L("From Steam: a game Steam installed on this device."),
		func() { ShowSteamImport(u) }}}
	if heroic {
		tiles = append(tiles, addTile{i18n.L("From Heroic"), kit.IconStorage, i18n.L("From Heroic: a GOG, Epic or Amazon game Heroic installed."),
			func() { ShowHeroicImport(u) }})
	}
	if lutris {
		tiles = append(tiles, addTile{i18n.L("From Lutris"), kit.IconFolder, i18n.L("From Lutris: a game Lutris installed."),
			func() { ShowLutrisImport(u) }})
	}
	pc := i18n.L("PC game: any other game, started from its executable.")
	switch {
	case heroic && lutris:
		pc = i18n.L("PC game: any other game, started from its executable or through Heroic or Lutris.")
	case heroic:
		pc = i18n.L("PC game: any other game, started from its executable or through Heroic.")
	case lutris:
		pc = i18n.L("PC game: any other game, started from its executable or through Lutris.")
	}
	tiles = append(tiles,
		addTile{i18n.L("PC game"), kit.IconComputer, pc, func() { u.Navigate("/new") }},
		addTile{i18n.L("Emulated game"), kit.IconPlay,
			i18n.L("Emulated game: a game file you play through an emulator such as Eden or Ryujinx."),
			func() { u.Navigate("/new/emulator") }})
	var help []string
	for _, t := range tiles {
		help = append(help, t.about)
	}
	d = u.Dialogs().Show(&kit.Dialog{
		Title: i18n.L("Add a game"),
		Width: float32(len(tiles))*(kit.ChoiceTileSize+16) + 80,
		Body: func(c *ui.Context, nav *kit.Navigator) {
			ui.Row(c).Gap(8).Wrap().Children(func() {
				for _, t := range tiles {
					kit.ChoiceTile(c, nav, "add:"+t.name, t.name, t.icon, false, func() { d.Close(); t.do() })
				}
			})
			kit.Muted(c, strings.Join(help, " "))
			cancel := func() { d.Close() }
			if nav.Item(nav.Handle("add-cancel"), ui.Button(c.Key("add-cancel"), i18n.L("Cancel")), cancel, kit.FocusSafe).Clicked() {
				cancel()
			}
		},
	})
}
