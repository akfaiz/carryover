// Package emulatorform is the form that adds or edits an emulator: its name,
// type, executable on this device, arguments and data folder. It is shared by
// the Emulators page and the game editor (which adds an emulator in a dialog),
// so it lives beside the pages, not in one of them.
//
// ViewModel holds the form's content and its rules and is tested without
// the toolkit; Form draws it.
package emulatorform

import (
	"encoding/json"
	"slices"
	"strings"

	"github.com/akfaiz/carryover/services"
)

// Backend is what the form needs of the emulator service.
type Backend interface {
	Presets() []services.PresetView
	GuessEmulator(exe string) services.EmuGuess
	DefaultDataDir(preset, name, exe, args string) string
	Save(services.EmulatorSetup) (services.EmulatorView, error)
}

// ParseEnv reads KEY=value lines. Lines without a key are ignored.
func ParseEnv(text string) map[string]string {
	env := map[string]string{}
	for l := range strings.SplitSeq(text, "\n") {
		if i := strings.Index(l, "="); i > 0 {
			env[strings.TrimSpace(l[:i])] = l[i+1:]
		}
	}
	return env
}

// FormatEnv writes variables as KEY=value lines, sorted by key.
func FormatEnv(env map[string]string) string {
	lines := make([]string, 0, len(env))
	for k, v := range env {
		lines = append(lines, k+"="+v)
	}
	slices.Sort(lines)
	return strings.Join(lines, "\n")
}

// ViewModel is the form's content and rules.
type ViewModel struct {
	be       Backend
	s        services.EmulatorSetup
	envText  string
	advanced bool
	saving   bool
	errText  string
	errMsg   *services.Message
	orig     string // what the form started with, to tell whether it changed
	flatpaks []services.FlatpakEmulator
}

// NewViewModel returns a form for s: an existing emulator, or the zero value
// for a new one.
func NewViewModel(be Backend, s services.EmulatorSetup) *ViewModel {
	vm := &ViewModel{be: be, s: s, envText: FormatEnv(s.Env)}
	vm.advanced = s.WorkDir != "" || s.WaitProcess != "" || vm.envText != ""
	vm.orig = vm.snapshot()
	return vm
}

// snapshot is the form's content as text, to compare with what it started with.
func (vm *ViewModel) snapshot() string {
	b, _ := json.Marshal(struct {
		S   services.EmulatorSetup
		Env string
	}{vm.s, vm.envText})
	return string(b)
}

// Setup is what the form holds now.
func (vm *ViewModel) Setup() services.EmulatorSetup { return vm.s }

// IsNew reports whether the form adds an emulator, rather than editing one.
func (vm *ViewModel) IsNew() bool { return vm.s.ID == "" }

// Dirty reports unsaved changes.
func (vm *ViewModel) Dirty() bool { return !vm.saving && vm.snapshot() != vm.orig }

func (vm *ViewModel) SetName(v string)        { vm.s.Name = v }
func (vm *ViewModel) SetExe(v string)         { vm.s.Exe = v }
func (vm *ViewModel) SetArgs(v string)        { vm.s.Args = v }
func (vm *ViewModel) SetWaitProcess(v string) { vm.s.WaitProcess = v }
func (vm *ViewModel) SetWorkDir(v string)     { vm.s.WorkDir = v }
func (vm *ViewModel) SetDataDir(v string)     { vm.s.DataDir = v }

// EnvText is the environment variables as the user types them.
func (vm *ViewModel) EnvText() string     { return vm.envText }
func (vm *ViewModel) SetEnvText(v string) { vm.envText = v }

// Advanced reports whether the "Advanced" section is open; it starts open when
// something in it is set.
func (vm *ViewModel) Advanced() bool     { return vm.advanced }
func (vm *ViewModel) SetAdvanced(b bool) { vm.advanced = b }

// Presets are the emulator types to choose from.
func (vm *ViewModel) Presets() []services.PresetView { return vm.be.Presets() }

// Preset returns the emulator type with an id (the zero value if none).
func (vm *ViewModel) Preset(id string) services.PresetView {
	for _, pr := range vm.be.Presets() {
		if pr.ID == id {
			return pr
		}
	}
	return services.PresetView{}
}

// CurrentPreset is the chosen emulator type (the zero value if none).
func (vm *ViewModel) CurrentPreset() services.PresetView { return vm.Preset(vm.s.Preset) }

