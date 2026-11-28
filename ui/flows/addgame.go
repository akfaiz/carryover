package flows

import (
	"github.com/egoist/mygo/ui"

	"github.com/akfaiz/carryover/services"
	"github.com/akfaiz/carryover/ui/env"
	"github.com/akfaiz/carryover/ui/i18n"
	"github.com/akfaiz/carryover/ui/kit"
)

// ShowAddGame asks what kind of game to add: one installed by Steam (or one
// Heroic, Lutris or Bottles has, when installed on this device), another PC
// game, or a game played through an emulator. Each opens the editor set up for
// it.
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
	tiles := []addTile{{i18n.L("From Steam"), kit.IconSteam, i18n.L("From Steam: a game Steam installed on this device."),
		func() { ShowSteamImport(u) }}}
	if heroic {
		tiles = append(tiles, addTile{i18n.L("From Heroic"), kit.IconHeroic, i18n.L("From Heroic: a GOG, Epic or Amazon game Heroic installed."),
			func() { ShowHeroicImport(u) }})
	}
	if lutris {
		tiles = append(tiles, addTile{i18n.L("From Lutris"), kit.IconLutris, i18n.L("From Lutris: a game Lutris installed."),
			func() { ShowLutrisImport(u) }})
	}
	if l.Bottles {
		tiles = append(tiles, addTile{i18n.L("From Bottles"), kit.IconComputer, i18n.L("From Bottles: a program added to one of your bottles."),
			func() { ShowBottlesImport(u) }})
	}
	pc := i18n.L("PC game: any other game, started from its executable.")
	switch {
	case l.Bottles:
		pc = i18n.L("PC game: any other game, started from its executable or through another launcher.")
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
	d = u.Dialogs().Show(&kit.Dialog{
		Title: i18n.L("Add a game"),
		Width: float32(len(tiles))*(kit.ChoiceTileSize+16) + 80,
		Body: func(c *ui.Context, nav *kit.Navigator) {
			hovered, focused := -1, -1
			kit.ChoiceGrid(c, d.ContentWidth(c), len(tiles), func(i int) {
				t := tiles[i]
				tile := kit.ChoiceTile(c, nav, "add:"+t.name, t.name, t.icon, false, func() { d.Close(); t.do() })
				if tile.Hovered() {
					hovered = i
				}
				if tile.Focused() {
					focused = i
				}
			})
			// With one tile under the pointer and another focused, the pointer wins.
			about := hovered
			if about < 0 {
				about = focused
			}
			tileAbout(c, tiles, about)
			kit.Footer(c, func() {
				kit.Btn(c, nav, "add-cancel", kit.BtnSecondary, nil, i18n.L("Cancel"), func() { d.Close() }, kit.BtnItem(kit.FocusSafe))
			})
		},
	})
}

// tileAbout explains the tile at index about, none for -1. Every text is laid
// out in the same cell, so the room is the longest one's whichever shows: a
// dialog that changed height would move its tiles from under the pointer.
func tileAbout(c *ui.Context, tiles []addTile, about int) {
	ui.Grid(c).Columns(1).Children(func() {
		for i, t := range tiles {
			text := kit.Muted(c.Key(t.name), t.about).ColumnStart(1).RowStart(1)
			if i != about {
				text.Invisible()
			}
		}
	})
}
