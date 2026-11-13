package ui

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

	"github.com/akfaiz/carryover/internal/cloudsync/cloud"
	"github.com/akfaiz/carryover/internal/ui/kit"
)

type cloudPage struct {
	u           *UI
	remotes     []cloud.Remote
	providers   []cloud.Provider
	defaultConf string
	remote      string
	root        string
	test        *kit.TestButton
	scroll      *container.Scroll
}

func newCloudPage(u *UI) *cloudPage { return &cloudPage{u: u, root: "Carryover"} }

func (p *cloudPage) view() fyne.CanvasObject {
	p.scroll = kit.ScrollPage(kit.Muted(lang.L("Loading…")))
	p.test = kit.NewTestButton(lang.L("Test connection"), lang.L("Connection works."), lang.L("Testing…"), func() func() error {
		remote, root := p.remote, p.root
		return func() error { return p.u.cloud.Test(remote, root) }
	})
	p.load(true)
	kit.Async(func() ([]cloud.Provider, error) { return p.u.cloud.Providers(), nil }, func(pr []cloud.Provider, _ error) { p.providers = pr })
	kit.Async(func() (string, error) { return p.u.cloud.DefaultRcloneConfig(), nil }, func(s string, _ error) {
		p.defaultConf = s
		p.render()
	})
	return p.scroll
}

// load reads remotes (and, with target, the configured location).
func (p *cloudPage) load(target bool) {
	kit.Async(func() ([]cloud.Remote, error) { return p.u.cloud.Remotes(), nil }, func(r []cloud.Remote, _ error) {
		p.remotes = r
		if target {
			st := p.u.sync.GetSettings()
			p.remote = st.Remote
			p.root = st.RemoteRoot
			if p.root == "" {
				p.root = "Carryover"
			}
		}
		p.render()
	})
}

// stateChanged re-renders when settings reload (e.g. after Save), so the
// "active" badge follows the saved remote.
func (p *cloudPage) stateChanged(what change) {
	if what == changeLibrary && p.scroll != nil {
		p.render()
	}
}

func (p *cloudPage) render() {
	u := p.u
	remotes := &kit.Choice{}
	remotes.Add("", lang.L("— none (sync disabled) —"))
	for _, r := range p.remotes {
		l := fmt.Sprintf("%s (%s)", r.Name, r.Type)
		if !r.Supported {
			l = lang.L("{{.Name}} ({{.Type}}, unsupported)", map[string]any{"Name": r.Name, "Type": r.Type})
		}
		remotes.Add(r.Name, l)
	}
	p.test.SetAllowed(p.remote != "")
	sel := kit.SelectOf(remotes, p.remote, func(v string) {
		for _, r := range p.remotes {
			if r.Name == v && !r.Supported {
				u.toast(lang.L("This provider isn't included in Carryover"), kit.ToastErr)
			}
		}
		p.remote = v
		p.test.Reset()
		p.test.SetAllowed(p.remote != "")
	})
	root := kit.MonoEntry("", p.root, func(s string) { p.root = s })

	location := kit.Card(lang.L("Sync location"),
		kit.Muted(lang.L("All devices must use the same remote storage and folder. Saves go to <folder>/games/<game>/.")),
		container.NewGridWithColumns(2, kit.Field(lang.L("Remote"), sel), kit.Field(lang.L("Folder"), root)),
		kit.HBox(kit.Primary(lang.L("Use this location"), p.saveTarget), p.test.Button),
		p.test.Result,
	)

	actions := []fyne.CanvasObject{}
	if p.defaultConf != "" {
		actions = append(actions, widget.NewButton(lang.L("Import from rclone"), func() { p.importConf(p.defaultConf) }))
	}
	actions = append(actions,
		widget.NewButton(lang.L("Import rclone.conf…"), p.pickConf),
		kit.Primary(lang.L("＋ Add remote"), func() { newRemoteWizard(p).show() }),
	)
	list := kit.VBox(kit.CardHead(lang.L("Remotes"), actions...))
	if len(p.remotes) == 0 {
		list.Add(kit.Muted(lang.L("No remotes yet. Add one (Google Drive, Dropbox, OneDrive, WebDAV/Nextcloud, SFTP or SMB) or import your existing rclone config.")))
	}
	for _, r := range p.remotes {
		list.Add(widget.NewSeparator())
		list.Add(newRemoteRow(r, r.Name == u.settings.Remote, func() {
			msg := lang.L("Delete remote “{{.Name}}”? The files in it are not touched.", map[string]any{"Name": r.Name})
			if r.Name == u.settings.Remote {
				msg = lang.L("Delete remote “{{.Name}}”? It is the sync location, so syncing stops until you choose another one. The files in it are not touched.", map[string]any{"Name": r.Name})
			}
			confirmDanger(u, confirmOpts{title: lang.L("Delete remote"), message: msg, onConfirm: func(bool) {
				kit.Async0(func() error { return u.cloud.DeleteRemote(r.Name) }, func(err error) {
					u.toastErr(err)
					p.load(false)
					u.refresh()
				})
			}})
		}))
	}
	if u.settings.Remote == "" {
		// First run: the remote comes first; the first one added becomes the
		// sync location by itself.
		intro := kit.Muted(lang.L("Carryover keeps your saves in your own cloud storage. Add a remote (or import your rclone config) and it becomes the sync location; every device then uses the same one."))
		kit.ReplaceScroll(p.scroll, kit.Stack(kit.PageHead(nil, lang.L("Cloud storage")), intro, kit.Card("", list), location))
		return
	}
	kit.ReplaceScroll(p.scroll, kit.Stack(kit.PageHead(nil, lang.L("Cloud storage")), location, kit.Card("", list)))
}

