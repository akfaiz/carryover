package emulatorform

import (
	"runtime"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/lang"
	"fyne.io/fyne/v2/widget"

	"github.com/akfaiz/carryover/internal/launcher"
	"github.com/akfaiz/carryover/internal/saves/pathvar"
	"github.com/akfaiz/carryover/services"
	"github.com/akfaiz/carryover/ui/components"
	"github.com/akfaiz/carryover/ui/env"
	"github.com/akfaiz/carryover/ui/kit"
)

// Form adds or edits an emulator. It draws itself into Host, so it works on the
// Emulators page and in a dialog over the game editor alike.
type Form struct {
	u       env.Env
	vm      *ViewModel
	tokens  []pathvar.Token
	advItem *kit.AccordionSection

	// Host holds the form.
	Host     *fyne.Container
	onCancel func()
	onSaved  func(services.EmulatorView)
	// Relayout (if set) lays out what holds the form again after it was drawn
	// anew: its height changes (a type adds the data folder fields).
	Relayout func()
}

// New returns a form for s (the zero value adds an emulator). onCancel is the
// Cancel button; onSaved gets the saved emulator.
func New(u env.Env, s services.EmulatorSetup, tokens []pathvar.Token, onCancel func(), onSaved func(services.EmulatorView)) *Form {
	f := &Form{u: u, vm: NewViewModel(u.Emus(), s), tokens: tokens, Host: container.NewStack(), onCancel: onCancel, onSaved: onSaved}
	f.render()
	return f
}

// Dirty reports unsaved changes.
func (f *Form) Dirty() bool { return f.vm.Dirty() }

// Title is the heading above the form.
func (f *Form) Title() string {
	if f.vm.IsNew() {
		return lang.L("Add emulator")
	}
	return lang.L("Edit emulator")
}

func (f *Form) render() {
	f.Host.Objects = []fyne.CanvasObject{f.form()}
	f.Host.Refresh()
	if f.Relayout != nil {
		f.Relayout()
	}
}

func (f *Form) form() fyne.CanvasObject {
	vm := f.vm
	s := vm.Setup()
	name := widget.NewEntry()
	name.SetPlaceHolder(lang.L("Emulator name (e.g. Eden)"))
	name.SetText(s.Name)
	name.OnChanged = vm.SetName

	example := "/home/deck/Applications/eden.AppImage"
	switch runtime.GOOS {
	case "windows":
		example = `C:\Emulators\Eden\eden.exe`
	case "darwin":
		example = "/Applications/Eden.app"
	}
	exe := kit.MonoEntry(example, s.Exe, vm.SetExe)
	browse := widget.NewButton(lang.L("Browse…"), func() {
		kit.PickProgram(f.u.Window(), lang.L("Choose the emulator executable"), func(path string) {
			path = launcher.AppBundle(path) // a program picked inside a macOS .app
			vm.SetExe(path)
			exe.SetText(path)
			if vm.ExePicked(path) {
				f.render()
			}
		})
	})
	preset := vm.CurrentPreset()
	argsPlaceholder := "-f -g {rom}"
	if preset.Args != "" {
		argsPlaceholder = preset.Args
	}
	args := kit.MonoEntry(argsPlaceholder, s.Args, vm.SetArgs)
	wait := kit.MonoEntry("eden.exe", s.WaitProcess, vm.SetWaitProcess)
	envVars := kit.MonoMultiLine("", vm.EnvText(), 3, vm.SetEnvText)
	workDir := components.NewPathInput(f.u, "", nil, s.WorkDir, "", vm.SetWorkDir)
	if f.advItem != nil {
		vm.SetAdvanced(f.advItem.Open())
	}
	f.advItem = kit.NewAccordionSection(lang.L("Advanced"), kit.VBox(
		kit.Field(lang.L("Working directory (default: executable's folder)"), workDir),
		kit.Field(lang.L("Wait for process (when the exe is a launcher that exits immediately)"), wait),
		kit.Field(lang.L("Environment variables (KEY=value per line)"), envVars),
	), vm.Advanced())

	label := lang.L("Save")
	if vm.Saving() {
		label = lang.L("Saving…")
	}
	save := kit.Primary(label, f.save)
	kit.SetEnabled(save, !vm.Saving())
	c := kit.VBox(
		kit.Field(lang.L("Executable on this device"), kit.RowFill(exe, browse)),
		kit.Field(lang.L("Name"), name),
		f.typeField(),
		kit.Field(lang.L("Arguments"), args),
		f.argsHints(preset),
		f.dataFolderFields(preset),
		f.advItem,
	)
	if vm.ErrText() != "" {
		c.Add(kit.Colored(vm.ErrText(), widget.DangerImportance))
	}
	c.Add(kit.HBox(widget.NewButton(lang.L("Cancel"), f.onCancel), save))
	return kit.Card("", c)
}

