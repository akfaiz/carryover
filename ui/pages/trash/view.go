package trash

import (
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/akfaiz/carryover/ui/i18n"

	"github.com/akfaiz/carryover/services"
	"github.com/akfaiz/carryover/ui/components"
	"github.com/akfaiz/carryover/ui/env"
	"github.com/akfaiz/carryover/ui/kit"
	"github.com/akfaiz/carryover/ui/store"
)

// Page is the Trash page.
type Page struct {
	u      env.Env
	vm     *ViewModel
	scroll *container.Scroll
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
		Changed:   p.render,
		Count: func(n int) {
			u.Store().SetTrashCount(n)
			u.RefreshSidebar()
		},
		Error: u.ToastErr,
	})
	return p
}

func (p *Page) View() fyne.CanvasObject {
	p.scroll = kit.ScrollPage(kit.Muted(i18n.L("Loading…")))
	p.vm.Open()
	return p.scroll
}

func (p *Page) StateChanged(what store.Topic) {
	if what == store.Library {
		p.vm.LibraryChanged()
	}
}

// Redraw draws the page again from its state (env.Redrawable).
func (p *Page) Redraw() { p.render() }

func (p *Page) render() {
	if !p.vm.Loaded() {
		return
	}
	body := kit.Stack(kit.PageHead(nil, i18n.L("Trash")),
		kit.Muted(i18n.L("Deleted games and emulators stay here for {{.Days}} days and can be restored with all their settings. Their cloud saves are not touched unless you delete them too.",
			map[string]any{"Days": services.TrashDays})))

	restored := func() { p.u.Toast(i18n.L("Restored"), kit.ToastOK) }
	games := kit.Stack()
	for _, g := range p.vm.Games() {
		games.Add(p.row(g.Name, g.DeletedAt, g.ExpiresAt,
			func() { p.vm.RestoreGame(g.ID, restored) },
			i18n.L("Also delete its cloud saves and backups"),
			func(saves bool) { p.vm.DeleteGame(g.ID, saves) }))
	}
	body.Add(p.section(i18n.L("Games"), i18n.L("No deleted games."), games, len(p.vm.Games())))

	emus := kit.Stack()
	for _, e := range p.vm.Emulators() {
		emus.Add(p.row(e.Name, e.DeletedAt, e.ExpiresAt,
			func() { p.vm.RestoreEmulator(e.ID, restored) },
			i18n.L("Also delete the cloud saves made with it"),
			func(saves bool) { p.vm.DeleteEmulator(e.ID, saves) }))
	}
	body.Add(p.section(i18n.L("Emulators"), i18n.L("No deleted emulators."), emus, len(p.vm.Emulators())))
	body.Add(p.leftovers())
	kit.ReplaceScroll(p.scroll, body)
}

// section is a card with a heading and either its rows or an empty message.
func (p *Page) section(title, empty string, rows *fyne.Container, n int) fyne.CanvasObject {
	if n == 0 {
		return kit.Card(title, kit.Muted(empty))
	}
	return kit.Card(title, rows)
}

// row is a restorable item: Restore, and Delete permanently with an optional
// deletion of its cloud saves.
func (p *Page) row(name string, deleted, expires time.Time, restore func(), checkLabel string, del func(saves bool)) fyne.CanvasObject {
	sub := kit.Small(i18n.L("Deleted {{.When}} · settings kept until {{.Until}}", map[string]any{"When": components.FmtDate(deleted), "Until": components.FmtDate(expires)}))
	restoreBtn := widget.NewButtonWithIcon(i18n.L("Restore"), theme.HistoryIcon(), restore)
	deleteBtn := kit.DangerButton(i18n.L("Delete permanently…"), func() {
		components.ConfirmDanger(p.u, components.ConfirmOpts{
			Title:     i18n.L("Delete permanently"),
			Message:   i18n.L("Delete {{.Name}} permanently? It can't be restored any more. Its cloud saves and backups are kept.", map[string]any{"Name": name}),
			Check:     checkLabel,
			Checked:   i18n.L("Delete {{.Name}} permanently together with its cloud saves and backups? This cannot be undone.", map[string]any{"Name": name}),
			KeptHint:  i18n.L("The cloud saves stay and are listed under Leftover cloud saves below until you delete them."),
			OnConfirm: del,
		})
	})
	kit.SetEnabled(restoreBtn, !p.vm.Busy())
	kit.SetEnabled(deleteBtn, !p.vm.Busy())
	return kit.VBox(kit.CardHead(name, restoreBtn, deleteBtn), sub)
}

// leftovers is the card of cloud save folders no game uses.
func (p *Page) leftovers() fyne.CanvasObject {
	title := i18n.L("Leftover cloud saves")
	intro := kit.Muted(i18n.L("Saves and backups in your cloud that no game in the library uses, for example after a game was deleted for good or an emulator was removed from it. Restore one to add it back to the library, or delete it to free the space."))
	switch p.vm.LeftoverState() {
	case LeftoverNoRemote:
		return kit.Card(title, intro, kit.Muted(i18n.L("Set up cloud storage to look for them.")))
	case LeftoverLoading:
		return kit.Card(title, intro, kit.Muted(i18n.L("Looking in the cloud…")))
	case LeftoverFailed:
		return kit.Card(title, intro, kit.Colored(p.vm.LeftoverError(), widget.DangerImportance))
	case LeftoverNone:
		return kit.Card(title, intro, kit.Muted(i18n.L("Nothing left over.")))
	}
	rows := kit.Stack(intro)
	for _, l := range p.vm.Leftovers() {
		rows.Add(p.leftoverRow(l))
	}
	return kit.Card(i18n.L("Leftover cloud saves ({{.Count}})", map[string]any{"Count": len(p.vm.Leftovers())}), rows)
}

func (p *Page) leftoverRow(l services.LeftoverView) fyne.CanvasObject {
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
	sub := kit.Small(i18n.L("{{.Count}} files, {{.Size}} · last synced {{.When}} · {{.Backups}} backups",
		map[string]any{"Count": l.Files, "Size": kit.FormatBytes(l.Bytes), "When": components.FmtDate(l.Time), "Backups": l.Backups}))
	restoreBtn := widget.NewButtonWithIcon(i18n.L("Restore"), theme.HistoryIcon(), func() {
		p.vm.RestoreLeftover(l.Key, func(id string) {
			p.u.Toast(i18n.L("Restored. Set where its saves are on this device, then sync."), kit.ToastOK)
			p.u.Navigate("/edit/" + id)
		})
	})
	deleteBtn := kit.DangerButton(i18n.L("Delete…"), func() {
		components.ConfirmDanger(p.u, components.ConfirmOpts{
			Title:     i18n.L("Delete cloud saves"),
			Message:   i18n.L("Permanently delete the cloud saves and backups of {{.Name}}? This cannot be undone.", map[string]any{"Name": name}),
			OnConfirm: func(bool) { p.vm.DeleteLeftover(l.Key) },
		})
	})
	kit.SetEnabled(restoreBtn, !p.vm.Busy())
	kit.SetEnabled(deleteBtn, !p.vm.Busy())
	return kit.VBox(kit.CardHead(name, restoreBtn, deleteBtn), sub)
}
