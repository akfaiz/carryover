package emulators

import (
	"github.com/egoist/mygo/ui"

	"github.com/akfaiz/carryover/internal/saves/pathvar"
	"github.com/akfaiz/carryover/services"
	"github.com/akfaiz/carryover/ui/components"
	"github.com/akfaiz/carryover/ui/components/emulatorform"
	"github.com/akfaiz/carryover/ui/env"
	"github.com/akfaiz/carryover/ui/i18n"
	"github.com/akfaiz/carryover/ui/kit"
	"github.com/akfaiz/carryover/ui/store"
)

// Page is the Emulators page.
type Page struct {
	u       env.Env
	vm      *ViewModel
	editing *emulatorform.Form // nil: the list is shown
	scroll  ui.ScrollState
}

// New returns the Emulators page.
func New(u env.Env) *Page {
	p := &Page{u: u}
	p.vm = NewViewModel(Deps{
		Emulators: u.Emus(),
		Tokens:    func() []pathvar.Token { return u.Lib().Tokens("") },
		Run:       func(work func() error, done func(error)) { kit.Async0(work, done) },
		Changed:   kit.Redraw,
		Error:     u.ToastErr,
	})
	p.vm.Reload()
	return p
}

// Scroll is the page's scroll container, which the pad's right stick scrolls.
func (p *Page) Scroll() *ui.ScrollState { return &p.scroll }

// StateChanged reloads the list when the library changed under the page.
func (p *Page) StateChanged(what store.Topic) {
	if what == store.Library {
		p.vm.LibraryChanged()
	}
}

// Dirty: an emulator form with unsaved changes is open.
func (p *Page) Dirty() bool { return p.editing != nil && p.editing.Dirty() }

// View draws the page.
func (p *Page) View(c *ui.Context) {
	nav := p.u.Navigator()
	kit.Page(c, kit.PageForm, &p.scroll, func() {
		switch {
		case !p.vm.Loaded():
			kit.PageHead(c, nav, nil, "", i18n.L("Emulators"), nil)
			kit.Muted(c, i18n.L("Loading…"))
		case p.editing != nil:
			kit.PageHead(c, nav, p.closeForm, i18n.L("Back"), p.editing.Title(), nil)
			p.editing.View(c)
		default:
			p.list(c)
		}
	})
}

func (p *Page) list(c *ui.Context) {
	nav := p.u.Navigator()
	kit.PageHead(c, nav, nil, "", i18n.L("Emulators"), func() {
		kit.Btn(c, nav, "emu-add", kit.BtnPrimary, kit.IconPlus, i18n.L("Add emulator"), func() { p.edit(services.EmulatorSetup{}) })
	})
	intro := i18n.L("An emulator is set up once per device. Games you play through it pick it in their editor, and each emulator keeps its own saves in the cloud.")
	if len(p.vm.List()) == 0 {
		kit.Muted(c, intro)
		kit.Card(c, "", func() {
			kit.EmptyState(c, kit.IconGamepad, i18n.L("No emulators yet"), i18n.L("Add one, for example Eden or Ryujinx for Nintendo Switch games."), func() {
				kit.Btn(c, nav, "emu-add-first", kit.BtnPrimary, kit.IconPlus, i18n.L("Add emulator"), func() { p.edit(services.EmulatorSetup{}) })
			})
		})
		return
	}
	kit.Muted(c, intro)
	kit.Card(c, "", func() {
		for i, ev := range p.vm.List() {
			p.row(c, i == 0, ev)
		}
	})
}

// closeForm goes back to the list, asking first if the form has changes.
func (p *Page) closeForm() {
	toList := func() { p.editing = nil; p.vm.SetEditing(false); kit.Redraw() }
	if p.Dirty() {
		components.ConfirmDiscard(p.u, toList)
		return
	}
	toList()
}

