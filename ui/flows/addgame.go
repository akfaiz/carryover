package flows

import (
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/akfaiz/carryover/ui/i18n"

	"github.com/akfaiz/carryover/services"
	"github.com/akfaiz/carryover/ui/components"
	"github.com/akfaiz/carryover/ui/env"
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

func showAddGame(u env.Env, l services.Launchers) {
	var d components.DialogHider
	pick := func(f func()) func() { return func() { d.Hide(); f() } }
	var tiles []fyne.CanvasObject
	var help []string
	add := func(name string, icon fyne.Resource, about string, f func()) {
		tiles = append(tiles, container.New(layout.NewGridWrapLayout(fyne.NewSize(kit.ChoiceTileSize+16, kit.ChoiceTileSize)),
			kit.NewChoiceTile(name, icon, false, pick(f))))
		help = append(help, about)
	}
	heroic, lutris := l.Heroic, l.Lutris

	add(i18n.L("From Steam"), theme.DownloadIcon(), i18n.L("From Steam: a game Steam installed on this device."),
		func() { ShowSteamImport(u) })
	if heroic {
		add(i18n.L("From Heroic"), theme.StorageIcon(), i18n.L("From Heroic: a GOG, Epic or Amazon game Heroic installed."),
			func() { ShowHeroicImport(u) })
	}
	if lutris {
		add(i18n.L("From Lutris"), theme.FolderOpenIcon(), i18n.L("From Lutris: a game Lutris installed."),
			func() { ShowLutrisImport(u) })
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
	add(i18n.L("PC game"), theme.ComputerIcon(), pc, func() { u.Navigate("/new") })
	add(i18n.L("Emulated game"), theme.MediaPlayIcon(),
		i18n.L("Emulated game: a game file you play through an emulator such as Eden or Ryujinx."),
		func() { u.Navigate("/new/emulator") })

	body := kit.VBox(
		kit.HBox(tiles...),
		kit.Muted(strings.Join(help, " ")),
		kit.HBox(layout.NewSpacer(), widget.NewButton(i18n.L("Cancel"), func() { d.Hide() })),
	)
	width := float32(len(tiles))*(kit.ChoiceTileSize+16) + 80
	d = components.ShowCustom(u, i18n.L("Add a game"), body, width)
}
