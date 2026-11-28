package emulatorform

import (
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/akfaiz/carryover/services"
)

type fakeBackend struct {
	presets []services.PresetView
	guess   services.EmuGuess
	guessed []string
	saved   []services.EmulatorSetup
	saveErr error
}

func (f *fakeBackend) Presets() []services.PresetView { return f.presets }
func (f *fakeBackend) GuessEmulator(exe string) services.EmuGuess {
	f.guessed = append(f.guessed, exe)
	return f.guess
}

// DefaultDataDir is "<preset>-dir", or "<preset>-flatpak" for an exe under
// /flatpak/ ("" without a type).
func (f *fakeBackend) DefaultDataDir(preset, _, exe, _ string) string {
	switch {
	case preset == "":
		return ""
	case strings.Contains(exe, "/flatpak/"):
		return preset + "-flatpak"
	}
	return preset + "-dir"
}
func (f *fakeBackend) Save(s services.EmulatorSetup) (services.EmulatorView, error) {
	f.saved = append(f.saved, s)
	return services.EmulatorView{ID: "new", Name: s.Name}, f.saveErr
}

func backend() *fakeBackend {
	return &fakeBackend{presets: []services.PresetView{
		{ID: "eden", Name: "Eden", Args: "-f -g {rom}"},
		{ID: "ryujinx", Name: "Ryujinx", Args: "{rom}"},
	}}
}

func TestParseAndFormatEnv(t *testing.T) {
	env := ParseEnv("B=2\nA = 1\n=skipped\nno equals\nC=x=y\n")
	assert.Equal(t, map[string]string{"B": "2", "A": " 1", "C": "x=y"}, env, "the key is trimmed, the value kept as typed")
	assert.Equal(t, "A=1\nB=2", FormatEnv(map[string]string{"B": "2", "A": "1"}), "sorted")
	assert.Empty(t, FormatEnv(nil))
	assert.Empty(t, ParseEnv(""))
}

func TestNewFormAndEditForm(t *testing.T) {
	vm := NewViewModel(backend(), services.EmulatorSetup{})
	assert.True(t, vm.IsNew())
	assert.False(t, vm.Advanced())
	assert.False(t, vm.Dirty())

	vm = NewViewModel(backend(), services.EmulatorSetup{ID: "e1", Name: "Eden", Env: map[string]string{"K": "v"}})
	assert.False(t, vm.IsNew())
	assert.Equal(t, "K=v", vm.EnvText())
	assert.True(t, vm.Advanced(), "something in Advanced is set: it starts open")

	assert.True(t, NewViewModel(backend(), services.EmulatorSetup{WorkDir: "/x"}).Advanced())
	assert.True(t, NewViewModel(backend(), services.EmulatorSetup{WaitProcess: "eden.exe"}).Advanced())
}

func TestDirtyTracksEveryField(t *testing.T) {
	for name, change := range map[string]func(*ViewModel){
		"name":    func(vm *ViewModel) { vm.SetName("x") },
		"exe":     func(vm *ViewModel) { vm.SetExe("x") },
		"args":    func(vm *ViewModel) { vm.SetArgs("x") },
		"wait":    func(vm *ViewModel) { vm.SetWaitProcess("x") },
		"workdir": func(vm *ViewModel) { vm.SetWorkDir("x") },
		"datadir": func(vm *ViewModel) { vm.SetDataDir("x") },
		"env":     func(vm *ViewModel) { vm.SetEnvText("A=1") },
		"preset":  func(vm *ViewModel) { vm.SetPreset("eden") },
	} {
		vm := NewViewModel(backend(), services.EmulatorSetup{})
		change(vm)
		assert.True(t, vm.Dirty(), name)
	}
}

func TestSetPresetFillsTheUsualArgumentsOnlyWhenTheUserHasNone(t *testing.T) {
	vm := NewViewModel(backend(), services.EmulatorSetup{})
	vm.SetPreset("eden")
	assert.Equal(t, "-f -g {rom}", vm.Setup().Args, "empty arguments take the type's")

	vm.SetPreset("ryujinx")
	assert.Equal(t, "{rom}", vm.Setup().Args, "still the previous type's own: replaced")

	vm.SetArgs("--my-flag {rom}")
	vm.SetPreset("eden")
	assert.Equal(t, "--my-flag {rom}", vm.Setup().Args, "the user's own are kept")

	vm.SetPreset("")
	assert.Equal(t, "--my-flag {rom}", vm.Setup().Args, "even for a type that isn't listed")
}

func TestOfferingAndUsingTheUsualArguments(t *testing.T) {
	vm := NewViewModel(backend(), services.EmulatorSetup{Preset: "eden", Args: "--mine"})
	assert.True(t, vm.OffersPresetArgs())
	vm.UsePresetArgs()
	assert.Equal(t, "-f -g {rom}", vm.Setup().Args)
	assert.False(t, vm.OffersPresetArgs(), "nothing to offer once they are set")

	assert.False(t, NewViewModel(backend(), services.EmulatorSetup{Args: "x"}).OffersPresetArgs(), "no type, no offer")
}

func TestExePickedProposesWhatIsStillEmpty(t *testing.T) {
	be := backend()
	be.guess = services.EmuGuess{Name: "Eden", Preset: "eden"}
	vm := NewViewModel(be, services.EmulatorSetup{})

	assert.True(t, vm.ExePicked("/apps/eden.AppImage"))
	s := vm.Setup()
	assert.Equal(t, "Eden", s.Name)
	assert.Equal(t, "eden", s.Preset)
	assert.Equal(t, "-f -g {rom}", s.Args)
	assert.Equal(t, []string{"/apps/eden.AppImage"}, be.guessed)
}