func (p *Page) row(c *ui.Context, first bool, ev services.EmulatorView) {
	nav := p.u.Navigator()
	t := c.Theme()
	ui.Column(c).Gap(kit.S2).Children(func() {
		if !first {
			ui.Divider(c)
		}
		ui.Row(c).Gap(kit.S3).AlignItems(ui.Center).Children(func() {
			ui.Row(c).Width(40).Height(40).Radius(kit.Radius).Background(t.Accent.Alpha(0.14)).Center().Children(func() {
				ui.Icon(c, kit.IconGamepad).FontSize(kit.IconMD).TextColor(t.Accent)
			})
			ui.Column(c).Gap(2).Grow(1).Shrink(1).Children(func() {
				kit.ItemName(c, ev.Name)
				ui.Row(c).Gap(kit.S2).AlignItems(ui.Center).Children(func() {
					switch {
					case ev.NoVersionHere:
						// Nothing to set up here: it runs on other devices only.
						kit.Badge(c, components.OnlyOn(ev.Platforms), kit.BadgeInfo)
					case ev.SetUp:
						kit.Badge(c, i18n.L("Ready on this device"), kit.BadgeOK)
					default:
						kit.Badge(c, i18n.L("Not set up on this device"), kit.BadgeNone)
					}
					kit.Small(c, i18n.N("Used by {{.Count}} game", ev.Games, map[string]any{"Count": ev.Games}))
				})
			})
			if !ev.NoVersionHere { // its setup on this device is what Edit changes
				kit.Btn(c, nav, [3]any{"emu-row", ev.ID, "edit"}, kit.BtnSecondary, kit.IconPencil, i18n.L("Edit"), func() { p.edit(SetupOf(ev)) })
			}
			kit.IconBtn(c, nav, [3]any{"emu-row", ev.ID, "more"}, kit.BtnSecondary, kit.IconMore, i18n.L("More"), func() { p.more(ev) })
		})
	})
}

// more is the menu of an emulator's row: clear its setup on this device, or
// delete it.
func (p *Page) more(ev services.EmulatorView) {
	var items []components.MenuItem
	if ev.SetUp {
		items = append(items, components.MenuItem{Key: [2]any{ev.ID, "unset"}, Label: i18n.L("Remove from this device…"), Icon: kit.IconExit, Action: func() { p.askUnset(ev) }})
	}
	items = append(items, components.MenuItem{Key: [2]any{ev.ID, "delete"}, Label: i18n.L("Delete…"), Icon: kit.IconTrash, Danger: true, Action: func() { p.askDelete(ev) }})
	components.ActionMenu(p.u, ev.Name, items)
}

// askUnset clears this device's setup of an emulator, keeping it everywhere else.
func (p *Page) askUnset(ev services.EmulatorView) {
	components.ConfirmDanger(p.u, components.ConfirmOpts{
		Title:     i18n.L("Remove from this device"),
		Message:   i18n.L("Clear where {{.Name}} is installed on this device? It stays set up on your other devices, and games keep it and their cloud saves. Set it up again with Edit.", map[string]any{"Name": ev.Name}),
		Action:    i18n.L("Remove"),
		OnConfirm: func(bool) { p.vm.Unset(ev.ID) },
	})
}

// askDelete moves an emulator to the trash, or deletes it with its cloud saves.
func (p *Page) askDelete(ev services.EmulatorView) {
	components.ConfirmDanger(p.u, components.ConfirmOpts{
		Title:     i18n.L("Delete emulator"),
		Message:   i18n.L("Move this emulator to the trash on all devices? You can restore it from the Trash page. Games keep their saves in the cloud, but can't be launched with it until then."),
		Check:     i18n.L("Also remove it from games and delete its cloud saves"),
		Checked:   i18n.L("Delete this emulator permanently on all devices, remove it from every game and delete the cloud saves and backups made with it? It can't be restored."),
		KeptHint:  i18n.L("The cloud saves stay until you delete them. Once it is gone for good, they are listed under Trash → Leftover cloud saves."),
		OnConfirm: func(saves bool) { p.vm.Delete(ev.ID, saves) },
	})
}

func (p *Page) edit(s services.EmulatorSetup) {
	p.editing = emulatorform.New(p.u, s, p.vm.Tokens(), p.closeForm, func(services.EmulatorView) {
		p.editing = nil
		p.vm.SetEditing(false)
		p.vm.Reload()
	})
	p.vm.SetEditing(true)
	kit.Redraw()
}
