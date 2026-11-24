package emulatorform

import (
	"runtime"

	"github.com/egoist/mygo/ui"

	"github.com/akfaiz/carryover/internal/saves/pathvar"
	"github.com/akfaiz/carryover/services"
	"github.com/akfaiz/carryover/ui/components"
	"github.com/akfaiz/carryover/ui/env"
	"github.com/akfaiz/carryover/ui/i18n"
	"github.com/akfaiz/carryover/ui/kit"
)

// Form adds or edits an emulator. It is drawn by whoever holds it (the
// Emulators page, or a dialog over the game editor): View each frame. The
// ViewModel owns the content; the form keeps the text of each field and copies
// it into the ViewModel when it changes (push), and back after the ViewModel
// itself changed a field (pull).
type Form struct {
	u      env.Env
	vm     *ViewModel
	tokens []pathvar.Token

	name, exe, args, wait, envText string
	workDir, dataDir               *components.PathInput
	advanced                       bool

	onCancel func()
	onSaved  func(services.EmulatorView)
}

// New returns a form for s (the zero value adds an emulator). onCancel is the
// Cancel button; onSaved gets the saved emulator.
func New(u env.Env, s services.EmulatorSetup, tokens []pathvar.Token, onCancel func(), onSaved func(services.EmulatorView)) *Form {
	f := &Form{u: u, vm: NewViewModel(u.Emus(), s), tokens: tokens, onCancel: onCancel, onSaved: onSaved}
	f.workDir = components.NewPathInput(u, "", nil, s.WorkDir, "", f.vm.SetWorkDir)
	f.dataDir = components.NewPathInput(u, "", tokens, s.DataDir, f.vm.DefaultDataDir(), f.vm.SetDataDir)
	f.pull()
	kit.Async(func() ([]services.FlatpakEmulator, error) { return u.Emus().Flatpaks(), nil },
		func(l []services.FlatpakEmulator, _ error) {
			if len(l) > 0 {
				f.vm.SetFlatpaks(l)
			}
		})
	return f
}

// Dirty reports unsaved changes.
func (f *Form) Dirty() bool { f.push(); return f.vm.Dirty() }

// Title is the heading above the form.
func (f *Form) Title() string {
	if f.vm.IsNew() {
		return i18n.L("Add emulator")
	}
	return i18n.L("Edit emulator")
}

// pull copies the ViewModel's fields into the form's, after the ViewModel
// changed them itself (a type chosen, a Flatpak, a program picked).
func (f *Form) pull() {
	s := f.vm.Setup()
	f.name, f.exe, f.args, f.wait, f.envText = s.Name, s.Exe, s.Args, s.WaitProcess, f.vm.EnvText()
	f.advanced = f.vm.Advanced()
	f.workDir.SetText(s.WorkDir)
	f.dataDir.Placeholder = f.vm.DefaultDataDir()
	f.dataDir.SetText(s.DataDir)
}

// push copies what was typed into the ViewModel.
func (f *Form) push() {
	s := f.vm.Setup()
	if f.name != s.Name {
		f.vm.SetName(f.name)
	}
	if f.exe != s.Exe {
		f.vm.SetExe(f.exe)
	}
	if f.args != s.Args {
		f.vm.SetArgs(f.args)
	}
	if f.wait != s.WaitProcess {
		f.vm.SetWaitProcess(f.wait)
	}
	if f.envText != f.vm.EnvText() {
		f.vm.SetEnvText(f.envText)
	}
	f.vm.SetAdvanced(f.advanced)
}

