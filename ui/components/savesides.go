package components

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/lang"
	"fyne.io/fyne/v2/widget"

	"github.com/akfaiz/carryover/internal/cloudsync/syncer"
	"github.com/akfaiz/carryover/ui/kit"
)

// SaveSides shows this device's saves and the cloud's side by side, the newer
// one marked. With onKeep, each side has a button that keeps it: onKeep gets
// "local" or "cloud". Used wherever a conflict is settled, so it reads the
// same in the play dialog and on the game page.
func SaveSides(local, remote syncer.Summary, hasRemote bool, onKeep func(side string)) fyne.CanvasObject {
	return SaveSidesNav(nil, local, remote, hasRemote, onKeep)
}

// SaveSidesNav is saveSides whose buttons belong to nav (a dialog driven with
// the keyboard and a gamepad); with a nil nav they are plain buttons.
func SaveSidesNav(nav *kit.Nav, local, remote syncer.Summary, hasRemote bool, onKeep func(side string)) fyne.CanvasObject {
	newer := ""
	if hasRemote {
		if local.Newest.After(remote.Newest) {
			newer = "local"
		} else if remote.Newest.After(local.Newest) {
			newer = "cloud"
		}
	}
	side := func(key, label, keepLabel string, s syncer.Summary, has bool) fyne.CanvasObject {
		head := kit.HBox(widget.NewLabelWithStyle(label, fyne.TextAlignLeading, fyne.TextStyle{Bold: true}))
		if newer == key {
			head.Add(container.NewCenter(kit.Badge(lang.L("Newer"), kit.BadgeOK)))
		}
		objs := []fyne.CanvasObject{head}
		if s.DeviceName != "" && key == "cloud" {
			objs = append(objs, kit.Muted(lang.L("from {{.Device}}", map[string]any{"Device": s.DeviceName})))
		}
		text := lang.L("no Carryover saves for this game yet")
		if has {
			text = lang.N("{{.Count}} file · {{.Size}}\nLast modified {{.Date}}", s.Files,
				map[string]any{"Count": s.Files, "Size": kit.FormatBytes(s.Bytes), "Date": FmtDate(s.Newest)})
		}
		info := widget.NewLabel(text)
		info.Wrapping = fyne.TextWrapWord
		objs = append(objs, info)
		if onKeep != nil {
			var keep interface {
				fyne.CanvasObject
				fyne.Disableable
			}
			if nav != nil {
				keep = nav.Button(keepLabel, widget.HighImportance, func() { onKeep(key) })
			} else {
				keep = kit.Primary(keepLabel, func() { onKeep(key) })
			}
			kit.SetEnabled(keep, has)
			objs = append(objs, keep)
		}
		return kit.Card("", objs...)
	}
	return container.NewGridWithColumns(2,
		side("local", lang.L("This device"), lang.L("Keep this device's saves"), local, true),
		side("cloud", lang.L("Cloud"), lang.L("Keep cloud saves"), remote, hasRemote))
}
