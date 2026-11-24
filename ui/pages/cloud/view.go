package cloud

import (
	"fmt"
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/akfaiz/carryover/internal/app/appdir"
	"github.com/akfaiz/carryover/internal/cloudsync/cloud"
	"github.com/akfaiz/carryover/ui/components"
	"github.com/akfaiz/carryover/ui/env"
	"github.com/akfaiz/carryover/ui/i18n"
	"github.com/akfaiz/carryover/ui/kit"
	"github.com/akfaiz/carryover/ui/store"
)

// Page is the Cloud storage page.
type Page struct {
	u      env.Env
	vm     *ViewModel
	test   *kit.TestButton
	scroll ui.ScrollState

	root     string // the folder field
	lastRoot string // the ViewModel's, as of the last frame
}

// New returns the Cloud storage page.
func New(u env.Env) *Page {
	p := &Page{u: u}
	p.vm = NewViewModel(Deps{
		Cloud:    u.Cloud(),
		Settings: u.Sync(),
		Current:  func() appdir.Settings { return u.Store().Settings },
		Run:      func(work func() error, done func(error)) { kit.Async0(work, done) },
		Changed:  kit.Redraw,
		Refresh:  u.Store().Refresh,
		Error:    u.ToastErr,
		Saved:    func() { u.Toast(i18n.L("Cloud location saved"), kit.ToastOK) },
		SyncFailed: func(err error) {
			u.Toast(i18n.L("Library sync: {{.Err}}", map[string]any{"Err": err}), kit.ToastErr)
		},
		Imported: p.reportImport,
	})
	p.test = kit.NewTestButton(i18n.L("Test connection"), i18n.L("Connection works."), i18n.L("Testing…"), func() func() error {
		remote, root := p.vm.Remote(), p.vm.Root()
		return func() error { return p.u.Cloud().Test(remote, root) }
	})
	p.vm.Open()
	return p
}

// Scroll is the page's scroll container, which the pad's right stick scrolls.
func (p *Page) Scroll() *ui.ScrollState { return &p.scroll }

// StateChanged: the page reads the ViewModel as it draws; a redraw is all it
// needs when the settings reload, so the "active" badge follows the saved remote.
func (p *Page) StateChanged(store.Topic) {}

// reportImport tells the user what an import found.
func (p *Page) reportImport(res cloud.ImportResult) {
	u := p.u
	if len(res.Imported) > 0 {
		u.Toast(i18n.L("Imported {{.Names}}", map[string]any{"Names": strings.Join(res.Imported, ", ")}), kit.ToastOK)
	}
	if len(res.Skipped) > 0 {
		u.Toast(i18n.L("Skipped unsupported: {{.Names}}", map[string]any{"Names": strings.Join(res.Skipped, ", ")}), kit.ToastErr)
	} else if len(res.Imported) == 0 {
		u.Toast(i18n.L("No remotes found in that file"), kit.ToastErr)
	}
}

// syncRoot keeps the folder field and the ViewModel's folder equal: the
// ViewModel's wins when it changed (settings loaded), the field's when typed.
func (p *Page) syncRoot() {
	switch {
	case p.vm.Root() != p.lastRoot:
		p.root, p.lastRoot = p.vm.Root(), p.vm.Root()
	case p.root != p.lastRoot:
		p.vm.SetRoot(p.root)
		p.lastRoot = p.root
	}
}

// View draws the page.
func (p *Page) View(c *ui.Context) {
	nav := p.u.Navigator()
	kit.Page(c, kit.PageForm, &p.scroll, func() {
		kit.PageHead(c, nav, nil, "", i18n.L("Cloud storage"), nil)
		p.syncRoot()
		if !p.vm.HasRemote() {
			// First run: the remote comes first; the first one added becomes the
			// sync location by itself.
			kit.Muted(c, i18n.L("Carryover keeps your saves in your own cloud storage. Add a remote (or import your rclone config) and it becomes the sync location; every device then uses the same one."))
			p.remotes(c)
			p.location(c)
			return
		}
		p.location(c)
		p.remotes(c)
	})
}

// btn draws a button that registers with the gamepad.
func (p *Page) btn(c *ui.Context, key string, kind kit.BtnKind, icon *ui.SVG, label string, action func()) {
	kit.Btn(c, p.u.Navigator(), [2]any{"cloud", key}, kind, icon, label, action)
}

