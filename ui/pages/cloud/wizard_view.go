package cloud

import (
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	"github.com/akfaiz/carryover/ui/i18n"

	"github.com/akfaiz/carryover/internal/cloudsync/cloud"
	"github.com/akfaiz/carryover/ui/kit"
)

// wizardView is the dialog that adds a remote: pick provider → options form →
// rclone's follow-up questions (OAuth etc.).
type wizardView struct {
	p    *Page
	vm   *Wizard
	d    *dialog.CustomDialog
	body *fyne.Container
}

func newWizardView(p *Page, vm *Wizard) *wizardView {
	return &wizardView{p: p, vm: vm, body: container.NewStack()}
}

func (w *wizardView) show() {
	win := w.p.u.Window()
	w.d = dialog.NewCustomWithoutButtons(i18n.L("Add remote"), container.NewVScroll(w.body), win)
	w.d.Resize(fyne.NewSize(min(640, win.Canvas().Size().Width-40), min(560, win.Canvas().Size().Height-40)))
	w.pick()
	w.d.Show()
	kit.OnBack(win, w.close)
}

func (w *wizardView) set(objs ...fyne.CanvasObject) {
	w.body.Objects = []fyne.CanvasObject{kit.VBox(objs...)}
	w.body.Refresh()
}

func (w *wizardView) close() { w.d.Hide() }

func (w *wizardView) pick() {
	grid := container.NewGridWrap(fyne.NewSize(190, 96))
	for _, pr := range w.vm.Providers() {
		name := widget.NewLabelWithStyle(pr.Description, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
		name.Wrapping = fyne.TextWrapWord
		b := kit.NewTappable(kit.Card("", name, kit.Small(pr.Type)), func() { w.vm.Choose(pr); w.renderForm("") })
		grid.Add(b)
	}
	w.set(kit.Heading(i18n.L("Choose storage provider")), grid, kit.HBox(widget.NewButton(i18n.L("Cancel"), w.close)))
}

func (w *wizardView) renderForm(errText string) {
	vm := w.vm
	pr := vm.Provider()
	name := widget.NewEntry()
	name.SetText(vm.Name())
	cont := kit.Primary(i18n.L("Continue"), w.create)
	name.OnChanged = func(s string) { vm.SetName(s); kit.SetEnabled(cont, vm.NameOK()) }
	kit.SetEnabled(cont, vm.NameOK())
	objs := []fyne.CanvasObject{kit.Heading(i18n.L("New {{.Provider}} remote", map[string]any{"Provider": pr.Description})), kit.Field(i18n.L("Name"), name)}
	opts, more := vm.VisibleOptions()
	for _, o := range opts {
		label := o.Name
		if o.Required {
			label += " *"
		}
		label += " — " + firstLine(o.Help)
		objs = append(objs, kit.Field(label, w.optionInput(o)))
	}
	if more {
		objs = append(objs, kit.HBox(kit.LowButton(i18n.L("Show all options"), func() { vm.ShowAll(); w.renderForm(errText) })))
	}
	if pr.OAuth {
		objs = append(objs, kit.Muted(i18n.L("After you continue, your browser opens to sign in. Come back here when done.")))
	}
	if errText != "" {
		objs = append(objs, kit.Colored(errText, widget.DangerImportance))
	}
	objs = append(objs, kit.HBox(widget.NewButton(i18n.L("Back"), w.pick), cont))
	w.set(objs...)
}

func firstLine(s string) string {
	first, _, _ := strings.Cut(s, "\n")
	return first
}

func (w *wizardView) optionInput(o cloud.ProviderOption) fyne.CanvasObject {
	set := func(v string) { w.vm.SetParam(o.Name, v) }
	cur := w.vm.Param(o.Name)
	if len(o.Examples) > 0 && o.Exclusive {
		c := &kit.Choice{}
		def := i18n.L("(default)")
		if o.Default != "" && o.Default != "<nil>" {
			def = i18n.L("(default: {{.Value}})", map[string]any{"Value": o.Default})
		}
		c.Add("", def)
		for _, ex := range o.Examples {
			c.Add(ex.Value, ex.Value+" — "+ex.Help)
		}
		return kit.Outlined(kit.SelectOf(c, cur, set))
	}
	if o.IsPassword {
		e := widget.NewPasswordEntry()
		e.SetText(cur)
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
		e.SetText(cur)
		e.OnChanged = set
		return e
	}
	e := widget.NewEntry()
	if o.Default != "<nil>" {
		e.SetPlaceHolder(o.Default)
	}
	e.SetText(cur)
	e.OnChanged = set
	return e
}

func (w *wizardView) working() {
	bar := widget.NewProgressBarInfinite()
	w.set(kit.Heading(i18n.L("Configuring…")), kit.Muted(i18n.L("If a browser window opened, finish signing in there.")), bar,
		// rclone keeps waiting in the background until its own timeout.
		kit.HBox(widget.NewButton(i18n.L("Cancel"), w.close)))
}

// run is how the wizard's steps run: off the UI thread, then back on it.
func (w *wizardView) run(work func() (*cloud.Question, error), done func(*cloud.Question, error)) {
	kit.Async(work, done)
}

// next shows the screen a step came to.
func (w *wizardView) next(r Result) {
	switch r.Kind {
	case ResultForm:
		w.renderForm(r.Err)
	case ResultQuestion:
		w.renderQuestion(r.Err)
	case ResultCreated:
		w.close()
		w.p.u.Toast(i18n.L("Remote “{{.Name}}” created", map[string]any{"Name": r.Name}), kit.ToastOK)
		w.p.vm.RemoteCreated(r.Name)
	}
}

func (w *wizardView) create() {
	w.working()
	w.vm.Create(w.run, w.next)
}

func (w *wizardView) reply(v string) {
	w.working()
	w.vm.Reply(v, w.run, w.next)
}

func (w *wizardView) renderQuestion(errText string) {
	q := w.vm.Question()
	help := widget.NewLabel(q.Help)
	help.Wrapping = fyne.TextWrapWord
	objs := []fyne.CanvasObject{kit.Heading(w.vm.Provider().Description), help}
	errLabel := func() {
		if errText != "" {
			objs = append(objs, kit.Colored(errText, widget.DangerImportance))
		}
	}
	if q.IsBool {
		errLabel()
		objs = append(objs, kit.HBox(widget.NewButton(i18n.L("No"), func() { w.reply("false") }), kit.Primary(i18n.L("Yes"), func() { w.reply("true") })))
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
		input = kit.Outlined(kit.SelectOf(c, answer, func(v string) { answer = v }))
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
	objs = append(objs, kit.FullHeight(input))
	errLabel()
	objs = append(objs, kit.HBox(widget.NewButton(i18n.L("Cancel"), w.close), kit.Primary(i18n.L("Continue"), func() { w.reply(answer) })))
	w.set(objs...)
}