// View draws the form.
func (f *Form) View(c *ui.Context) ui.Element {
	f.push()
	vm, nav := f.vm, f.u.Navigator()
	preset := vm.CurrentPreset()

	example := "/home/deck/Applications/eden.AppImage"
	switch runtime.GOOS {
	case "windows":
		example = `C:\Emulators\Eden\eden.exe`
	case "darwin":
		example = "/Applications/Eden.app"
	}
	argsPlaceholder := "-f -g {rom}"
	if preset.Args != "" {
		argsPlaceholder = preset.Args
	}
	field := func(key string, v *string, placeholder string) ui.Element {
		return kit.Field(c, nav, [2]any{"emu", key}, v, placeholder).Font("monospace")
	}

	return kit.Card(c, "", func() {
		kit.Labeled(c, i18n.L("Executable on this device"), func() {
			ui.Row(c).Gap(kit.S2).Children(func() {
				field("exe", &f.exe, example).Grow(1)
				kit.Btn(c, nav, "emu-browse", kit.BtnSecondary, kit.IconFolderOpen, i18n.L("Browse…"), f.browse)
			})
		})
		f.flatpakField(c)
		kit.Labeled(c, i18n.L("Name"), func() { field("name", &f.name, i18n.L("Emulator name (e.g. Eden)")) })
		f.typeField(c)
		kit.Labeled(c, i18n.L("Arguments"), func() { field("args", &f.args, argsPlaceholder) })
		f.argsHints(c, preset)
		f.dataFolderFields(c, preset)

		kit.Disclosure(c, nav, "emu-adv", i18n.L("Advanced"), &f.advanced, func() {
			kit.Labeled(c, i18n.L("Working directory (default: executable's folder)"), func() { f.workDir.View(c, "workdir") })
			kit.Labeled(c, i18n.L("Wait for process (when the exe is a launcher that exits immediately)"), func() { field("wait", &f.wait, "eden.exe") })
			kit.Labeled(c, i18n.L("Environment variables (KEY=value per line)"), func() {
				kit.TextArea(c, nav, [2]any{"emu", "env"}, &f.envText, "", 80).Label(i18n.L("Environment variables (KEY=value per line)"))
			})
		})

		if vm.ErrText() != "" {
			kit.Danger(c, vm.ErrText())
		}
		kit.Footer(c, func() {
			kit.Btn(c, nav, "emu-cancel", kit.BtnSecondary, nil, i18n.L("Cancel"), f.onCancel, kit.BtnItem(kit.FocusSafe))
			label := i18n.L("Save")
			if vm.Saving() {
				label = i18n.L("Saving…")
			}
			kit.Btn(c, nav, "emu-save", kit.BtnPrimary, nil, label, f.save, kit.BtnDisabled(vm.Saving()))
		})
	})
}

func (f *Form) browse() {
	components.BrowseFrom(f.u, "", f.vm.Setup().Exe, func(dir string) {
		kit.PickProgram(f.u.Window(), i18n.L("Choose the emulator executable"), dir, func(path string) {
			path = services.AppBundle(path) // a program picked inside a macOS .app
			f.vm.SetExe(path)
			f.vm.ExePicked(path)
			f.pull()
		})
	})
}

// flatpakField offers the emulators installed with Flatpak (Linux): choosing
// one sets the executable to its launcher and fills in its name, type,
// arguments and data folder. Nothing is shown when there are none.
func (f *Form) flatpakField(c *ui.Context) {
	l := f.vm.Flatpaks()
	if len(l) == 0 {
		return
	}
	apps := &kit.Choice{}
	apps.Add("", i18n.L("Choose an installed Flatpak…"))
	for _, a := range l {
		apps.Add(a.AppID, a.Name+" ("+a.AppID+")")
	}
	kit.Labeled(c, i18n.L("Or an emulator installed with Flatpak"), func() {
		kit.SelectOf(c, f.u.Navigator(), "flatpak", i18n.L("Flatpak"), apps, f.vm.CurrentFlatpak(), func(id string) {
			if id != "" && id != f.vm.CurrentFlatpak() {
				f.vm.UseFlatpak(id)
				f.pull()
			}
		})
	})
}

