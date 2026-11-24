package cloud

import (
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/akfaiz/carryover/internal/cloudsync/cloud"
	"github.com/akfaiz/carryover/ui/i18n"
	"github.com/akfaiz/carryover/ui/kit"
)

// wizardStage is the screen of the wizard's dialog.
type wizardStage int

const (
	stagePick     wizardStage = iota // choose a provider
	stageForm                        // its options
	stageWorking                     // rclone is configuring (a browser may be open)
	stageQuestion                    // one of rclone's follow-up questions
)

// wizardView is the dialog that adds a remote: pick provider → options form →
// rclone's follow-up questions (OAuth etc.). It keeps the text of each field
// and copies it into the Wizard when it changes.
type wizardView struct {
	p     *Page
	vm    *Wizard
	d     *kit.Dialog
	stage wizardStage
	err   string

	name   string
	params map[string]*string // the option fields, by option name
	answer string             // the answer to the question on screen
	foot   func()             // the buttons under the scrolling content, set by the stage
}

func newWizardView(p *Page, vm *Wizard) *wizardView {
	return &wizardView{p: p, vm: vm, params: map[string]*string{}}
}

func (w *wizardView) show() {
	w.d = w.p.u.Dialogs().Show(&kit.Dialog{Title: i18n.L("Add remote"), Wide: true, Back: w.close, Body: w.body})
}

func (w *wizardView) close() { w.d.Close() }

func (w *wizardView) body(c *ui.Context, nav *kit.Navigator) {
	w.foot = nil
	ui.Scroll(c).MaxHeight(480).Children(func() {
		ui.Column(c).Gap(10).Children(func() {
			switch w.stage {
			case stagePick:
				w.pick(c, nav)
			case stageForm:
				w.form(c, nav)
			case stageWorking:
				w.working(c, nav)
			case stageQuestion:
				w.question(c, nav)
			}
		})
	})
	// The buttons stay in sight however long the form is.
	if w.foot != nil {
		w.foot()
	}
}

// button draws a button that registers with the gamepad.
func (w *wizardView) button(c *ui.Context, nav *kit.Navigator, key, label string, primary bool, disabled bool, action func(), opts ...kit.ItemOption) {
	k := [2]any{"wizard", key}
	var b ui.Element
	if primary {
		b = ui.PrimaryButton(c.Key(k), label)
	} else {
		b = ui.Button(c.Key(k), label)
	}
	if nav.Item(nav.Handle(k), b.Disabled(disabled), action, opts...).Clicked() && !disabled {
		action()
	}
}

func (w *wizardView) pick(c *ui.Context, nav *kit.Navigator) {
	kit.Heading(c, i18n.L("Choose storage provider"))
	ui.Row(c).Wrap().Gap(10).Children(func() {
		for _, pr := range w.vm.Providers() {
			k := [2]any{"provider", pr.Type}
			choose := func() { w.vm.Choose(pr); w.name = w.vm.Name(); w.stage, w.err = stageForm, "" }
			b := ui.ButtonBase(c.Key(k)).Label(pr.Description).Width(190).Height(96).Radius(10).
				Background(c.Theme().Surface).Border(1, c.Theme().Border).Children(func() {
				ui.Column(c).Padding(12).Gap(4).Children(func() {
					ui.Text(c, pr.Description).Bold().Wrap()
					kit.Small(c, pr.Type)
				})
			})
			if nav.Item(nav.Handle(k), b, choose).Clicked() {
				choose()
			}
		}
	})
	w.foot = func() { w.button(c, nav, "cancel", i18n.L("Cancel"), false, false, w.close, kit.FocusSafe) }
}

func (w *wizardView) form(c *ui.Context, nav *kit.Navigator) {
	vm := w.vm
	pr := vm.Provider()
	if w.name != vm.Name() {
		vm.SetName(w.name)
	}
	kit.Heading(c, i18n.L("New {{.Provider}} remote", map[string]any{"Provider": pr.Description}))
	kit.Labeled(c, i18n.L("Name"), func() {
		nav.Item(nav.Handle("wiz-name"), ui.TextInput(c.Key("wiz-name"), &w.name).Label(i18n.L("Name")), nil, kit.TextField)
	})
	opts, more := vm.VisibleOptions()
	for _, o := range opts {
		label := o.Name
		if o.Required {
			label += " *"
		}
		label += " — " + firstLine(o.Help)
		kit.Labeled(c, label, func() { w.optionInput(c, nav, o) })
	}
	if more {
		w.button(c, nav, "all", i18n.L("Show all options"), false, false, vm.ShowAll)
	}
	if pr.OAuth {
		kit.Muted(c, i18n.L("After you continue, your browser opens to sign in. Come back here when done."))
	}
	if w.err != "" {
		kit.Danger(c, w.err)
	}
	w.foot = func() {
		ui.Row(c).Gap(8).Children(func() {
			w.button(c, nav, "back", i18n.L("Back"), false, false, func() { w.stage, w.err = stagePick, "" }, kit.FocusSafe)
			w.button(c, nav, "continue", i18n.L("Continue"), true, !vm.NameOK(), w.create)
		})
	}
}

func firstLine(s string) string {
	first, _, _ := strings.Cut(s, "\n")
	return first
}

// field is the text of an option, kept across frames.
func (w *wizardView) field(name string) *string {
	if w.params[name] == nil {
		v := w.vm.Param(name)
		w.params[name] = &v
	}
	return w.params[name]
}

