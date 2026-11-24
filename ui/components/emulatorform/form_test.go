package emulatorform_test

import (
	"testing"

	"github.com/egoist/mygo/ui"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/akfaiz/carryover/services"
	"github.com/akfaiz/carryover/ui/components/emulatorform"
	"github.com/akfaiz/carryover/ui/env"
	"github.com/akfaiz/carryover/ui/internal/uitest"
	"github.com/akfaiz/carryover/ui/kit"
	"github.com/akfaiz/carryover/ui/shell"
)

func newHarness(t *testing.T) *uitest.Harness {
	t.Helper()
	h := uitest.New(t, shell.Config{
		Page:         func(env.Env, string) env.Page { return nil },
		AddGame:      func(env.Env) {},
		SyncProblems: func(env.Env, int, []services.SyncResult) {},
	})
	h.Tester.SetSize(1000, 1400)
	h.Settle()
	return h
}

// The dialog adds an emulator: type its name and program, Save.
func TestShowDialogAddsAnEmulator(t *testing.T) {
	h := newHarness(t)
	var saved []services.EmulatorView
	emulatorform.ShowDialog(h.Shell, func(ev services.EmulatorView) { saved = append(saved, ev) })
	h.Settle()
	h.Settle()
	require.True(t, h.Shell.Dialogs().Open())
	for _, want := range []string{"Add emulator", "Executable on this device", "Name", "Arguments", "Advanced", "Cancel", "Save"} {
		assert.Contains(t, h.Texts(), want)
	}

	require.NoError(t, h.Tester.Click("Emulator name (e.g. Eden)"))
	h.Tester.Type("My Emu")
	h.Settle()
	exe := t.TempDir() + "/emu.exe"
	require.NoError(t, h.Tester.Click(exampleExe(h)))
	h.Tester.Type(exe)
	h.Settle()

	h.Click("Save")
	for range 20 {
		h.Settle()
		if len(saved) > 0 {
			break
		}
	}
	require.Len(t, saved, 1)
	assert.Equal(t, "My Emu", saved[0].Name)
	assert.False(t, h.Shell.Dialogs().Open())
}

// exampleExe is the placeholder of the program field: the only text of it on screen.
func exampleExe(h *uitest.Harness) string {
	for _, tx := range h.Texts() {
		for _, ext := range []string{"eden.exe", "eden.AppImage", "Eden.app"} {
			if len(tx) > len(ext) && tx[len(tx)-len(ext):] == ext {
				return tx
			}
		}
	}
	return ""
}

// Leaving the form clean closes it; with something typed it asks first.
func TestCancelAsksOnlyWhenSomethingChanged(t *testing.T) {
	h := newHarness(t)
	emulatorform.ShowDialog(h.Shell, func(services.EmulatorView) {})
	h.Settle()
	h.Settle()
	h.Click("Cancel")
	assert.False(t, h.Shell.Dialogs().Open())

	emulatorform.ShowDialog(h.Shell, func(services.EmulatorView) {})
	h.Settle()
	h.Settle()
	require.NoError(t, h.Tester.Click("Emulator name (e.g. Eden)"))
	h.Tester.Type("x")
	h.Settle()
	h.Click("Cancel")
	assert.True(t, h.Shell.Dialogs().Open(), "still open: asked first")
	assert.Contains(t, h.Texts(), "Discard changes?")
	h.Click("Discard changes")
	assert.False(t, h.Shell.Dialogs().Open())
}

// An emulator of a known type shows what the type suggests, and a form for an
// existing one is titled so.
func TestFormOfAKnownType(t *testing.T) {
	h := newHarness(t)
	presets := h.Shell.Emus().Presets()
	require.NotEmpty(t, presets)
	var preset services.PresetView
	for _, p := range presets {
		if p.Args != "" {
			preset = p
			break
		}
	}
	require.NotEmpty(t, preset.ID)

	cancelled := 0
	f := emulatorform.New(h.Shell, services.EmulatorSetup{ID: "existing", Name: "Mine", Preset: preset.ID, Exe: "/x/emu", WorkDir: "/w", Args: "--old"},
		nil, func() { cancelled++ }, func(services.EmulatorView) {})
	assert.Equal(t, "Edit emulator", f.Title())
	assert.False(t, f.Dirty())

	h.Shell.Dialogs().Show(&kit.Dialog{Title: f.Title(), Wide: true, Body: func(c *ui.Context, _ *kit.Navigator) {
		kit.ScrollBox(c, 900, 0, func() { f.View(c) })
	}})
	h.Settle()
	h.Settle()
	assert.Contains(t, h.Texts(), "Usual for this type: "+preset.Args)
	h.Click("Use")
	assert.True(t, f.Dirty(), "the usual arguments replaced the old ones")
	h.Click("Cancel")
	assert.Equal(t, 1, cancelled)
}
