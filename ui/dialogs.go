package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/lang"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/akfaiz/carryover/services"
	"github.com/akfaiz/carryover/ui/components"
	"github.com/akfaiz/carryover/ui/env"
	"github.com/akfaiz/carryover/ui/kit"
)

// showSyncProblems lists the games a "Sync all" could not sync, each opening
// its page, so a conflict or an error isn't lost in a toast.
func showSyncProblems(u env.Env, total int, problems []services.SyncResult) {
	var d components.DialogHider
	rows := kit.VBox()
	for _, r := range problems {
		state := kit.Badge(lang.L("Sync error"), kit.BadgeErr)
		detail := r.Error
		if r.Action == "conflict" {
			state = kit.Badge(lang.L("Conflict"), kit.BadgeErr)
			detail = lang.L("Changed on this device and in the cloud. Open the game to choose which saves to keep.")
		}
		id := r.GameID
		rows.Add(kit.NewTappable(kit.VBox(
			kit.HBox(widget.NewLabelWithStyle(r.Name, fyne.TextAlignLeading, fyne.TextStyle{Bold: true}), container.NewCenter(state)),
			kit.Muted(detail),
		), func() {
			d.Hide()
			u.Navigate("/game/" + id)
		}))
	}
	scroll := container.NewVScroll(rows)
	scroll.SetMinSize(fyne.NewSize(0, min(320, rows.MinSize().Height)))
	body := kit.VBox(
		kit.Muted(lang.N("{{.Count}} game synced.", total-len(problems), map[string]any{"Count": total - len(problems)})+" "+
			lang.L("These need a look — click one to open it:")),
		scroll,
		kit.HBox(layout.NewSpacer(), widget.NewButton(lang.L("Close"), func() { d.Hide() }),
			kit.Primary(lang.L("Show them in the library"), func() {
				d.Hide()
				setLibraryFilter(u, filterAttention)
				u.Navigate("/")
			})),
	)
	d = components.ShowCustom(u, lang.N("{{.Count}} game needs attention", len(problems), map[string]any{"Count": len(problems)}), body, 560)
}

// showAddGame asks what kind of game to add: one installed by Steam, another
// PC game, or a game played through an emulator. Each opens the editor set up
// for it.
func showAddGame(u env.Env) {
	var d components.DialogHider
	pick := func(f func()) func() { return func() { d.Hide(); f() } }
	tile := func(name string, icon fyne.Resource, f func()) fyne.CanvasObject {
		return container.New(layout.NewGridWrapLayout(fyne.NewSize(kit.ChoiceTileSize+16, kit.ChoiceTileSize)),
			kit.NewChoiceTile(name, icon, false, pick(f)))
	}
	body := kit.VBox(
		kit.HBox(
			tile(lang.L("From Steam"), theme.DownloadIcon(), func() { showSteamImport(u) }),
			tile(lang.L("PC game"), theme.ComputerIcon(), func() { u.Navigate("/new") }),
			tile(lang.L("Emulated game"), theme.MediaPlayIcon(), func() { u.Navigate("/new/emulator") }),
		),
		kit.Muted(lang.L("From Steam: a game Steam installed on this device. PC game: any other game, started from its executable or through Heroic. Emulated game: a game file you play through an emulator such as Eden or Ryujinx.")),
		kit.HBox(layout.NewSpacer(), widget.NewButton(lang.L("Cancel"), func() { d.Hide() })),
	)
	d = components.ShowCustom(u, lang.L("Add a game"), body, 500)
}
