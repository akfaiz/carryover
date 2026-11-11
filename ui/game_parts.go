package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/lang"
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
		val = colored(lang.L("not set on this device"), widget.WarningImportance)
	}
	return labeledRow(110, name, val)
}

// backupRow is one backup with Pin, Delete and Restore buttons (no Pin for
// a safety copy kept on this device).
func backupRow(b syncer.Backup, restoring, canRestore, canPin bool, onRestore, onPin, onDelete func()) fyne.CanvasObject {
	var desc string
	switch {
	case b.Local:
		desc = lang.L("{{.Device}}'s saves (copy kept on this device)", map[string]any{"Device": b.DeviceName})
	case b.Kind == "local":
		desc = lang.L("{{.Device}}'s local saves", map[string]any{"Device": b.DeviceName})
	case b.Kind == "manual":
		desc = lang.L("{{.Device}}'s saves (manual backup)", map[string]any{"Device": b.DeviceName})
	default:
		desc = lang.L("cloud saves (replaced by {{.Device}})", map[string]any{"Device": b.DeviceName})
	}
	pinLabel := lang.L("Pin")
	if b.Pinned {
		desc += " · " + lang.L("pinned")
		pinLabel = lang.L("Unpin")
	}
	label := lang.L("Restore")
	if restoring {
		label = "…"
	}
	restore := widget.NewButton(label, onRestore)
	setEnabled(restore, canRestore)
	pin := widget.NewButton(pinLabel, onPin)
	setEnabled(pin, canPin)
	del := widget.NewButtonWithIcon("", theme.DeleteIcon(), onDelete)
	setEnabled(del, canPin)
	if b.Local {
		return labeledRow(140, widget.NewLabel(fmtDate(b.Time)), muted(desc), del, restore)
	}
	return labeledRow(140, widget.NewLabel(fmtDate(b.Time)), muted(desc), pin, del, restore)
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
	return card(lang.L("Other devices"), devs)
}

// planText describes a sync plan action in words.
func planText(action string) string {
	switch action {
	case "none":
		return lang.L("Local and cloud saves are identical.")
	case "push":
		return lang.L("Local saves are newer — they will be uploaded.")
	case "pull":
		return lang.L("Cloud saves are newer — they will be downloaded.")
	case "conflict":
		return lang.L("Both changed since the last sync. Choose which to keep.")
	}
	return ""
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
	local := vbox(muted(lang.L("This device")),
		widget.NewLabel(lang.N("{{.Count}} file · {{.Size}}\nmodified {{.Date}}", pl.Local.Files,
			map[string]any{"Count": pl.Local.Files, "Size": formatBytes(pl.Local.Bytes), "Date": fmtDate(pl.Local.Newest)})))
	cloudTitle := lang.L("Cloud")
	if pl.Remote.DeviceName != "" {
		cloudTitle = lang.L("Cloud (from {{.Device}})", map[string]any{"Device": pl.Remote.DeviceName})
	}
	cloudText := lang.L("no Carryover saves for this game yet")
	if pl.HasRemote {
		cloudText = lang.N("{{.Count}} file · {{.Size}}\nuploaded {{.Date}}", pl.Remote.Files,
			map[string]any{"Count": pl.Remote.Files, "Size": formatBytes(pl.Remote.Bytes), "Date": fmtDate(pl.Remote.Time)})
	}
	c := vbox(
		widget.NewLabelWithStyle(planText(pl.Action), fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		container.NewGridWithColumns(2, local, vbox(muted(cloudTitle), widget.NewLabel(cloudText))),
	)
	if resolvable {
		keepCloud := widget.NewButton(lang.L("Keep cloud saves"), func() { onResolve("cloud") })
		setEnabled(keepCloud, pl.HasRemote)
		c.Add(hbox(widget.NewButton(lang.L("Keep this device's saves"), func() { onResolve("local") }), keepCloud))
	}
	v := &planView{content: c}
	v.ExtendBaseWidget(v)
	return v
}

// CreateRenderer implements fyne.Widget.
func (v *planView) CreateRenderer() fyne.WidgetRenderer { return widget.NewSimpleRenderer(v.content) }
