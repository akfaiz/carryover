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
	ui.Scroll(c).Grow(1).TrackScroll(&p.scroll).Children(func() {
		ui.Column(c).Gap(10).Padding(0, 20, 20, 20).Children(func() {
			switch {
			case !p.vm.Loaded():
				kit.PageHead(c, nav, nil, nil, i18n.L("Emulators"), nil)
				kit.Muted(c, i18n.L("Loading…"))
			case p.editing != nil:
				kit.PageHead(c, nav, nav.Handle("emu-back"), p.closeForm, p.editing.Title(), nil)
				p.editing.View(c)
			default:
				p.list(c)
			}
		})
	})
}

func (p *Page) list(c *ui.Context) {
	nav := p.u.Navigator()
	kit.PageHead(c, nav, nil, nil, i18n.L("Emulators"), func() {
		add := func() { p.edit(services.EmulatorSetup{}) }
		if nav.Item(nav.Handle("emu-add"), ui.PrimaryButton(c.Key("emu-add"), i18n.L("Add emulator")), add).Clicked() {
			add()
		}
	})
	kit.Muted(c, i18n.L("An emulator is set up once per device. Games you play through it pick it in their editor, and each emulator keeps its own saves in the cloud."))
	if len(p.vm.List()) == 0 {
		kit.Card(c, "", func() {
			kit.Muted(c, i18n.L("No emulators yet. Add one, for example Eden or Ryujinx for Nintendo Switch games."))
		})
	}
	for _, ev := range p.vm.List() {
		p.row(c, ev)
	}
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

func (p *Page) row(c *ui.Context, ev services.EmulatorView) {
	nav := p.u.Navigator()
	button := func(key, label string, danger bool, action func()) {
		k := [3]any{"emu-row", ev.ID, key}
		b := ui.Button(c.Key(k), label)
		if danger {
			b = b.TextColor(c.Theme().Danger)
		}
		if nav.Item(nav.Handle(k), b, action).Clicked() {
			action()
		}
	}
	kit.Card(c, "", func() {
		kit.CardHead(c, ev.Name, func() {
			if ev.SetUp {
				kit.Badge(c, i18n.L("Ready on this device"), kit.BadgeOK)
			} else {
				kit.Badge(c, i18n.L("Not set up on this device"), kit.BadgeNone)
			}
		})
		kit.Small(c, i18n.N("Used by {{.Count}} game", ev.Games, map[string]any{"Count": ev.Games}))
		ui.Row(c).Gap(8).Wrap().Children(func() {
			button("edit", i18n.L("Edit"), false, func() { p.edit(SetupOf(ev)) })
			if ev.SetUp {
				button("unset", i18n.L("Remove from this device…"), false, func() { p.askUnset(ev) })
			}
			button("delete", i18n.L("Delete…"), true, func() { p.askDelete(ev) })
		})
	})
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
