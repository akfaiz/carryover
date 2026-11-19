package game

import (
	"slices"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/lang"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/akfaiz/carryover/internal/cloudsync/syncer"
	"github.com/akfaiz/carryover/services"
	"github.com/akfaiz/carryover/ui/components"
	"github.com/akfaiz/carryover/ui/env"
	"github.com/akfaiz/carryover/ui/kit"
)

// saveLocationRow shows one save location of a game on this device: its
// label and either the resolved folder (click to open), an error or a hint.
func saveLocationRow(u env.Env, label, resolved, pathErr string) fyne.CanvasObject {
	name := widget.NewLabelWithStyle(label, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	var val fyne.CanvasObject
	switch {
	case pathErr != "":
		val = kit.Colored(pathErr, widget.DangerImportance)
	case resolved != "":
		// One line, shortened in the middle: the full path in the accent
		// colour wrapped over three lines and outshouted the page.
		val = kit.NewTappable(container.NewBorder(nil, nil, widget.NewIcon(theme.FolderOpenIcon()), nil, kit.NewPathLabel(resolved)), func() { u.OpenFolder(resolved) })
	default:
		val = kit.Colored(lang.L("not set on this device"), widget.WarningImportance)
	}
	return kit.LabeledRow(110, name, val)
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
	kit.SetEnabled(restore, canRestore)
	pin := widget.NewButton(pinLabel, onPin)
	kit.SetEnabled(pin, canPin)
	del := widget.NewButtonWithIcon("", theme.DeleteIcon(), onDelete)
	kit.SetEnabled(del, canPin)
	if b.Local {
		return kit.LabeledRow(140, widget.NewLabel(components.FmtDate(b.Time)), kit.Muted(desc), del, restore)
	}
	return kit.LabeledRow(140, widget.NewLabel(components.FmtDate(b.Time)), kit.Muted(desc), pin, del, restore)
}

// otherDevicesCard lists the save paths other devices use for the game; nil
// when they have none to show (see otherDevicesLine).
func otherDevicesCard(g services.GameView) fyne.CanvasObject {
	if !slices.ContainsFunc(g.OtherDevices, func(d services.DevicePaths) bool {
		return slices.ContainsFunc(g.Slots, func(s services.SaveSlot) bool { return d.Paths[s.Key] != "" })
	}) {
		return nil
	}
	devs := kit.VBox()
	for _, d := range g.OtherDevices {
		devs.Add(widget.NewLabelWithStyle(d.DeviceName, fyne.TextAlignLeading, fyne.TextStyle{Bold: true}))
		for _, s := range g.Slots {
			if v := d.Paths[s.Key]; v != "" {
				l := kit.Mono(components.LabelOf(s.Label, s.Key) + ": " + v)
				l.Importance = widget.LowImportance
				l.SizeName = theme.SizeNameCaptionText
				devs.Add(l)
			}
		}
	}
	return kit.Card(lang.L("Other devices"), devs)
}

// otherDevicesLine names the other devices that have the game, for the page
// header when otherDevicesCard has no paths to list: a card holding only
// names was mostly empty.
func otherDevicesLine(g services.GameView) fyne.CanvasObject {
	names := make([]string, 0, len(g.OtherDevices))
	for _, d := range g.OtherDevices {
		names = append(names, d.DeviceName)
	}
	return kit.Small(lang.L("Also on {{.Devices}}", map[string]any{"Devices": strings.Join(names, ", ")}))
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
type planView struct{ kit.Composite }

// newPlanView shows pl. With resolvable, "Keep …" buttons call onResolve with
// "local" or "cloud".
func newPlanView(pl *services.PlanView, resolvable bool, onResolve func(resolution string)) *planView {
	if !resolvable {
		onResolve = nil
	}
	c := kit.VBox(
		widget.NewLabelWithStyle(planText(pl.Action), fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		components.SaveSides(pl.Local, pl.Remote, pl.HasRemote, onResolve),
	)
	v := &planView{}
	v.Init(v, c)
	return v
}
