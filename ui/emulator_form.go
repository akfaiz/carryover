package ui

import (
	"encoding/json"
	"runtime"
	"slices"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/lang"
	"fyne.io/fyne/v2/widget"

	"github.com/akfaiz/carryover/internal/saves/pathvar"
	"github.com/akfaiz/carryover/internal/ui/kit"
	"github.com/akfaiz/carryover/services"
)

// emulatorForm adds or edits an emulator: its name, type, executable on this
// device, arguments and data folder. It draws itself into host, so it works
// on the Emulators page and in a dialog over the game editor alike.
type emulatorForm struct {
	u        *UI
	s        *services.EmulatorSetup
	tokens   []pathvar.Token
	envText  string
	advanced bool
	advItem  *kit.AccordionSection
	saving   bool
	errText  string
	orig     string // what the form started with, to tell whether it changed

	host     *fyne.Container
	onCancel func()
	onSaved  func(services.EmulatorView)
	// relayout (if set) lays out what holds the form again after it was
	// drawn anew: its height changes (a type adds the data folder fields).
	relayout func()
}

func newEmulatorForm(u *UI, s services.EmulatorSetup, tokens []pathvar.Token, onCancel func(), onSaved func(services.EmulatorView)) *emulatorForm {
	f := &emulatorForm{u: u, s: &s, tokens: tokens, host: container.NewStack(), onCancel: onCancel, onSaved: onSaved}
	var env []string
	for k, v := range s.Env {
		env = append(env, k+"="+v)
	}
	slices.Sort(env)
	f.envText = strings.Join(env, "\n")
	f.advanced = s.WorkDir != "" || s.WaitProcess != "" || f.envText != ""
	f.orig = f.snapshot()
	f.render()
	return f
}

// snapshot is the form's content as text, to compare with what it started with.
func (f *emulatorForm) snapshot() string {
	b, _ := json.Marshal(struct {
		S   services.EmulatorSetup
		Env string
	}{*f.s, f.envText})
	return string(b)
}

// dirty reports unsaved changes.
func (f *emulatorForm) dirty() bool { return !f.saving && f.snapshot() != f.orig }

func (f *emulatorForm) title() string {
	if f.s.ID == "" {
		return lang.L("Add emulator")
	}
	return lang.L("Edit emulator")
}

func (f *emulatorForm) render() {
	f.host.Objects = []fyne.CanvasObject{f.form()}
	f.host.Refresh()
	if f.relayout != nil {
		f.relayout()
	}
}

func (f *emulatorForm) form() fyne.CanvasObject {
	s := f.s
	name := widget.NewEntry()
	name.SetPlaceHolder(lang.L("Emulator name (e.g. Eden)"))
	name.SetText(s.Name)
	name.OnChanged = func(v string) { s.Name = v }

	example := "/home/deck/Applications/eden.AppImage"
	if runtime.GOOS == "windows" {
		example = `C:\Emulators\Eden\eden.exe`
	}
	exe := kit.MonoEntry(example, s.Exe, func(v string) { s.Exe = v })
	browse := widget.NewButton(lang.L("Browse…"), func() {
		kit.PickFile(f.u.win, lang.L("Choose the emulator executable"), nil, func(path string) {
			s.Exe = path
			exe.SetText(path)
			f.exePicked(path)
		})
	})
	preset := f.preset(s.Preset)
	argsPlaceholder := "-f -g {rom}"
	if preset.Args != "" {
		argsPlaceholder = preset.Args
	}
	args := kit.MonoEntry(argsPlaceholder, s.Args, func(v string) { s.Args = v })
	wait := kit.MonoEntry("eden.exe", s.WaitProcess, func(v string) { s.WaitProcess = v })
	env := kit.MonoMultiLine("", f.envText, 3, func(v string) { f.envText = v })
	workDir := newPathInput(f.u, "", nil, s.WorkDir, "", func(v string) { s.WorkDir = v })
	if f.advItem != nil {
		f.advanced = f.advItem.Open()
	}
	f.advItem = kit.NewAccordionSection(lang.L("Advanced"), kit.VBox(
		kit.Field(lang.L("Working directory (default: executable's folder)"), workDir),
		kit.Field(lang.L("Wait for process (when the exe is a launcher that exits immediately)"), wait),
		kit.Field(lang.L("Environment variables (KEY=value per line)"), env),
	), f.advanced)

	label := lang.L("Save")
	if f.saving {
		label = lang.L("Saving…")
	}
	save := kit.Primary(label, f.save)
	kit.SetEnabled(save, !f.saving)
	c := kit.VBox(
		kit.Field(lang.L("Executable on this device"), kit.RowFill(exe, browse)),
		kit.Field(lang.L("Name"), name),
		f.typeField(),
		kit.Field(lang.L("Arguments"), args),
		f.argsHints(preset),
		f.dataFolderFields(preset),
		f.advItem,
	)
	if f.errText != "" {
		c.Add(kit.Colored(f.errText, widget.DangerImportance))
	}
	c.Add(kit.HBox(widget.NewButton(lang.L("Cancel"), f.onCancel), save))
	return kit.Card("", c)
}

