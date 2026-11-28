package trash

import (
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/akfaiz/carryover/services"
	"github.com/akfaiz/carryover/ui/components"
	"github.com/akfaiz/carryover/ui/env"
	"github.com/akfaiz/carryover/ui/i18n"
	"github.com/akfaiz/carryover/ui/kit"
	"github.com/akfaiz/carryover/ui/store"
)

// Page is the Trash page.
type Page struct {
	u      env.Env
	vm     *ViewModel
	scroll ui.ScrollState
}

// New returns the Trash page.
func New(u env.Env) *Page {
	p := &Page{u: u}
	p.vm = NewViewModel(Deps{
		Games:     u.Lib(),
		Emulators: u.Emus(),
		Leftovers: u.Lib(),
		HasRemote: func() bool { return u.Store().Settings.Remote != "" },
		Run:       func(work func() error, done func(error)) { kit.Async0(work, done) },
		Changed:   kit.Redraw,
		Count:     func(n int) { u.Store().SetTrashCount(n); kit.Redraw() },
		Error:     u.ToastErr,
	})
	p.vm.Open()
	return p
}

// Scroll is the page's scroll container, which the pad's right stick scrolls.
func (p *Page) Scroll() *ui.ScrollState { return &p.scroll }

// StateChanged reloads the lists when the library changed under the page.
func (p *Page) StateChanged(what store.Topic) {
	if what == store.Library {
		p.vm.LibraryChanged()
	}
}

// View draws the page.
func (p *Page) View(c *ui.Context) {
	kit.Page(c, kit.PageForm, &p.scroll, func() {
		kit.PageHead(c, p.u.Navigator(), nil, "", i18n.L("Trash"), nil)
		if !p.vm.Loaded() {
			return // read from disk at once: a "Loading…" would only flash
		}
		retention := i18n.L("Deleted games and emulators stay here for {{.Days}} days and can be restored with all their settings. Their cloud saves are not touched unless you delete them too.",
			map[string]any{"Days": services.TrashDays})
		games, emus := p.vm.Games(), p.vm.Emulators()
		ls := p.vm.LeftoverState()
		if len(games) == 0 && len(emus) == 0 && (ls == LeftoverNone || ls == LeftoverNoRemote || ls == LeftoverLoading) {
			// Nothing at all: one empty state, not a card per kind. It shows at
			// once while the cloud is still searched for leftovers (most often
			// there are none), so the page doesn't swap a card for it after.
			kit.Card(c, "", func() {
				kit.EmptyState(c, kit.IconTrash, i18n.L("Trash is empty"), retention, nil)
				if ls == LeftoverLoading {
					kit.BusyLine(c, i18n.L("Looking in the cloud…")).AlignSelf(ui.Center)
				}
			})
			return
		}
		kit.Muted(c, retention)

		// A card per kind that has something in it.
		restored := func() { p.u.Toast(i18n.L("Restored"), kit.ToastOK) }
		if len(games) > 0 {
			kit.Card(c, i18n.L("Games"), func() {
				for i, g := range games {
					p.row(c, i == 0, "game:"+g.ID, g.Name, g.DeletedAt, g.ExpiresAt,
						func() { p.vm.RestoreGame(g.ID, restored) },
						i18n.L("Also delete its cloud saves and backups"),
						func(saves bool) { p.vm.DeleteGame(g.ID, saves) })
				}
			})
		}
		if len(emus) > 0 {
			kit.Card(c, i18n.L("Emulators"), func() {
				for i, e := range emus {
					p.row(c, i == 0, "emu:"+e.ID, e.Name, e.DeletedAt, e.ExpiresAt,
						func() { p.vm.RestoreEmulator(e.ID, restored) },
						i18n.L("Also delete the cloud saves made with it"),
						func(saves bool) { p.vm.DeleteEmulator(e.ID, saves) })
				}
			})
		}
		p.leftovers(c)
	})
}

// button draws a button that registers with the gamepad, disabled while the
// page is busy.
func (p *Page) button(c *ui.Context, key any, kind kit.BtnKind, icon *ui.SVG, label string, action func()) {
	kit.Btn(c, p.u.Navigator(), [2]any{"trash", key}, kind, icon, label, action, kit.BtnDisabled(p.vm.Busy()))
}

