package ui

import (
	"fmt"
	"path/filepath"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/akfaiz/playrelay/internal/cloud"
)

type cloudPage struct {
	u           *UI
	remotes     []cloud.Remote
	providers   []cloud.Provider
	defaultConf string
	remote      string
	root        string
	test        *testButton
	scroll      *container.Scroll
}

func newCloudPage(u *UI) *cloudPage { return &cloudPage{u: u, root: "PlayRelay"} }

func (p *cloudPage) view() fyne.CanvasObject {
	p.scroll = scrollPage(muted("Loading…"))
	p.test = newTestButton("Test connection", "Connection works.", func() func() error {
		remote, root := p.remote, p.root
		return func() error { return p.u.cloud.Test(remote, root) }
	})
	p.load(true)
	async(func() ([]cloud.Provider, error) { return p.u.cloud.Providers(), nil }, func(pr []cloud.Provider, _ error) { p.providers = pr })
	async(func() (string, error) { return p.u.cloud.DefaultRcloneConfig(), nil }, func(s string, _ error) {
		p.defaultConf = s
		p.render()
	})
	return p.scroll
}

// load reads remotes (and, with target, the configured location).
func (p *cloudPage) load(target bool) {
	async(func() ([]cloud.Remote, error) { return p.u.cloud.Remotes(), nil }, func(r []cloud.Remote, _ error) {
		p.remotes = r
		if target {
			st := p.u.sync.GetSettings()
			p.remote = st.Remote
			p.root = st.RemoteRoot
			if p.root == "" {
				p.root = "PlayRelay"
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
	remotes := &choice{}
	remotes.add("", "— none (sync disabled) —")
	for _, r := range p.remotes {
		l := fmt.Sprintf("%s (%s)", r.Name, r.Type)
		if !r.Supported {
			l = fmt.Sprintf("%s (%s, unsupported)", r.Name, r.Type)
		}
		remotes.add(r.Name, l)
	}
	p.test.SetAllowed(p.remote != "")
	sel := selectOf(remotes, p.remote, func(v string) {
		for _, r := range p.remotes {
			if r.Name == v && !r.Supported {
				u.toast("This provider isn't included in PlayRelay", toastErr)
			}
		}
		p.remote = v
		p.test.Reset()
		p.test.SetAllowed(p.remote != "")
	})
	root := widget.NewEntry()
	root.TextStyle.Monospace = true
	root.SetText(p.root)
	root.OnChanged = func(s string) { p.root = s }

	location := card("Sync location",
		muted("All devices must use the same remote storage and folder. Saves go to <folder>/games/<game>/."),
		container.NewGridWithColumns(2, field("Remote", sel), field("Folder", root)),
		hbox(primary("Save", p.saveTarget), p.test.Button),
		p.test.Result,
	)

	actions := []fyne.CanvasObject{}
	if p.defaultConf != "" {
		actions = append(actions, widget.NewButton("Import from rclone", func() { p.importConf(p.defaultConf) }))
	}
	actions = append(actions,
		widget.NewButton("Import rclone.conf…", p.pickConf),
		primary("＋ Add remote", func() { newRemoteWizard(p).show() }),
	)
	list := vbox(cardHead("Remotes", actions...))
	if len(p.remotes) == 0 {
		list.Add(muted("No remotes yet. Add one (Google Drive, Dropbox, OneDrive, WebDAV/Nextcloud, SFTP or SMB) or import your existing rclone config."))
	}
	for _, r := range p.remotes {
		name := hbox(widget.NewLabelWithStyle(r.Name, fyne.TextAlignLeading, fyne.TextStyle{Bold: true}))
		if r.Name == u.settings.Remote {
			name.Add(container.NewCenter(badge("active", badgeOK)))
		}
		typ := hbox(small(r.Type))
		if !r.Supported {
			typ.Add(container.NewCenter(badge("unsupported", badgeWarn)))
		}
		del := widget.NewButtonWithIcon("Delete", theme.DeleteIcon(), func() {
			dialog.ShowConfirm("Delete remote", "Delete remote “"+r.Name+"”?", func(ok bool) {
				if !ok {
					return
				}
				async0(func() error { return u.cloud.DeleteRemote(r.Name) }, func(err error) {
					u.toastErr(err)
					p.load(false)
					u.refresh()
				})
			}, u.win)
		})
		list.Add(widget.NewSeparator())
		list.Add(container.NewBorder(nil, nil, nil, del, container.NewGridWithColumns(2, name, typ)))
	}
	replaceScroll(p.scroll, vbox(pageHead(nil, "Cloud storage"), location, card("", list)))
}

func (p *cloudPage) saveTarget() {
	u := p.u
	st := u.settings
	st.Remote = p.remote
	st.RemoteRoot = strings.Trim(strings.TrimSpace(p.root), "/")
	if st.RemoteRoot == "" {
		st.RemoteRoot = "PlayRelay"
	}
	remote := st.Remote
	async0(func() error {
		_, err := u.sync.SaveSettings(st)
		return err
	}, func(err error) {
		if err != nil {
			u.toastErr(err)
			return
		}
		u.refresh()
		u.toast("Cloud location saved", toastOK)
		if remote != "" {
			async0(u.sync.SyncLibrary, func(err error) {
				if err != nil {
					u.toast("Library sync: "+err.Error(), toastErr)
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
	d.SetTitleText("Select rclone.conf")
	d.SetFilter(storage.NewExtensionFileFilter([]string{".conf"}))
	d.Resize(dialogSize(p.u.win))
	d.Show()
}

func (p *cloudPage) importConf(path string) {
	u := p.u
	async(func() (cloud.ImportResult, error) { return u.cloud.ImportConfig(path) }, func(res cloud.ImportResult, err error) {
		if err != nil {
			u.toastErr(err)
			return
		}
		if len(res.Imported) > 0 {
			u.toast("Imported "+strings.Join(res.Imported, ", "), toastOK)
		}
		if len(res.Skipped) > 0 {
			u.toast("Skipped unsupported: "+strings.Join(res.Skipped, ", "), toastErr)
		} else if len(res.Imported) == 0 {
			u.toast("No remotes found in that file", toastErr)
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
	w.d = dialog.NewCustomWithoutButtons("Add remote", container.NewVScroll(w.body), w.p.u.win)
	w.d.Resize(fyne.NewSize(min(640, w.p.u.win.Canvas().Size().Width-40), min(560, w.p.u.win.Canvas().Size().Height-40)))
	w.pick()
	w.d.Show()
}

func (w *remoteWizard) set(objs ...fyne.CanvasObject) {
	w.body.Objects = []fyne.CanvasObject{vbox(objs...)}
	w.body.Refresh()
}

func (w *remoteWizard) close() { w.d.Hide() }

func (w *remoteWizard) pick() {
	grid := container.NewGridWrap(fyne.NewSize(190, 96))
	for _, pr := range w.p.providers {
		b := newTappable(card("", widget.NewLabelWithStyle(pr.Type, fyne.TextAlignLeading, fyne.TextStyle{Bold: true}), muted(pr.Description)),
			func() { w.form(pr) })
		grid.Add(b)
	}
	w.set(heading("Choose storage provider"), grid, hbox(widget.NewButton("Cancel", w.close)))
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
	cont := primary("Continue", w.create)
	name.OnChanged = func(s string) { w.name = s; setEnabled(cont, strings.TrimSpace(s) != "") }
	setEnabled(cont, strings.TrimSpace(w.name) != "")
	objs := []fyne.CanvasObject{heading("New " + pr.Description + " remote"), field("Name", name)}
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
		objs = append(objs, field(label, w.optionInput(o)))
	}
	if len(pr.Options) > visible {
		objs = append(objs, hbox(lowButton("Show all options", func() { w.showAll = true; w.renderForm(errText) })))
	}
	if pr.OAuth {
		objs = append(objs, muted("After you continue, your browser opens to sign in. Come back here when done."))
	}
	if errText != "" {
		objs = append(objs, colored(errText, widget.DangerImportance))
	}
	objs = append(objs, hbox(widget.NewButton("Back", w.pick), cont))
	w.set(objs...)
}

func firstLine(s string) string {
	first, _, _ := strings.Cut(s, "\n")
	return first
}

func (w *remoteWizard) optionInput(o cloud.ProviderOption) fyne.CanvasObject {
	set := func(v string) { w.params[o.Name] = v }
	if len(o.Examples) > 0 && o.Exclusive {
		c := &choice{}
		def := "(default)"
		if o.Default != "" && o.Default != "<nil>" {
			def = "(default: " + o.Default + ")"
		}
		c.add("", def)
		for _, ex := range o.Examples {
			c.add(ex.Value, ex.Value+" — "+ex.Help)
		}
		return selectOf(c, w.params[o.Name], set)
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
	w.set(heading("Configuring…"), muted("If a browser window opened, finish signing in there."), bar,
		// rclone keeps waiting in the background until its own timeout.
		hbox(widget.NewButton("Cancel", w.close)))
}

func (w *remoteWizard) step(fn func() (*cloud.Question, error)) {
	w.working()
	async(fn, func(q *cloud.Question, err error) {
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
			w.p.remote = strings.TrimSpace(w.name)
			w.p.load(false)
			u.toast("Remote “"+strings.TrimSpace(w.name)+"” created", toastOK)
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
	objs := []fyne.CanvasObject{heading(w.prov.Description), help}
	errLabel := func() {
		if errText != "" {
			objs = append(objs, colored(errText, widget.DangerImportance))
		}
	}
	if q.IsBool {
		errLabel()
		objs = append(objs, hbox(widget.NewButton("No", func() { w.reply("false") }), primary("Yes", func() { w.reply("true") })))
		w.set(objs...)
		return
	}
	answer := q.Default
	var input fyne.CanvasObject
	switch {
	case len(q.Examples) > 0 && q.Exclusive:
		c := &choice{}
		for _, ex := range q.Examples {
			c.add(ex.Value, ex.Value+" — "+ex.Help)
		}
		input = selectOf(c, answer, func(v string) { answer = v })
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
	objs = append(objs, hbox(widget.NewButton("Cancel", w.close), primary("Continue", func() { w.reply(answer) })))
	w.set(objs...)
}
