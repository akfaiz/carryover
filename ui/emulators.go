package ui

import (
	"path/filepath"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/lang"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/akfaiz/carryover/internal/saves/pathvar"
	"github.com/akfaiz/carryover/internal/ui/kit"
	"github.com/akfaiz/carryover/services"
)

// pickFileDialog asks for a file and passes its path to done.
func pickFileDialog(u *UI, title string, exts []string, done func(string)) {
	d := dialog.NewFileOpen(func(r fyne.URIReadCloser, err error) {
		if err != nil || r == nil {
			return
		}
		p := filepath.FromSlash(r.URI().Path())
		_ = r.Close()
		done(p)
	}, u.win)
	d.SetTitleText(title)
	if len(exts) > 0 {
		d.SetFilter(storage.NewExtensionFileFilter(exts))
	}
	d.Resize(dialogSize(u.win))
	d.Show()
}

// emulatorsPage lists the emulators and edits them: where each is installed on
// this device and how it is started. A game then picks its emulators.
type emulatorsPage struct {
	u *UI

	list    []services.EmulatorView
	tokens  []pathvar.Token
	loaded  bool
	editing *emulatorForm // nil: the list is shown

	scroll *container.Scroll
}

func newEmulatorsPage(u *UI) *emulatorsPage { return &emulatorsPage{u: u} }

func (p *emulatorsPage) view() fyne.CanvasObject {
	p.scroll = kit.ScrollPage(kit.Muted(lang.L("Loading…")))
	p.reload()
	return p.scroll
}

func (p *emulatorsPage) reload() {
	type loaded struct {
		list   []services.EmulatorView
		tokens []pathvar.Token
	}
	kit.Async(func() (loaded, error) { return loaded{p.u.emus.List(), p.u.lib.Tokens("")}, nil }, func(l loaded, _ error) {
		p.list, p.tokens, p.loaded = l.list, l.tokens, true
		p.render()
	})
}

func (p *emulatorsPage) stateChanged(what change) {
	if what == changeLibrary && p.editing == nil {
		p.reload()
	}
}

// dirty: an emulator form with unsaved changes is open.
func (p *emulatorsPage) dirty() bool { return p.editing != nil && p.editing.dirty() }

func (p *emulatorsPage) render() {
	if !p.loaded {
		return
	}
	if p.editing != nil {
		kit.ReplaceScroll(p.scroll, kit.Stack(kit.PageHead(p.closeForm, p.editing.title()), p.editing.host))
		return
	}
	add := widget.NewButtonWithIcon(lang.L("Add emulator"), theme.ContentAddIcon(), func() { p.edit(services.EmulatorSetup{}) })
	add.Importance = widget.HighImportance
	body := kit.Stack(kit.PageHead(nil, lang.L("Emulators"), add),
		kit.Muted(lang.L("An emulator is set up once per device. Games you play through it pick it in their editor, and each emulator keeps its own saves in the cloud.")))
	if len(p.list) == 0 {
		body.Add(kit.Card("", kit.Muted(lang.L("No emulators yet. Add one, for example Eden or Ryujinx for Nintendo Switch games."))))
	}
	for _, ev := range p.list {
		body.Add(p.row(ev))
	}
	kit.ReplaceScroll(p.scroll, body)
}

// closeForm goes back to the list, asking first if the form has changes.
func (p *emulatorsPage) closeForm() {
	toList := func() { p.editing = nil; p.render() }
	if p.dirty() {
		confirmDiscard(p.u, toList)
		return
	}
	toList()
}

func (p *emulatorsPage) row(ev services.EmulatorView) fyne.CanvasObject {
	state := kit.Badge(lang.L("Ready on this device"), kit.BadgeOK)
	if !ev.SetUp {
		state = kit.Badge(lang.L("Not set up on this device"), kit.BadgeNone)
	}
	games := kit.Small(lang.N("Used by {{.Count}} game", ev.Games, map[string]any{"Count": ev.Games}))
	edit := widget.NewButtonWithIcon(lang.L("Edit"), theme.DocumentCreateIcon(), func() {
		p.edit(services.EmulatorSetup{
			ID: ev.ID, Name: ev.Name, Exe: ev.Device.Exe, Args: ev.Device.Args, WorkDir: ev.Device.WorkDir,
			Env: ev.Device.Env, WaitProcess: ev.Device.WaitProcess, Preset: ev.Preset, DataDir: ev.Device.DataDir,
		})
	})
	del := kit.DangerButton(lang.L("Delete…"), func() { p.askDelete(ev) })
	return kit.Card("", kit.VBox(kit.CardHead(ev.Name, state), kit.HBox(games), kit.HBox(edit, del)))
}

// askDelete moves an emulator to the trash, or deletes it with its cloud saves.
func (p *emulatorsPage) askDelete(ev services.EmulatorView) {
	confirmDanger(p.u, confirmOpts{
		title:    lang.L("Delete emulator"),
		message:  lang.L("Move this emulator to the trash on all devices? You can restore it from the Trash page. Games keep their saves in the cloud, but can't be launched with it until then."),
		check:    lang.L("Also remove it from games and delete its cloud saves"),
		checked:  lang.L("Delete this emulator permanently on all devices, remove it from every game and delete the cloud saves and backups made with it? It can't be restored."),
		keptHint: lang.L("The cloud saves stay until you delete them. Once it is gone for good, they are listed under Trash → Leftover cloud saves."),
		onConfirm: func(saves bool) {
			del := p.u.emus.Delete
			if saves {
				del = p.u.emus.DeleteWithSaves
			}
			kit.Async0(func() error { return del(ev.ID) }, func(err error) {
				p.u.toastErr(err)
				p.reload()
			})
		},
	})
}

func (p *emulatorsPage) edit(s services.EmulatorSetup) {
	p.editing = newEmulatorForm(p.u, s, p.tokens, p.closeForm, func(services.EmulatorView) {
		p.editing = nil
		p.reload()
	})
	p.editing.relayout = p.render // the page is built again around the form
	p.render()
}