// useFirstRemote makes a remote just added the sync location when there is
// none yet, so a first setup needs no extra step.
func (p *cloudPage) useFirstRemote(name string) {
	if p.u.settings.Remote != "" || name == "" {
		return
	}
	p.remote = name
	p.saveTarget()
}

func (p *cloudPage) saveTarget() {
	u := p.u
	st := u.settings
	st.Remote = p.remote
	st.RemoteRoot = strings.Trim(strings.TrimSpace(p.root), "/")
	if st.RemoteRoot == "" {
		st.RemoteRoot = "Carryover"
	}
	remote := st.Remote
	kit.Async0(func() error {
		_, err := u.sync.SaveSettings(st)
		return err
	}, func(err error) {
		if err != nil {
			u.toastErr(err)
			return
		}
		u.refresh()
		u.toast(lang.L("Cloud location saved"), kit.ToastOK)
		if remote != "" {
			kit.Async0(u.sync.SyncLibrary, func(err error) {
				if err != nil {
					u.toast(lang.L("Library sync: {{.Err}}", map[string]any{"Err": err}), kit.ToastErr)
				}
			})
		}
	})
}

func (p *cloudPage) pickConf() {
	d := dialog.NewFileOpen(func(r fyne.URIReadCloser, err error) {
		if err != nil || r == nil {
			return
		}
		path := filepath.FromSlash(r.URI().Path())
		_ = r.Close()
		p.importConf(path)
	}, p.u.win)
	d.SetTitleText(lang.L("Select rclone.conf"))
	d.SetFilter(storage.NewExtensionFileFilter([]string{".conf"}))
	d.Resize(kit.DialogSize(p.u.win))
	d.Show()
}

func (p *cloudPage) importConf(path string) {
	u := p.u
	kit.Async(func() (cloud.ImportResult, error) { return u.cloud.ImportConfig(path) }, func(res cloud.ImportResult, err error) {
		if err != nil {
			u.toastErr(err)
			return
		}
		if len(res.Imported) > 0 {
			u.toast(lang.L("Imported {{.Names}}", map[string]any{"Names": strings.Join(res.Imported, ", ")}), kit.ToastOK)
		}
		if len(res.Imported) == 1 {
			p.useFirstRemote(res.Imported[0])
		}
		if len(res.Skipped) > 0 {
			u.toast(lang.L("Skipped unsupported: {{.Names}}", map[string]any{"Names": strings.Join(res.Skipped, ", ")}), kit.ToastErr)
		} else if len(res.Imported) == 0 {
			u.toast(lang.L("No remotes found in that file"), kit.ToastErr)
		}
		p.load(false)
	})
}

// remoteWizard creates a remote: pick provider → options form → rclone's
// follow-up questions (OAuth etc.).
type remoteWizard struct {
	p        *cloudPage
	d        *dialog.CustomDialog
	body     *fyne.Container
	prov     *cloud.Provider
	name     string
	params   map[string]string
	showAll  bool
	question *cloud.Question
}