// typeField asks for the emulator's type. Choosing one fills in the usual
// arguments (when none are set), and lets the game editor suggest save locations.
func (f *Form) typeField(c *ui.Context) {
	types := &kit.Choice{}
	types.Add("", i18n.L("Not listed"))
	for _, pr := range f.vm.Presets() {
		types.Add(pr.ID, pr.Name)
	}
	kit.Labeled(c, i18n.L("Emulator type (suggests the arguments and save locations)"), func() {
		kit.SelectOf(c, f.u.Navigator(), "type", i18n.L("Emulator type"), types, f.vm.Setup().Preset, func(v string) {
			f.vm.SetPreset(v)
			f.pull()
		})
	})
}

// argsHints explains {rom} and offers the type's usual arguments.
func (f *Form) argsHints(c *ui.Context, preset services.PresetView) {
	nav := f.u.Navigator()
	kit.Muted(c, i18n.L("{{.Token}} is replaced by the game's file or ID, which you set per game. Without it, the game file is added after the arguments.", map[string]any{"Token": "{rom}"}))
	if f.vm.OffersPresetArgs() {
		ui.Row(c).Gap(kit.S2).AlignItems(ui.Center).Children(func() {
			kit.Small(c, i18n.L("Usual for this type: {{.Args}}", map[string]any{"Args": preset.Args}))
			kit.Btn(c, nav, "emu-use-args", kit.BtnSecondary, nil, i18n.L("Use"), func() { f.vm.UsePresetArgs(); f.pull() })
		})
	}
	if preset.ArgsNote != "" {
		kit.Muted(c, preset.ArgsNote)
	}
	if preset.Args != "" {
		kit.Muted(c, i18n.L("Suggested arguments can differ between versions of the emulator; check its command line help."))
	}
}

// dataFolderFields asks for the emulator's data folder, where its saves are.
func (f *Form) dataFolderFields(c *ui.Context, preset services.PresetView) {
	s := f.vm.Setup()
	if s.Preset == "" {
		return
	}
	f.dataDir.Placeholder = f.vm.DefaultDataDir()
	kit.Labeled(c, i18n.L("Data folder on this device: {{.Hint}}", map[string]any{"Hint": preset.RootHint}), func() { f.dataDir.View(c, "datadir") })
	if f.dataDir.Placeholder != "" {
		kit.Muted(c, i18n.L("Left empty, the usual folder shown above is used."))
	}
	if !preset.PerGame {
		kit.Muted(c, i18n.L("This emulator keeps the saves of every game together, so all games share one save location."))
	}
}

func (f *Form) save() {
	f.push()
	s := f.vm.BeginSave()
	kit.Async(func() (services.EmulatorView, error) { return f.vm.Save(s) }, func(ev services.EmulatorView, err error) {
		if !f.vm.FinishSave(err) {
			return
		}
		f.u.Toast(i18n.L("Saved"), kit.ToastOK)
		f.onSaved(ev)
	})
}

// ShowDialog adds an emulator in a dialog, so a form open underneath (the game
// editor) keeps what was typed into it. onSaved gets the new one.
func ShowDialog(u env.Env, onSaved func(services.EmulatorView)) {
	kit.Async(func() ([]pathvar.Token, error) { return u.Lib().Tokens(""), nil }, func(tokens []pathvar.Token, _ error) {
		var d *kit.Dialog
		var f *Form
		cancel := func() {
			if f.Dirty() {
				components.ConfirmDiscard(u, func() { d.Close() })
				return
			}
			d.Close()
		}
		f = New(u, services.EmulatorSetup{}, tokens, cancel, func(ev services.EmulatorView) {
			d.Close()
			onSaved(ev)
		})
		d = u.Dialogs().Show(&kit.Dialog{Title: f.Title(), Wide: true, Back: cancel, Body: func(c *ui.Context, _ *kit.Navigator) {
			kit.ScrollBox(c, 560, 0, func() { f.View(c) })
		}})
	})
}
