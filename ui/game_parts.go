package ui

import (
	"fmt"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/akfaiz/carryover/internal/syncer"
	"github.com/akfaiz/carryover/services"
)

// saveLocationRow shows one save location of a game on this device: its
// label and either the resolved folder (click to open), an error or a hint.
func saveLocationRow(u *UI, label, resolved, pathErr string) fyne.CanvasObject {
	name := widget.NewLabelWithStyle(label, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	var val fyne.CanvasObject
	switch {
	case pathErr != "":
		val = colored(pathErr, widget.DangerImportance)
	case resolved != "":
		l := mono(resolved)
		l.Importance = widget.HighImportance
		val = newTappable(container.NewBorder(nil, nil, widget.NewIcon(theme.FolderOpenIcon()), nil, l), func() { u.openFolder(resolved) })
	default:
		val = colored("not set on this device", widget.WarningImportance)
	}
	return labeledRow(110, name, val)
}

// backupRow is one cloud backup with a Restore button.
func backupRow(b syncer.Backup, restoring, canRestore bool, onRestore func()) fyne.CanvasObject {
	desc := b.DeviceName + "'s local saves"
	if b.Kind != "local" {
		desc = "cloud saves (replaced by " + b.DeviceName + ")"
	}
	label := "Restore"
	if restoring {
		label = "…"
	}
	restore := widget.NewButton(label, onRestore)
	setEnabled(restore, canRestore)
	return labeledRow(140, widget.NewLabel(fmtDate(b.Time)), muted(desc), restore)
}

// otherDevicesCard lists the save paths other devices use for the game.
func otherDevicesCard(g services.GameView) fyne.CanvasObject {
	devs := vbox()
	for _, d := range g.OtherDevices {
		devs.Add(widget.NewLabelWithStyle(d.DeviceName, fyne.TextAlignLeading, fyne.TextStyle{Bold: true}))
		for _, s := range g.Slots {
			if v := d.Paths[s.Key]; v != "" {
				l := mono(labelOf(s.Label, s.Key) + ": " + v)
				l.Importance = widget.LowImportance
				l.SizeName = theme.SizeNameCaptionText
				devs.Add(l)
			}
		}
	}
	return card("Other devices", devs)
}

var planText = map[string]string{
	"none":     "Local and cloud saves are identical.",
	"push":     "Local saves are newer — they will be uploaded.",
	"pull":     "Cloud saves are newer — they will be downloaded.",
	"conflict": "Both changed since the last sync. Choose which to keep.",
}

// planView compares this device's saves with the cloud and, on a conflict,
// offers to keep either side.
type planView struct {
	widget.BaseWidget
	content fyne.CanvasObject
}

// newPlanView shows pl. With resolvable, "Keep …" buttons call onResolve with
// "local" or "cloud".
func newPlanView(pl *services.PlanView, resolvable bool, onResolve func(resolution string)) *planView {
	local := vbox(muted("This device"),
		widget.NewLabel(fmt.Sprintf("%d files · %s\nmodified %s", pl.Local.Files, formatBytes(pl.Local.Bytes), fmtDate(pl.Local.Newest))))
	cloudTitle := "Cloud"
	if pl.Remote.DeviceName != "" {
		cloudTitle += " (from " + pl.Remote.DeviceName + ")"
	}
	cloudText := "no Carryover saves for this game yet"
	if pl.HasRemote {
		cloudText = fmt.Sprintf("%d files · %s\nuploaded %s", pl.Remote.Files, formatBytes(pl.Remote.Bytes), fmtDate(pl.Remote.Time))
	}
	c := vbox(
		widget.NewLabelWithStyle(planText[pl.Action], fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		container.NewGridWithColumns(2, local, vbox(muted(cloudTitle), widget.NewLabel(cloudText))),
	)
	if resolvable {
		keepCloud := widget.NewButton("Keep cloud saves", func() { onResolve("cloud") })
		setEnabled(keepCloud, pl.HasRemote)
		c.Add(hbox(widget.NewButton("Keep this device's saves", func() { onResolve("local") }), keepCloud))
	}
	v := &planView{content: c}
	v.ExtendBaseWidget(v)
	return v
}

// CreateRenderer implements fyne.Widget.
func (v *planView) CreateRenderer() fyne.WidgetRenderer { return widget.NewSimpleRenderer(v.content) }