// SetPreset chooses the emulator type. Its usual arguments replace the current
// ones only if the user hasn't written their own, and so does its usual data
// folder.
func (vm *ViewModel) SetPreset(id string) {
	old := vm.CurrentPreset()
	if vm.s.DataDir != "" && vm.s.DataDir == vm.DefaultDataDir() {
		vm.s.DataDir = "" // the old type's usual folder: the new type's is shown instead
	}
	vm.s.Preset = id
	if vm.s.Args == "" || vm.s.Args == old.Args {
		vm.s.Args = vm.Preset(id).Args
	}
}

// DefaultDataDir is the data folder used while none is set: the type's usual
// one, inside the app's own home for a Flatpak ("" if there is none).
func (vm *ViewModel) DefaultDataDir() string {
	return vm.be.DefaultDataDir(vm.s.Preset, vm.s.Name, vm.s.Exe, vm.s.Args)
}

// SetFlatpaks sets the emulators installed with Flatpak, to choose from.
func (vm *ViewModel) SetFlatpaks(l []services.FlatpakEmulator) { vm.flatpaks = l }

// Flatpaks are the emulators installed with Flatpak on this device.
func (vm *ViewModel) Flatpaks() []services.FlatpakEmulator { return vm.flatpaks }

// CurrentFlatpak is the app id of the Flatpak the executable starts ("" if it
// isn't one of Flatpaks).
func (vm *ViewModel) CurrentFlatpak() string {
	for _, f := range vm.flatpaks {
		if f.Exe == strings.TrimSpace(vm.s.Exe) {
			return f.AppID
		}
	}
	return ""
}

// UseFlatpak makes the executable the launcher of an installed Flatpak and
// proposes its name, type and usual arguments for whatever is still empty.
func (vm *ViewModel) UseFlatpak(appID string) {
	for _, f := range vm.flatpaks {
		if f.AppID == appID {
			vm.s.Exe = f.Exe
			vm.propose(services.EmuGuess{Name: f.Name, Preset: f.Preset})
			return
		}
	}
}

// OffersPresetArgs reports whether the type's usual arguments differ from the
// ones set, so the form offers to use them.
func (vm *ViewModel) OffersPresetArgs() bool {
	p := vm.CurrentPreset()
	return p.Args != "" && vm.s.Args != p.Args
}

// UsePresetArgs sets the type's usual arguments.
func (vm *ViewModel) UsePresetArgs() { vm.s.Args = vm.CurrentPreset().Args }

// ExePicked proposes the emulator's name, type and usual arguments from the
// executable or AppImage just chosen, for whatever is still empty. It reports
// whether anything changed.
func (vm *ViewModel) ExePicked(path string) (changed bool) {
	return vm.propose(vm.be.GuessEmulator(path))
}

// propose fills in what is still empty from a guess and reports whether
// anything changed.
func (vm *ViewModel) propose(g services.EmuGuess) (changed bool) {
	if strings.TrimSpace(vm.s.Name) == "" && g.Name != "" {
		vm.s.Name, changed = g.Name, true
	}
	if vm.s.Preset == "" && g.Preset != "" {
		vm.s.Preset, changed = g.Preset, true
		if vm.s.Args == "" {
			vm.s.Args = vm.Preset(g.Preset).Args
		}
	}
	return changed
}

// Saving reports whether a save runs; the form waits.
func (vm *ViewModel) Saving() bool { return vm.saving }

// ErrText is why the last save failed, "" if it didn't.
func (vm *ViewModel) ErrText() string { return vm.errText }

// ErrMsg is ErrText to translate, if it is the core's.
func (vm *ViewModel) ErrMsg() *services.Message { return vm.errMsg }

// BeginSave marks the form as saving and returns what to save.
func (vm *ViewModel) BeginSave() services.EmulatorSetup {
	s := vm.s
	s.Env = ParseEnv(vm.envText)
	vm.saving, vm.errText, vm.errMsg = true, "", nil
	return s
}

// Save sends a setup from BeginSave to the service (off the UI thread).
func (vm *ViewModel) Save(s services.EmulatorSetup) (services.EmulatorView, error) {
	return vm.be.Save(s)
}

// FinishSave records the result of a save. It reports whether it worked; the
// form then counts as unchanged.
func (vm *ViewModel) FinishSave(err error) bool {
	vm.saving = false
	if err != nil {
		vm.errText, vm.errMsg = services.ErrorText(err)
		return false
	}
	vm.orig = vm.snapshot()
	return true
}