// exePicked proposes the emulator's name, type and usual arguments from the
// executable or AppImage just chosen, for whatever is still empty.
func (f *emulatorForm) exePicked(path string) {
	s := f.s
	g := f.u.emus.GuessEmulator(path)
	changed := false
	if strings.TrimSpace(s.Name) == "" && g.Name != "" {
		s.Name, changed = g.Name, true
	}
	if s.Preset == "" && g.Preset != "" {
		s.Preset, changed = g.Preset, true
		if s.Args == "" {
			s.Args = f.preset(g.Preset).Args
		}
	}
	if changed {
		f.render()
	}
}

// preset returns the emulator type with an id (the zero value if none).
func (f *emulatorForm) preset(id string) services.PresetView {
	for _, pr := range f.u.emus.Presets() {
		if pr.ID == id {
			return pr
		}
	}
	return services.PresetView{}
}

// typeField asks for the emulator's type. Choosing one fills in the usual
// arguments (when none are set), and lets the game editor suggest save locations.
func (f *emulatorForm) typeField() fyne.CanvasObject {
	s := f.s
	types := &kit.Choice{}
	types.Add("", lang.L("Not listed"))
	for _, pr := range f.u.emus.Presets() {
		types.Add(pr.ID, pr.Name)
	}
	kind := kit.SelectOf(types, s.Preset, func(v string) {
		old := f.preset(s.Preset)
		s.Preset = v
		// Replace the arguments only if the user hasn't written their own.
		if s.Args == "" || s.Args == old.Args {
			s.Args = f.preset(v).Args
		}
		f.render()
	})
	return kit.Field(lang.L("Emulator type (suggests the arguments and save locations)"), kind)
}

// argsHints explains {rom} and offers the type's usual arguments.
func (f *emulatorForm) argsHints(preset services.PresetView) fyne.CanvasObject {
	s := f.s
	c := kit.VBox(kit.Muted(lang.L("{{.Token}} is replaced by the game's file or ID, which you set per game. Without it, the game file is added after the arguments.", map[string]any{"Token": "{rom}"})))
	if preset.Args != "" && s.Args != preset.Args {
		c.Add(kit.HBox(kit.Small(lang.L("Usual for this type: {{.Args}}", map[string]any{"Args": preset.Args})),
			kit.LowButton(lang.L("Use"), func() { s.Args = preset.Args; f.render() })))
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
func (f *emulatorForm) dataFolderFields(preset services.PresetView) fyne.CanvasObject {
	s := f.s
	if s.Preset == "" {
		return kit.VBox()
	}
	c := kit.VBox()
	placeholder := f.u.emus.DefaultDataDir(s.Preset, s.Name)
	c.Add(kit.Field(lang.L("Data folder on this device: {{.Hint}}", map[string]any{"Hint": preset.RootHint}),
		newPathInput(f.u, "", f.tokens, s.DataDir, placeholder, func(v string) { s.DataDir = v })))
	if placeholder != "" {
		c.Add(kit.Muted(lang.L("Left empty, the usual folder shown above is used.")))
	}
	if !preset.PerGame {
		c.Add(kit.Muted(lang.L("This emulator keeps the saves of every game together, so all games share one save location.")))
	}
	return c
}

func (f *emulatorForm) save() {
	s := *f.s
	s.Env = map[string]string{}
	for l := range strings.SplitSeq(f.envText, "\n") {
		if i := strings.Index(l, "="); i > 0 {
			s.Env[strings.TrimSpace(l[:i])] = l[i+1:]
		}
	}
	f.saving, f.errText = true, ""
	f.render()
	kit.Async(func() (services.EmulatorView, error) { return f.u.emus.Save(s) }, func(ev services.EmulatorView, err error) {
		f.saving = false
		if err != nil {
			f.errText = err.Error()
			f.render()
			return
		}
		f.orig = f.snapshot()
		f.u.toast(lang.L("Saved"), kit.ToastOK)
		f.onSaved(ev)
	})
}

// showEmulatorDialog adds an emulator in a dialog, so a form open underneath
// (the game editor) keeps what was typed into it. onSaved gets the new one.
func showEmulatorDialog(u *UI, onSaved func(services.EmulatorView)) {
	kit.Async(func() ([]pathvar.Token, error) { return u.lib.Tokens(""), nil }, func(tokens []pathvar.Token, _ error) {
		var d dialogHider
		var f *emulatorForm
		f = newEmulatorForm(u, services.EmulatorSetup{}, tokens,
			func() {
				if f.dirty() {
					confirmDiscard(u, func() { d.Hide() })
					return
				}
				d.Hide()
			},
			func(ev services.EmulatorView) {
				d.Hide()
				onSaved(ev)
			})
		scroll := container.NewVScroll(f.host)
		f.relayout = func() {
			// A new content object makes the scroll lay it out at its new height.
			scroll.Content = container.NewStack(f.host)
			scroll.Refresh()
		}
		d = kit.ScrollDialog(u.win, f.title(), scroll)
	})
}