func (w *wizardView) optionInput(c *ui.Context, nav *kit.Navigator, o cloud.ProviderOption) {
	cur := w.field(o.Name)
	defer func() {
		if *cur != w.vm.Param(o.Name) {
			w.vm.SetParam(o.Name, *cur)
		}
	}()
	k := [2]any{"wiz-opt", o.Name}
	if len(o.Examples) > 0 && o.Exclusive {
		ch := &kit.Choice{}
		def := i18n.L("(default)")
		if o.Default != "" && o.Default != "<nil>" {
			def = i18n.L("(default: {{.Value}})", map[string]any{"Value": o.Default})
		}
		ch.Add("", def)
		for _, ex := range o.Examples {
			ch.Add(ex.Value, ex.Value+" — "+ex.Help)
		}
		kit.SelectOf(c, nav, w.p.u.Dialogs(), k, o.Name, ch, *cur, func(v string) { *cur = v })
		return
	}
	in := ui.TextInput(c.Key(k), cur)
	if o.IsPassword {
		in = in.Password()
	} else if o.Default != "<nil>" {
		in = in.Placeholder(o.Default)
	}
	nav.Item(nav.Handle(k), in, nil, kit.TextField)
	if !o.IsPassword && len(o.Examples) > 0 {
		w.examples(c, nav, k, o, cur)
	}
}

// examples offers the example values of an option that also takes any text.
func (w *wizardView) examples(c *ui.Context, nav *kit.Navigator, k [2]any, o cloud.ProviderOption, cur *string) {
	ui.Row(c).Wrap().Gap(6).Children(func() {
		for i, ex := range o.Examples {
			ek := [3]any{k[0], k[1], i}
			set := func() { *cur = ex.Value }
			if nav.Item(nav.Handle(ek), ui.Button(c.Key(ek), ex.Value), set).Clicked() {
				set()
			}
		}
	})
}

func (w *wizardView) working(c *ui.Context, nav *kit.Navigator) {
	kit.Heading(c, i18n.L("Configuring…"))
	kit.Muted(c, i18n.L("If a browser window opened, finish signing in there."))
	ui.Progress(c, -1)
	// rclone keeps waiting in the background until its own timeout.
	w.foot = func() { w.button(c, nav, "cancel", i18n.L("Cancel"), false, false, w.close, kit.FocusSafe) }
}

// run is how the wizard's steps run: off the UI thread, then back on it.
func (w *wizardView) run(work func() (*cloud.Question, error), done func(*cloud.Question, error)) {
	kit.Async(work, done)
}

// next shows the screen a step came to.
func (w *wizardView) next(r Result) {
	switch r.Kind {
	case ResultForm:
		w.stage, w.err = stageForm, r.Err
	case ResultQuestion:
		w.stage, w.err = stageQuestion, r.Err
		w.answer = w.vm.Question().Default
	case ResultCreated:
		w.close()
		w.p.u.Toast(i18n.L("Remote “{{.Name}}” created", map[string]any{"Name": r.Name}), kit.ToastOK)
		w.p.vm.RemoteCreated(r.Name)
	}
}

func (w *wizardView) create() {
	w.stage = stageWorking
	w.vm.Create(w.run, w.next)
}

func (w *wizardView) reply(v string) {
	w.stage = stageWorking
	w.vm.Reply(v, w.run, w.next)
}

func (w *wizardView) question(c *ui.Context, nav *kit.Navigator) {
	q := w.vm.Question()
	kit.Heading(c, w.vm.Provider().Description)
	ui.Text(c, q.Help).Wrap()
	if w.err != "" {
		kit.Danger(c, w.err)
	}
	if q.IsBool {
		w.foot = func() {
			ui.Row(c).Gap(8).Children(func() {
				w.button(c, nav, "no", i18n.L("No"), false, false, func() { w.reply("false") }, kit.FocusSafe)
				w.button(c, nav, "yes", i18n.L("Yes"), true, false, func() { w.reply("true") })
			})
		}
		return
	}
	k := "wiz-answer"
	switch {
	case len(q.Examples) > 0 && q.Exclusive:
		ch := &kit.Choice{}
		for _, ex := range q.Examples {
			ch.Add(ex.Value, ex.Value+" — "+ex.Help)
		}
		kit.SelectOf(c, nav, w.p.u.Dialogs(), k, w.vm.Provider().Description, ch, w.answer, func(v string) { w.answer = v })
	default:
		in := ui.TextInput(c.Key(k), &w.answer)
		if q.IsPassword {
			in = in.Password()
		}
		nav.Item(nav.Handle(k), in, nil, kit.TextField)
		if len(q.Examples) > 0 && !q.IsPassword {
			ui.Row(c).Wrap().Gap(6).Children(func() {
				for i, ex := range q.Examples {
					ek := [2]any{k, i}
					set := func() { w.answer = ex.Value }
					if nav.Item(nav.Handle(ek), ui.Button(c.Key(ek), ex.Value), set).Clicked() {
						set()
					}
				}
			})
		}
	}
	w.foot = func() {
		ui.Row(c).Gap(8).Children(func() {
			w.button(c, nav, "qcancel", i18n.L("Cancel"), false, false, w.close, kit.FocusSafe)
			w.button(c, nav, "qcontinue", i18n.L("Continue"), true, false, func() { w.reply(w.answer) })
		})
	}
}