// typeField asks for the emulator's type. Choosing one fills in the usual
// arguments (when none are set), and lets the game editor suggest save locations.
func (f *Form) typeField() fyne.CanvasObject {
	types := &kit.Choice{}
	types.Add("", lang.L("Not listed"))
	for _, pr := range f.vm.Presets() {
		types.Add(pr.ID, pr.Name)
	}
	kind := kit.SelectOf(types, f.vm.Setup().Preset, func(v string) {
		f.vm.SetPreset(v)
		f.render()
	})
	return kit.Field(lang.L("Emulator type (suggests the arguments and save locations)"), kit.Outlined(kind))
}

// argsHints explains {rom} and offers the type's usual arguments.
func (f *Form) argsHints(preset services.PresetView) fyne.CanvasObject {
	c := kit.VBox(kit.Muted(lang.L("{{.Token}} is replaced by the game's file or ID, which you set per game. Without it, the game file is added after the arguments.", map[string]any{"Token": "{rom}"})))
	if f.vm.OffersPresetArgs() {
		c.Add(kit.HBox(kit.Small(lang.L("Usual for this type: {{.Args}}", map[string]any{"Args": preset.Args})),
			kit.LowButton(lang.L("Use"), func() { f.vm.UsePresetArgs(); f.render() })))
	}
	if preset.ArgsNote != "" {
		c.Add(kit.Muted(preset.ArgsNote))
	}
	if preset.Args != "" {
		c.Add(kit.Muted(lang.L("Suggested arguments can differ between versions of the emulator; check its command line help.")))
	}
	return c
}

// dataFolderFields asks for the emulator's data folder, where its saves are.
func (f *Form) dataFolderFields(preset services.PresetView) fyne.CanvasObject {
	s := f.vm.Setup()
	if s.Preset == "" {
		return kit.VBox()
	}
	c := kit.VBox()
	placeholder := f.u.Emus().DefaultDataDir(s.Preset, s.Name)
	c.Add(kit.Field(lang.L("Data folder on this device: {{.Hint}}", map[string]any{"Hint": preset.RootHint}),
		components.NewPathInput(f.u, "", f.tokens, s.DataDir, placeholder, f.vm.SetDataDir)))
	if placeholder != "" {
		c.Add(kit.Muted(lang.L("Left empty, the usual folder shown above is used.")))
	}
	if !preset.PerGame {
		c.Add(kit.Muted(lang.L("This emulator keeps the saves of every game together, so all games share one save location.")))
	}
	return c
}

func (f *Form) save() {
	s := f.vm.BeginSave()
	f.render()
	kit.Async(func() (services.EmulatorView, error) { return f.vm.Save(s) }, func(ev services.EmulatorView, err error) {
		if !f.vm.FinishSave(err) {
			f.render()
			return
		}
		f.u.Toast(lang.L("Saved"), kit.ToastOK)
		f.onSaved(ev)
	})
}

// ShowDialog adds an emulator in a dialog, so a form open underneath (the game
// editor) keeps what was typed into it. onSaved gets the new one.
func ShowDialog(u env.Env, onSaved func(services.EmulatorView)) {
	kit.Async(func() ([]pathvar.Token, error) { return u.Lib().Tokens(""), nil }, func(tokens []pathvar.Token, _ error) {
		var d components.DialogHider
		var f *Form
		cancel := func() {
			if f.Dirty() {
				components.ConfirmDiscard(u, func() { d.Hide() })
				return
			}
			d.Hide()
		}
		f = New(u, services.EmulatorSetup{}, tokens, cancel,
			func(ev services.EmulatorView) {
				d.Hide()
				onSaved(ev)
			})
		scroll := container.NewVScroll(f.Host)
		f.Relayout = func() {
			// A new content object makes the scroll lay it out at its new height.
			scroll.Content = container.NewStack(f.Host)
			scroll.Refresh()
		}
		d = kit.ScrollDialog(u.Window(), f.Title(), scroll)
		kit.OnBack(u.Window(), cancel) // B asks before dropping what was typed, like Cancel
	})
}
