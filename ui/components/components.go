// Package components holds widgets and dialogs that know about the app (help
// text, confirm dialogs, status badges, the save-sides chooser). This is the
// MyGo build: each function draws into the Context it is given, and keeps the
// same shape throughout, so a page reads the same everywhere.
package components

import (
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"github.com/akfaiz/carryover/internal/cloudsync/syncer"
	"github.com/akfaiz/carryover/services"
	"github.com/akfaiz/carryover/ui/env"
	"github.com/akfaiz/carryover/ui/i18n"
	"github.com/akfaiz/carryover/ui/kit"
)

// StatusBadge is the sync state badge of a game.
func StatusBadge(c *ui.Context, g services.GameView) ui.Element {
	text, kind := StatusOf(g)
	return kit.Badge(c, text, kind)
}

// Help is a short explanation with a small "More" button under it that
// unfolds the rest. Whether it is unfolded is remembered for the session (by
// its summary), so a page that is built again doesn't fold it.
func Help(c *ui.Context, u env.Env, summary string, details ...string) ui.Element {
	open := u.HelpOpen()
	return ui.Column(c).Gap(kit.S1).Children(func() {
		kit.Muted(c, summary)
		if len(details) == 0 {
			return
		}
		if open[summary] {
			for _, d := range details {
				kit.Muted(c, d)
			}
		}
		label, icon := i18n.L("More"), kit.IconChevronDown
		if open[summary] {
			label, icon = i18n.L("Less"), kit.IconChevronUp
		}
		kit.Btn(c, u.Navigator(), [2]any{"help", summary}, kit.BtnGhost, icon, label,
			func() { open[summary] = !open[summary] }, kit.BtnSmall()).AlignSelf(ui.Start).Margin(0, 0, 0, -kit.S2)
	})
}

// ConfirmDanger asks before something is deleted, in a dialog: the message, an
// optional checkbox to delete more, Cancel and a red action button.
func ConfirmDanger(u env.Env, o ConfirmOpts) {
	action := o.Action
	if action == "" {
		action = i18n.L("Delete")
	}
	u.Confirm(kit.ConfirmOptions{
		Title: o.Title, Body: o.Message, Cancel: i18n.L("Cancel"), OK: action, Danger: true,
		Check: o.Check, Checked: o.Checked, KeptHint: o.KeptHint, OnConfirm: o.OnConfirm,
	}, nil)
}

// ConfirmDiscard asks whether to leave a form with unsaved changes.
func ConfirmDiscard(u env.Env, leave func()) {
	u.Confirm(kit.ConfirmOptions{
		Title: i18n.L("Discard changes?"), Body: i18n.L("Your changes on this page haven't been saved."),
		Cancel: i18n.L("Keep editing"), OK: i18n.L("Discard changes"), Danger: true,
	}, leave)
}

// RemoveButton is a "Remove" button tinted red, so it doesn't read like the
// button next to it. key tells it from the others on the page.
func RemoveButton(c *ui.Context, u env.Env, key any, onRemove func()) ui.Element {
	return kit.Btn(c, u.Navigator(), [2]any{"remove", key}, kit.BtnDanger, kit.IconTrash, i18n.L("Remove"), onRemove)
}

// CopyField is a read-only text with a Copy button that tells it copied.
func CopyField(c *ui.Context, u env.Env, text string) ui.Element {
	return ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
		kit.Mono(c, text).Grow(1)
		kit.Btn(c, u.Navigator(), [2]any{"copy", text}, kit.BtnSecondary, nil, i18n.L("Copy"), func() {
			mygo.Clipboard.WriteText(text)
			u.Toast(i18n.L("Copied"), kit.ToastOK)
		})
	})
}

// SaveSides shows this device's saves and the cloud's side by side, the newer
// one marked. With onKeep, each side has a button that keeps it: onKeep gets
// "local" or "cloud". Used wherever a conflict is settled, so it reads the
// same in the play dialog and on the game page. The buttons register with nav
// (the gamepad), under keys prefixed with scope.
func SaveSides(c *ui.Context, nav *kit.Navigator, scope string, local, remote syncer.Summary, hasRemote bool, onKeep func(side string)) ui.Element {
	newer := ""
	if hasRemote {
		if local.Newest.After(remote.Newest) {
			newer = "local"
		} else if remote.Newest.After(local.Newest) {
			newer = "cloud"
		}
	}
	side := func(key, label, keepLabel string, s syncer.Summary, has bool) {
		kit.SubCard(c, func() {
			ui.Row(c).Gap(kit.S2).AlignItems(ui.Center).Children(func() {
				ui.Text(c, label).FontWeight(600)
				if newer == key {
					kit.Badge(c, i18n.L("Newer"), kit.BadgeOK)
				}
			})
			if s.DeviceName != "" && key == "cloud" {
				kit.Muted(c, i18n.L("from {{.Device}}", map[string]any{"Device": s.DeviceName}))
			}
			text := i18n.L("no Carryover saves for this game yet")
			if has {
				text = i18n.N("{{.Count}} file · {{.Size}}\nLast modified {{.Date}}", s.Files,
					map[string]any{"Count": s.Files, "Size": kit.FormatBytes(s.Bytes), "Date": FmtDate(s.Newest)})
			}
			ui.Text(c, text).Wrap()
			if onKeep != nil {
				kit.Btn(c, nav, [3]any{"keep", scope, key}, kit.BtnPrimary, nil, keepLabel, func() { onKeep(key) }, kit.BtnDisabled(!has))
			}
		}).Grow(1)
	}
	return ui.Row(c).Gap(kit.S3).AlignItems(ui.Stretch).Children(func() {
		side("local", i18n.L("This device"), i18n.L("Keep this device's saves"), local, true)
		side("cloud", i18n.L("Cloud"), i18n.L("Keep cloud saves"), remote, hasRemote)
	})
}