// row is a restorable item: Restore, and Delete permanently with an optional
// deletion of its cloud saves.
func (p *Page) row(c *ui.Context, first bool, key, name string, deleted, expires time.Time, restore func(), checkLabel string, del func(saves bool)) {
	kit.ListRow(c, first, name, func() {
		kit.Small(c, i18n.L("Deleted {{.When}} · settings kept until {{.Until}}", map[string]any{"When": components.FmtDate(deleted), "Until": components.FmtDate(expires)}))
	}, func() {
		p.button(c, key+"/restore", kit.BtnSecondary, kit.IconRestore, i18n.L("Restore"), restore)
		p.button(c, key+"/delete", kit.BtnDanger, kit.IconTrash, i18n.L("Delete permanently…"), func() {
			components.ConfirmDanger(p.u, components.ConfirmOpts{
				Title:     i18n.L("Delete permanently"),
				Message:   i18n.L("Delete {{.Name}} permanently? It can't be restored any more. Its cloud saves and backups are kept.", map[string]any{"Name": name}),
				Check:     checkLabel,
				Checked:   i18n.L("Delete {{.Name}} permanently together with its cloud saves and backups? This cannot be undone.", map[string]any{"Name": name}),
				KeptHint:  i18n.L("The cloud saves stay and are listed under Leftover cloud saves below until you delete them."),
				OnConfirm: del,
			})
		})
	})
}

// leftovers is the card of cloud save folders no game uses.
func (p *Page) leftovers(c *ui.Context) {
	title := i18n.L("Leftover cloud saves")
	intro := func() {
		components.Help(c, p.u, i18n.L("Cloud saves that no game in your library uses."),
			i18n.L("Saves and backups in your cloud that no game in the library uses, for example after a game was deleted for good or an emulator was removed from it. Restore one to add it back to the library, or delete it to free the space."))
	}
	switch p.vm.LeftoverState() {
	case LeftoverNoRemote:
		kit.Card(c, title, func() { intro(); kit.Muted(c, i18n.L("Set up cloud storage to look for them.")) })
	case LeftoverLoading:
		kit.Card(c, title, func() { intro(); kit.Muted(c, i18n.L("Looking in the cloud…")) })
	case LeftoverFailed:
		kit.Card(c, title, func() { intro(); kit.Danger(c, i18n.Text(p.vm.LeftoverError(), p.vm.LeftoverErrorMsg())) })
	case LeftoverNone:
		kit.Card(c, title, func() { intro(); kit.Muted(c, i18n.L("Nothing left over.")) })
	default:
		kit.Card(c, i18n.L("Leftover cloud saves ({{.Count}})", map[string]any{"Count": len(p.vm.Leftovers())}), func() {
			intro()
			for i, l := range p.vm.Leftovers() {
				p.leftoverRow(c, i == 0, l)
			}
		})
	}
}

func (p *Page) leftoverRow(c *ui.Context, first bool, l services.LeftoverView) {
	name := l.GameName
	if name == "" {
		name = i18n.L("Unknown game")
	}
	if l.EmulatorID != "" {
		emu := l.EmulatorName
		if emu == "" {
			emu = i18n.L("Unknown emulator")
		}
		name += " · " + emu
	}
	kit.ListRow(c, first, name, func() {
		kit.Small(c, i18n.L("{{.Count}} files, {{.Size}} · last synced {{.When}} · {{.Backups}} backups",
			map[string]any{"Count": l.Files, "Size": kit.FormatBytes(l.Bytes), "When": components.FmtDate(l.Time), "Backups": l.Backups}))
	}, func() {
		p.button(c, "left:"+l.Key+"/restore", kit.BtnSecondary, kit.IconRestore, i18n.L("Restore"), func() {
			p.vm.RestoreLeftover(l.Key, func(id string) {
				p.u.Toast(i18n.L("Restored. Set where its saves are on this device, then sync."), kit.ToastOK)
				p.u.Navigate("/edit/" + id)
			})
		})
		p.button(c, "left:"+l.Key+"/delete", kit.BtnDanger, kit.IconTrash, i18n.L("Delete…"), func() {
			components.ConfirmDanger(p.u, components.ConfirmOpts{
				Title:     i18n.L("Delete cloud saves"),
				Message:   i18n.L("Permanently delete the cloud saves and backups of {{.Name}}? This cannot be undone.", map[string]any{"Name": name}),
				OnConfirm: func(bool) { p.vm.DeleteLeftover(l.Key) },
			})
		})
	})
}
