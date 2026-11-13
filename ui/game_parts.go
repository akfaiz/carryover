package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/lang"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/akfaiz/carryover/internal/cloudsync/syncer"
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
	if !resolvable {
		onResolve = nil
	}
	c := vbox(
		widget.NewLabelWithStyle(planText(pl.Action), fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		saveSides(pl.Local, pl.Remote, pl.HasRemote, onResolve),
	)
	v := &planView{content: c}
	v.ExtendBaseWidget(v)
	return v
}

// saveSides shows this device's saves and the cloud's side by side, the newer
// one marked. With onKeep, each side has a button that keeps it: onKeep gets
// "local" or "cloud". Used wherever a conflict is settled, so it reads the
// same in the play dialog and on the game page.
func saveSides(local, remote syncer.Summary, hasRemote bool, onKeep func(side string)) fyne.CanvasObject {
	newer := ""
	if hasRemote {
		if local.Newest.After(remote.Newest) {
			newer = "local"
		} else if remote.Newest.After(local.Newest) {
			newer = "cloud"
		}
	}
	side := func(key, label, keepLabel string, s syncer.Summary, has bool) fyne.CanvasObject {
		head := hbox(widget.NewLabelWithStyle(label, fyne.TextAlignLeading, fyne.TextStyle{Bold: true}))
		if newer == key {
			head.Add(container.NewCenter(badge(lang.L("newer"), badgeOK)))
		}
		objs := []fyne.CanvasObject{head}
		if s.DeviceName != "" && key == "cloud" {
			objs = append(objs, muted(lang.L("from {{.Device}}", map[string]any{"Device": s.DeviceName})))
		}
		text := lang.L("no Carryover saves for this game yet")
		if has {
			text = lang.N("{{.Count}} file · {{.Size}}\nLast modified {{.Date}}", s.Files,
				map[string]any{"Count": s.Files, "Size": formatBytes(s.Bytes), "Date": fmtDate(s.Newest)})
		}
		info := widget.NewLabel(text)
		info.Wrapping = fyne.TextWrapWord
		objs = append(objs, info)
		if onKeep != nil {
			keep := primary(keepLabel, func() { onKeep(key) })
			setEnabled(keep, has)
			objs = append(objs, keep)
		}
		return card("", objs...)
	}
	return container.NewGridWithColumns(2,
		side("local", lang.L("This device"), lang.L("Keep this device's saves"), local, true),
		side("cloud", lang.L("Cloud"), lang.L("Keep cloud saves"), remote, hasRemote))
}

// CreateRenderer implements fyne.Widget.
func (v *planView) CreateRenderer() fyne.WidgetRenderer { return widget.NewSimpleRenderer(v.content) }