func newRemoteWizard(p *cloudPage) *remoteWizard {
	return &remoteWizard{p: p, body: container.NewStack()}
}

func (w *remoteWizard) show() {
	w.d = dialog.NewCustomWithoutButtons(lang.L("Add remote"), container.NewVScroll(w.body), w.p.u.win)
	w.d.Resize(fyne.NewSize(min(640, w.p.u.win.Canvas().Size().Width-40), min(560, w.p.u.win.Canvas().Size().Height-40)))
	w.pick()
	w.d.Show()
}

func (w *remoteWizard) set(objs ...fyne.CanvasObject) {
	w.body.Objects = []fyne.CanvasObject{kit.VBox(objs...)}
	w.body.Refresh()
}

func (w *remoteWizard) close() { w.d.Hide() }

func (w *remoteWizard) pick() {
	grid := container.NewGridWrap(fyne.NewSize(190, 96))
	for _, pr := range w.p.providers {
		name := widget.NewLabelWithStyle(pr.Description, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
		name.Wrapping = fyne.TextWrapWord
		b := kit.NewTappable(kit.Card("", name, kit.Small(pr.Type)), func() { w.form(pr) })
		grid.Add(b)
	}
	w.set(kit.Heading(lang.L("Choose storage provider")), grid, kit.HBox(widget.NewButton(lang.L("Cancel"), w.close)))
}

func (w *remoteWizard) form(pr cloud.Provider) {
	if w.prov == nil || w.prov.Type != pr.Type {
		w.prov = &pr
		w.name = pr.Type
		if pr.Type == "drive" {
			w.name = "gdrive"
		}
		for i := 2; w.exists(w.name); i++ {
			w.name = fmt.Sprintf("%s%d", pr.Type, i)
		}
		w.params = map[string]string{}
		w.showAll = false
	}
	w.renderForm("")
}

func (w *remoteWizard) exists(name string) bool {
	for _, r := range w.p.remotes {
		if r.Name == name {
			return true
		}
	}
	return false
}

func (w *remoteWizard) renderForm(errText string) {
	pr := w.prov
	name := widget.NewEntry()
	name.SetText(w.name)
	cont := kit.Primary(lang.L("Continue"), w.create)
	name.OnChanged = func(s string) { w.name = s; kit.SetEnabled(cont, strings.TrimSpace(s) != "") }
	kit.SetEnabled(cont, strings.TrimSpace(w.name) != "")
	objs := []fyne.CanvasObject{kit.Heading(lang.L("New {{.Provider}} remote", map[string]any{"Provider": pr.Description})), kit.Field(lang.L("Name"), name)}
	visible := 0
	for _, o := range pr.Options {
		exclusive := len(o.Examples) > 0 && o.Exclusive
		if !w.showAll && !o.Required && o.Name != "provider" && !exclusive {
			continue
		}
		visible++
		label := o.Name
		if o.Required {
			label += " *"
		}
		label += " — " + firstLine(o.Help)
		objs = append(objs, kit.Field(label, w.optionInput(o)))
	}
	if len(pr.Options) > visible {
		objs = append(objs, kit.HBox(kit.LowButton(lang.L("Show all options"), func() { w.showAll = true; w.renderForm(errText) })))
	}
	if pr.OAuth {
		objs = append(objs, kit.Muted(lang.L("After you continue, your browser opens to sign in. Come back here when done.")))
	}
	if errText != "" {
		objs = append(objs, kit.Colored(errText, widget.DangerImportance))
	}
	objs = append(objs, kit.HBox(widget.NewButton(lang.L("Back"), w.pick), cont))
	w.set(objs...)
}

func firstLine(s string) string {
	first, _, _ := strings.Cut(s, "\n")
	return first
}

func (w *remoteWizard) optionInput(o cloud.ProviderOption) fyne.CanvasObject {
	set := func(v string) { w.params[o.Name] = v }
	if len(o.Examples) > 0 && o.Exclusive {
		c := &kit.Choice{}
		def := lang.L("(default)")
		if o.Default != "" && o.Default != "<nil>" {
			def = lang.L("(default: {{.Value}})", map[string]any{"Value": o.Default})
		}
		c.Add("", def)
		for _, ex := range o.Examples {
			c.Add(ex.Value, ex.Value+" — "+ex.Help)
		}
		return kit.SelectOf(c, w.params[o.Name], set)
	}
	if o.IsPassword {
		e := widget.NewPasswordEntry()
		e.SetText(w.params[o.Name])
		e.OnChanged = set
		return e
	}
	if len(o.Examples) > 0 {
		var opts []string
		for _, ex := range o.Examples {
			opts = append(opts, ex.Value)
		}
		e := widget.NewSelectEntry(opts)
		e.SetPlaceHolder(o.Default)
		e.SetText(w.params[o.Name])
		e.OnChanged = set
		return e
	}
	e := widget.NewEntry()
	if o.Default != "<nil>" {
		e.SetPlaceHolder(o.Default)
	}
	e.SetText(w.params[o.Name])
	e.OnChanged = set
	return e
}

func (w *remoteWizard) working() {
	bar := widget.NewProgressBarInfinite()
	w.set(kit.Heading(lang.L("Configuring…")), kit.Muted(lang.L("If a browser window opened, finish signing in there.")), bar,
		// rclone keeps waiting in the background until its own timeout.
		kit.HBox(widget.NewButton(lang.L("Cancel"), w.close)))
}

func (w *remoteWizard) step(fn func() (*cloud.Question, error)) {
	w.working()
	kit.Async(fn, func(q *cloud.Question, err error) {
		u := w.p.u
		if err != nil {
			if w.question != nil {
				w.renderQuestion(err.Error())
			} else {
				w.renderForm(err.Error())
			}
			return
		}
		if q == nil || q.Done {
			w.close()
			name := strings.TrimSpace(w.name)
			w.p.remote = name
			w.p.load(false)
			u.toast(lang.L("Remote “{{.Name}}” created", map[string]any{"Name": name}), kit.ToastOK)
			w.p.useFirstRemote(name)
			return
		}
		w.question = q
		w.renderQuestion(q.Error)
	})
}

func (w *remoteWizard) create() {
	name, typ, params := strings.TrimSpace(w.name), w.prov.Type, w.params
	w.step(func() (*cloud.Question, error) { return w.p.u.cloud.CreateRemote(name, typ, params) })
}

func (w *remoteWizard) reply(v string) {
	name, state := strings.TrimSpace(w.name), w.question.State
	w.step(func() (*cloud.Question, error) { return w.p.u.cloud.ContinueRemote(name, state, v) })
}

func (w *remoteWizard) renderQuestion(errText string) {
	q := w.question
	help := widget.NewLabel(q.Help)
	help.Wrapping = fyne.TextWrapWord
	objs := []fyne.CanvasObject{kit.Heading(w.prov.Description), help}
	errLabel := func() {
		if errText != "" {
			objs = append(objs, kit.Colored(errText, widget.DangerImportance))
		}
	}
	if q.IsBool {
		errLabel()
		objs = append(objs, kit.HBox(widget.NewButton(lang.L("No"), func() { w.reply("false") }), kit.Primary(lang.L("Yes"), func() { w.reply("true") })))
		w.set(objs...)
		return
	}
	answer := q.Default
	var input fyne.CanvasObject
	switch {
	case len(q.Examples) > 0 && q.Exclusive:
		c := &kit.Choice{}
		for _, ex := range q.Examples {
			c.Add(ex.Value, ex.Value+" — "+ex.Help)
		}
		input = kit.SelectOf(c, answer, func(v string) { answer = v })
	case q.IsPassword:
		e := widget.NewPasswordEntry()
		e.SetText(answer)
		e.OnChanged = func(s string) { answer = s }
		input = e
	case len(q.Examples) > 0:
		var opts []string
		for _, ex := range q.Examples {
			opts = append(opts, ex.Value)
		}
		e := widget.NewSelectEntry(opts)
		e.SetText(answer)
		e.OnChanged = func(s string) { answer = s }
		input = e
	default:
		e := widget.NewEntry()
		e.SetText(answer)
		e.OnChanged = func(s string) { answer = s }
		input = e
	}
	objs = append(objs, input)
	errLabel()
	objs = append(objs, kit.HBox(widget.NewButton(lang.L("Cancel"), w.close), kit.Primary(lang.L("Continue"), func() { w.reply(answer) })))
	w.set(objs...)
}