func (p *Page) location(c *ui.Context) {
	u, vm, nav := p.u, p.vm, p.u.Navigator()
	remotes := &kit.Choice{}
	remotes.Add("", i18n.L("— none (sync disabled) —"))
	for _, r := range vm.Remotes() {
		l := fmt.Sprintf("%s (%s)", r.Name, r.Type)
		if !r.Supported {
			l = i18n.L("{{.Name}} ({{.Type}}, unsupported)", map[string]any{"Name": r.Name, "Type": r.Type})
		}
		remotes.Add(r.Name, l)
	}
	p.test.SetAllowed(vm.Remote() != "")
	kit.Card(c, i18n.L("Sync location"), func() {
		kit.Muted(c, i18n.L("All devices must use the same remote storage and folder. Saves go to <folder>/games/<game>/."))
		ui.Row(c).Gap(kit.S3).Children(func() {
			ui.Column(c).Grow(1).Children(func() {
				kit.Labeled(c, i18n.L("Remote"), func() {
					kit.SelectOf(c, nav, "remote", i18n.L("Remote"), remotes, vm.Remote(), func(v string) {
						if vm.Unsupported(v) {
							u.Toast(i18n.L("This provider isn't included in Carryover"), kit.ToastErr)
						}
						vm.SetRemote(v)
						p.test.Reset()
					})
				})
			})
			ui.Column(c).Grow(1).Children(func() {
				kit.Labeled(c, i18n.L("Folder"), func() {
					kit.Field(c, nav, "cloud-root", &p.root, "").Label(i18n.L("Folder")).Font("monospace")
				})
			})
		})
		ui.Row(c).Gap(kit.S2).Children(func() {
			p.btn(c, "use", kit.BtnPrimary, nil, i18n.L("Use this location"), vm.SaveTarget)
			p.test.Button(c, nav, "cloud")
		})
		p.test.Result(c)
	})
}

func (p *Page) remotes(c *ui.Context) {
	u, vm := p.u, p.vm
	actions := func() {
		if vm.DefaultConfig() != "" {
			p.btn(c, "import-default", kit.BtnSecondary, kit.IconDownload, i18n.L("Import from rclone"), func() { vm.ImportConf(vm.DefaultConfig()) })
		}
		p.btn(c, "import-file", kit.BtnSecondary, kit.IconFolderOpen, i18n.L("Import rclone.conf…"), p.pickConf)
		p.btn(c, "add", kit.BtnPrimary, kit.IconPlus, i18n.L("Add remote"), p.addRemote)
	}
	if len(vm.Remotes()) == 0 {
		// The first step of the setup: one empty state with every way to start.
		kit.Card(c, "", func() {
			kit.EmptyState(c, kit.IconCloud, i18n.L("No remotes yet"),
				i18n.L("Add one (Google Drive, Dropbox, OneDrive, WebDAV/Nextcloud, SFTP or SMB) or import your existing rclone config."), actions)
		})
		return
	}
	kit.Card(c, "", func() {
		kit.CardHead(c, i18n.L("Remotes"), actions)
		for i, r := range vm.Remotes() {
			p.remoteRow(c, i == 0, r, vm.IsActive(r.Name), func() {
				msg := i18n.L("Delete remote “{{.Name}}”? The files in it are not touched.", map[string]any{"Name": r.Name})
				if vm.IsActive(r.Name) {
					msg = i18n.L("Delete remote “{{.Name}}”? It is the sync location, so syncing stops until you choose another one. The files in it are not touched.", map[string]any{"Name": r.Name})
				}
				components.ConfirmDanger(u, components.ConfirmOpts{Title: i18n.L("Delete remote"), Message: msg, OnConfirm: func(bool) {
					vm.DeleteRemote(r.Name)
				}})
			})
		}
	})
}

// remoteRow is one configured rclone remote: name (with an "active" badge),
// type (with an "unsupported" badge) and a delete button.
func (p *Page) remoteRow(c *ui.Context, first bool, r cloud.Remote, active bool, onDelete func()) {
	kit.ListRow(c, first, r.Name, func() {
		ui.Row(c).Gap(kit.S2).AlignItems(ui.Center).Children(func() {
			kit.Small(c, r.Type)
			if active {
				kit.Badge(c, i18n.L("Active"), kit.BadgeOK)
			}
			if !r.Supported {
				kit.Badge(c, i18n.L("Unsupported"), kit.BadgeWarn)
			}
		})
	}, func() {
		p.btn(c, "remote-delete:"+r.Name, kit.BtnDanger, kit.IconTrash, i18n.L("Delete"), onDelete)
	})
}

func (p *Page) pickConf() {
	kit.PickFile(p.u.Window(), i18n.L("Select rclone.conf"), []string{".conf"}, "", p.vm.ImportConf)
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
