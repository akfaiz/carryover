package cloud

import (
	"fmt"
	"path/filepath"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/lang"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/widget"

	"github.com/akfaiz/carryover/internal/app/appdir"
	"github.com/akfaiz/carryover/internal/cloudsync/cloud"
	"github.com/akfaiz/carryover/ui/components"
	"github.com/akfaiz/carryover/ui/env"
	"github.com/akfaiz/carryover/ui/kit"
	"github.com/akfaiz/carryover/ui/store"
)

// Page is the Cloud storage page.
type Page struct {
	u      env.Env
	vm     *ViewModel
	test   *kit.TestButton
	scroll *container.Scroll
}

// New returns the Cloud storage page.
func New(u env.Env) *Page {
	p := &Page{u: u}
	p.vm = NewViewModel(Deps{
		Cloud:    u.Cloud(),
		Settings: u.Sync(),
		Current:  func() appdir.Settings { return u.Store().Settings },
		Run:      func(work func() error, done func(error)) { kit.Async0(work, done) },
		Changed:  p.render,
		Refresh:  u.Store().Refresh,
		Error:    u.ToastErr,
		Saved:    func() { u.Toast(lang.L("Cloud location saved"), kit.ToastOK) },
		SyncFailed: func(err error) {
			u.Toast(lang.L("Library sync: {{.Err}}", map[string]any{"Err": err}), kit.ToastErr)
		},
		Imported: p.reportImport,
	})
	return p
}

func (p *Page) View() fyne.CanvasObject {
	p.scroll = kit.ScrollPage(kit.Muted(lang.L("Loading…")))
	p.test = kit.NewTestButton(lang.L("Test connection"), lang.L("Connection works."), lang.L("Testing…"), func() func() error {
		remote, root := p.vm.Remote(), p.vm.Root()
		return func() error { return p.u.Cloud().Test(remote, root) }
	})
	p.vm.Open()
	return p.scroll
}

// StateChanged re-renders when settings reload (e.g. after Save), so the
// "active" badge follows the saved remote.
func (p *Page) StateChanged(what store.Topic) {
	if what == store.Library && p.scroll != nil {
		p.render()
	}
}

// Redraw draws the page again from its state (env.Redrawable).
func (p *Page) Redraw() { p.render() }

// reportImport tells the user what an import found.
func (p *Page) reportImport(res cloud.ImportResult) {
	u := p.u
	if len(res.Imported) > 0 {
		u.Toast(lang.L("Imported {{.Names}}", map[string]any{"Names": strings.Join(res.Imported, ", ")}), kit.ToastOK)
	}
	if len(res.Skipped) > 0 {
		u.Toast(lang.L("Skipped unsupported: {{.Names}}", map[string]any{"Names": strings.Join(res.Skipped, ", ")}), kit.ToastErr)
	} else if len(res.Imported) == 0 {
		u.Toast(lang.L("No remotes found in that file"), kit.ToastErr)
	}
}

