package emulators

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/lang"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/akfaiz/carryover/internal/saves/pathvar"
	"github.com/akfaiz/carryover/services"
	"github.com/akfaiz/carryover/ui/components"
	"github.com/akfaiz/carryover/ui/components/emulatorform"
	"github.com/akfaiz/carryover/ui/env"
	"github.com/akfaiz/carryover/ui/kit"
	"github.com/akfaiz/carryover/ui/store"
)

// Page is the Emulators page.
type Page struct {
	u       env.Env
	vm      *ViewModel
	editing *emulatorform.Form // nil: the list is shown
	scroll  *container.Scroll
}

// New returns the Emulators page.
func New(u env.Env) *Page {
	p := &Page{u: u}
	p.vm = NewViewModel(Deps{
		Emulators: u.Emus(),
		Tokens:    func() []pathvar.Token { return u.Lib().Tokens("") },
		Run:       func(work func() error, done func(error)) { kit.Async0(work, done) },
		Changed:   p.render,
		Error:     u.ToastErr,
	})
	return p
}

func (p *Page) View() fyne.CanvasObject {
	p.scroll = kit.ScrollPage(kit.Muted(lang.L("Loading…")))
	p.vm.Reload()
	return p.scroll
}

func (p *Page) StateChanged(what store.Topic) {
	if what == store.Library {
		p.vm.LibraryChanged()
	}
}

// Dirty: an emulator form with unsaved changes is open.
func (p *Page) Dirty() bool { return p.editing != nil && p.editing.Dirty() }

// Redraw draws the page again from its state (env.Redrawable).
func (p *Page) Redraw() { p.render() }

func (p *Page) render() {
	if !p.vm.Loaded() {
		return
	}
	if p.editing != nil {
		kit.ReplaceScroll(p.scroll, kit.Stack(kit.PageHead(p.closeForm, p.editing.Title()), p.editing.Host))
		return
	}
	add := widget.NewButtonWithIcon(lang.L("Add emulator"), theme.ContentAddIcon(), func() { p.edit(services.EmulatorSetup{}) })
	add.Importance = widget.HighImportance
	body := kit.Stack(kit.PageHead(nil, lang.L("Emulators"), add),
		kit.Muted(lang.L("An emulator is set up once per device. Games you play through it pick it in their editor, and each emulator keeps its own saves in the cloud.")))
	if len(p.vm.List()) == 0 {
		body.Add(kit.Card("", kit.Muted(lang.L("No emulators yet. Add one, for example Eden or Ryujinx for Nintendo Switch games."))))
	}
	for _, ev := range p.vm.List() {
		body.Add(p.row(ev))
	}
	kit.ReplaceScroll(p.scroll, body)
}

// closeForm goes back to the list, asking first if the form has changes.
func (p *Page) closeForm() {
	toList := func() { p.editing = nil; p.vm.SetEditing(false); p.render() }
	if p.Dirty() {
		components.ConfirmDiscard(p.u, toList)
		return
	}
	toList()
}

func (p *Page) row(ev services.EmulatorView) fyne.CanvasObject {
	state := kit.Badge(lang.L("Ready on this device"), kit.BadgeOK)
	if !ev.SetUp {
		state = kit.Badge(lang.L("Not set up on this device"), kit.BadgeNone)
	}
	games := kit.Small(lang.N("Used by {{.Count}} game", ev.Games, map[string]any{"Count": ev.Games}))
	edit := widget.NewButtonWithIcon(lang.L("Edit"), theme.DocumentCreateIcon(), func() { p.edit(SetupOf(ev)) })
	del := kit.DangerButton(lang.L("Delete…"), func() { p.askDelete(ev) })
	actions := kit.HBox(edit)
	if ev.SetUp {
		actions.Add(kit.LowButton(lang.L("Remove from this device…"), func() { p.askUnset(ev) }))
	}
	actions.Add(del)
	return kit.Card("", kit.VBox(kit.CardHead(ev.Name, state), kit.HBox(games), actions))
}

// askUnset clears this device's setup of an emulator, keeping it everywhere else.
func (p *Page) askUnset(ev services.EmulatorView) {
	components.ConfirmDanger(p.u, components.ConfirmOpts{
		Title:     lang.L("Remove from this device"),
		Message:   lang.L("Clear where {{.Name}} is installed on this device? It stays set up on your other devices, and games keep it and their cloud saves. Set it up again with Edit.", map[string]any{"Name": ev.Name}),
		Action:    lang.L("Remove"),
		OnConfirm: func(bool) { p.vm.Unset(ev.ID) },
	})
}

// askDelete moves an emulator to the trash, or deletes it with its cloud saves.
func (p *Page) askDelete(ev services.EmulatorView) {
	components.ConfirmDanger(p.u, components.ConfirmOpts{
		Title:     lang.L("Delete emulator"),
		Message:   lang.L("Move this emulator to the trash on all devices? You can restore it from the Trash page. Games keep their saves in the cloud, but can't be launched with it until then."),
		Check:     lang.L("Also remove it from games and delete its cloud saves"),
		Checked:   lang.L("Delete this emulator permanently on all devices, remove it from every game and delete the cloud saves and backups made with it? It can't be restored."),
		KeptHint:  lang.L("The cloud saves stay until you delete them. Once it is gone for good, they are listed under Trash → Leftover cloud saves."),
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
	p.editing.Relayout = p.render // the page is built again around the form
	p.render()
}
