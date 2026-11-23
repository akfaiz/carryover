package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"

	"github.com/akfaiz/carryover/ui/i18n"

	"github.com/akfaiz/carryover/services"
	"github.com/akfaiz/carryover/ui/components"
	"github.com/akfaiz/carryover/ui/env"
	"github.com/akfaiz/carryover/ui/kit"
	"github.com/akfaiz/carryover/ui/pages/library"
)

// showSyncProblems lists the games a "Sync all" could not sync, each opening
// its page, so a conflict or an error isn't lost in a toast.
func showSyncProblems(u env.Env, total int, problems []services.SyncResult) {
	var d components.DialogHider
	rows := kit.VBox()
	for _, r := range problems {
		state := kit.Badge(i18n.L("Sync error"), kit.BadgeErr)
		detail := r.Error
		if r.Action == "conflict" {
			state = kit.Badge(i18n.L("Conflict"), kit.BadgeErr)
			detail = i18n.L("Changed on this device and in the cloud. Open the game to choose which saves to keep.")
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
		kit.Muted(i18n.N("{{.Count}} game synced.", total-len(problems), map[string]any{"Count": total - len(problems)})+" "+
			i18n.L("These need a look — click one to open it:")),
		scroll,
		kit.HBox(layout.NewSpacer(), widget.NewButton(i18n.L("Close"), func() { d.Hide() }),
			kit.Primary(i18n.L("Show them in the library"), func() {
				d.Hide()
				library.ShowAttention(u)
				u.Navigate("/")
			})),
	)
	d = components.ShowCustom(u, i18n.N("{{.Count}} game needs attention", len(problems), map[string]any{"Count": len(problems)}), body, 560)
}