func (p *Page) render() {
	u := p.u
	vm := p.vm
	remotes := &kit.Choice{}
	remotes.Add("", lang.L("— none (sync disabled) —"))
	for _, r := range vm.Remotes() {
		l := fmt.Sprintf("%s (%s)", r.Name, r.Type)
		if !r.Supported {
			l = lang.L("{{.Name}} ({{.Type}}, unsupported)", map[string]any{"Name": r.Name, "Type": r.Type})
		}
		remotes.Add(r.Name, l)
	}
	p.test.SetAllowed(vm.Remote() != "")
	sel := kit.SelectOf(remotes, vm.Remote(), func(v string) {
		if vm.Unsupported(v) {
			u.Toast(lang.L("This provider isn't included in Carryover"), kit.ToastErr)
		}
		vm.SetRemote(v)
		p.test.Reset()
		p.test.SetAllowed(v != "")
	})
	root := kit.MonoEntry("", vm.Root(), vm.SetRoot)

	location := kit.Card(lang.L("Sync location"),
		kit.Muted(lang.L("All devices must use the same remote storage and folder. Saves go to <folder>/games/<game>/.")),
		container.NewGridWithColumns(2, kit.Field(lang.L("Remote"), kit.Outlined(sel)), kit.Field(lang.L("Folder"), root)),
		kit.HBox(kit.Primary(lang.L("Use this location"), vm.SaveTarget), p.test.Button),
		p.test.Result,
	)

	actions := []fyne.CanvasObject{}
	if vm.DefaultConfig() != "" {
		actions = append(actions, widget.NewButton(lang.L("Import from rclone"), func() { vm.ImportConf(vm.DefaultConfig()) }))
	}
	actions = append(actions,
		widget.NewButton(lang.L("Import rclone.conf…"), p.pickConf),
		kit.Primary(lang.L("＋ Add remote"), p.addRemote),
	)
	list := kit.VBox(kit.CardHead(lang.L("Remotes"), actions...))
	if len(vm.Remotes()) == 0 {
		list.Add(kit.Muted(lang.L("No remotes yet. Add one (Google Drive, Dropbox, OneDrive, WebDAV/Nextcloud, SFTP or SMB) or import your existing rclone config.")))
	}
	for _, r := range vm.Remotes() {
		list.Add(widget.NewSeparator())
		list.Add(newRemoteRow(r, vm.IsActive(r.Name), func() {
			msg := lang.L("Delete remote “{{.Name}}”? The files in it are not touched.", map[string]any{"Name": r.Name})
			if vm.IsActive(r.Name) {
				msg = lang.L("Delete remote “{{.Name}}”? It is the sync location, so syncing stops until you choose another one. The files in it are not touched.", map[string]any{"Name": r.Name})
			}
			components.ConfirmDanger(u, components.ConfirmOpts{Title: lang.L("Delete remote"), Message: msg, OnConfirm: func(bool) {
				vm.DeleteRemote(r.Name)
			}})
		}))
	}
	if !vm.HasRemote() {
		// First run: the remote comes first; the first one added becomes the
		// sync location by itself.
		intro := kit.Muted(lang.L("Carryover keeps your saves in your own cloud storage. Add a remote (or import your rclone config) and it becomes the sync location; every device then uses the same one."))
		kit.ReplaceScroll(p.scroll, kit.Stack(kit.PageHead(nil, lang.L("Cloud storage")), intro, kit.Card("", list), location))
		return
	}
	kit.ReplaceScroll(p.scroll, kit.Stack(kit.PageHead(nil, lang.L("Cloud storage")), location, kit.Card("", list)))
}

func (p *Page) pickConf() {
	d := dialog.NewFileOpen(func(r fyne.URIReadCloser, err error) {
		if err != nil || r == nil {
			return
		}
		path := filepath.FromSlash(r.URI().Path())
		_ = r.Close()
		p.vm.ImportConf(path)
	}, p.u.Window())
	d.SetTitleText(lang.L("Select rclone.conf"))
	d.SetFilter(storage.NewExtensionFileFilter([]string{".conf"}))
	kit.ShowFilePicker(p.u.Window(), d)
}

// ImportConf imports the remotes of an rclone config file, as the page's
// import buttons do.
func (p *Page) ImportConf(path string) { p.vm.ImportConf(path) }

// UseLocation chooses a remote and stores it as the sync location, as the
// page's "Use this location" button does.
func (p *Page) UseLocation(remote string) {
	p.vm.SetRemote(remote)
	p.vm.SaveTarget()
}

// AddRemote opens the wizard that adds a remote.
func (p *Page) AddRemote() { p.addRemote() }

func (p *Page) addRemote() {
	exists := func(name string) bool {
		for _, r := range p.vm.Remotes() {
			if r.Name == name {
				return true
			}
		}
		return false
	}
	newWizardView(p, NewWizard(p.u.Cloud(), p.vm.Providers(), exists)).show()
}