func TestExePickedKeepsWhatTheUserWrote(t *testing.T) {
	be := backend()
	be.guess = services.EmuGuess{Name: "Eden", Preset: "eden"}
	vm := NewViewModel(be, services.EmulatorSetup{Name: "My Switch", Preset: "ryujinx", Args: "{rom}"})

	assert.False(t, vm.ExePicked("/apps/eden.AppImage"))
	s := vm.Setup()
	assert.Equal(t, "My Switch", s.Name)
	assert.Equal(t, "ryujinx", s.Preset)
	assert.Equal(t, "{rom}", s.Args)

	// A blank name counts as empty; arguments the user already has stay.
	vm = NewViewModel(be, services.EmulatorSetup{Name: "  ", Args: "--mine"})
	assert.True(t, vm.ExePicked("/apps/eden.AppImage"))
	assert.Equal(t, "Eden", vm.Setup().Name)
	assert.Equal(t, "--mine", vm.Setup().Args)

	be.guess = services.EmuGuess{}
	assert.False(t, NewViewModel(be, services.EmulatorSetup{}).ExePicked("/x"), "nothing recognised")
}

func TestUseFlatpak(t *testing.T) {
	const exe = "/var/lib/flatpak/exports/bin/org.ryujinx.Ryujinx"
	vm := NewViewModel(backend(), services.EmulatorSetup{})
	assert.Empty(t, vm.CurrentFlatpak())
	vm.SetFlatpaks([]services.FlatpakEmulator{
		{AppID: "org.ryujinx.Ryujinx", Name: "Ryujinx", Preset: "ryujinx", Exe: exe},
	})
	assert.Len(t, vm.Flatpaks(), 1)

	vm.UseFlatpak("org.no.Such")
	assert.Empty(t, vm.Setup().Exe, "an unknown app changes nothing")

	vm.UseFlatpak("org.ryujinx.Ryujinx")
	s := vm.Setup()
	assert.Equal(t, exe, s.Exe, "its launcher is the executable")
	assert.Equal(t, "Ryujinx", s.Name)
	assert.Equal(t, "ryujinx", s.Preset)
	assert.Equal(t, "{rom}", s.Args)
	assert.Equal(t, "org.ryujinx.Ryujinx", vm.CurrentFlatpak())
	assert.Equal(t, "ryujinx-flatpak", vm.DefaultDataDir(), "the data folder inside the Flatpak's home")
	assert.True(t, vm.Dirty())

	// What the user already wrote stays.
	vm = NewViewModel(backend(), services.EmulatorSetup{Name: "My Switch", Preset: "eden", Args: "--mine"})
	vm.SetFlatpaks([]services.FlatpakEmulator{{AppID: "org.ryujinx.Ryujinx", Name: "Ryujinx", Preset: "ryujinx", Exe: exe}})
	vm.UseFlatpak("org.ryujinx.Ryujinx")
	assert.Equal(t, services.EmulatorSetup{Name: "My Switch", Exe: exe, Preset: "eden", Args: "--mine"}, vm.Setup())
}

func TestSetPresetReplacesTheUsualDataFolderOnly(t *testing.T) {
	vm := NewViewModel(backend(), services.EmulatorSetup{Preset: "eden", DataDir: "eden-dir"})
	vm.SetPreset("ryujinx")
	assert.Empty(t, vm.Setup().DataDir, "the old type's usual folder goes; the new one's is shown")
	assert.Equal(t, "ryujinx-dir", vm.DefaultDataDir())

	vm = NewViewModel(backend(), services.EmulatorSetup{Preset: "eden", DataDir: "/my/eden"})
	vm.SetPreset("ryujinx")
	assert.Equal(t, "/my/eden", vm.Setup().DataDir, "a folder of the user's own stays")
}

func TestSaveLifecycle(t *testing.T) {
	be := backend()
	vm := NewViewModel(be, services.EmulatorSetup{})
	vm.SetName("Eden")
	vm.SetEnvText("A=1\nB=2")
	require.True(t, vm.Dirty())

	s := vm.BeginSave()
	assert.True(t, vm.Saving())
	assert.False(t, vm.Dirty(), "a form that is saving has nothing left to lose")
	assert.Equal(t, map[string]string{"A": "1", "B": "2"}, s.Env, "the variables are parsed for the save")

	ev, err := vm.Save(s)
	require.NoError(t, err)
	assert.Equal(t, "Eden", ev.Name)
	require.Len(t, be.saved, 1)

	assert.True(t, vm.FinishSave(nil))
	assert.False(t, vm.Saving())
	assert.False(t, vm.Dirty(), "saved: unchanged")
	assert.Empty(t, vm.ErrText())
}

func TestAFailedSaveShowsWhyAndStaysDirty(t *testing.T) {
	vm := NewViewModel(backend(), services.EmulatorSetup{})
	vm.SetName("Eden")
	vm.BeginSave()

	assert.False(t, vm.FinishSave(errors.New("name taken")))
	assert.Equal(t, "name taken", vm.ErrText())
	assert.False(t, vm.Saving(), "the Save button comes back")
	assert.True(t, vm.Dirty(), "nothing was saved")

	vm.BeginSave()
	assert.Empty(t, vm.ErrText(), "the next try starts clean")
}
