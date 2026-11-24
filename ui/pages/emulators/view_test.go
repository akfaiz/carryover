package emulators_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/akfaiz/carryover/services"
	"github.com/akfaiz/carryover/ui/env"
	"github.com/akfaiz/carryover/ui/internal/uitest"
	"github.com/akfaiz/carryover/ui/kit"
	"github.com/akfaiz/carryover/ui/pages/emulators"
	"github.com/akfaiz/carryover/ui/shell"
)

func newHarness(t *testing.T) *uitest.Harness {
	t.Helper()
	h := uitest.New(t, shell.Config{
		Page:         func(e env.Env, _ string) env.Page { return emulators.New(e) },
		AddGame:      func(env.Env) {},
		SyncProblems: func(env.Env, int, []services.SyncResult) {},
	})
	h.Open("/emulators")
	return h
}

// typeName types into the form's name field.
func typeName(t *testing.T, h *uitest.Harness, name string) {
	t.Helper()
	require.NoError(t, h.Tester.Click("Emulator name (e.g. Eden)"))
	h.Tester.Type(name)
	h.Settle()
}

func TestEmptyList(t *testing.T) {
	h := newHarness(t)
	assert.Contains(t, h.Texts(), "Add emulator")
	assert.Contains(t, h.Texts(), "No emulators yet. Add one, for example Eden or Ryujinx for Nintendo Switch games.")
}

func TestAddEmulatorThenDelete(t *testing.T) {
	h := newHarness(t)
	h.Click("Add emulator")
	typeName(t, h, "Eden")
	h.Click("Save")
	h.Settle()
	require.Len(t, h.Shell.Emus().List(), 1)
	assert.Contains(t, h.Texts(), "Edit")

	h.Click("Delete…")
	h.Click("Delete")
	h.Settle()
	assert.Empty(t, h.Shell.Emus().List())
}

func TestFormCancelAndDiscard(t *testing.T) {
	h := newHarness(t)
	h.Click("Add emulator")
	h.Click("Cancel")
	assert.Contains(t, h.Texts(), "Add emulator", "an untouched form closes at once")

	h.Click("Add emulator")
	typeName(t, h, "X")
	h.Click("Cancel")
	assert.Contains(t, h.Texts(), "Discard changes", "a changed form asks first")
	h.Click("Discard changes")
	assert.Contains(t, h.Texts(), "Add emulator")
	assert.Empty(t, h.Shell.Emus().List())
}

// Leaving the page with a dirty form asks too.
func TestLeavingWithAChangedFormAsks(t *testing.T) {
	h := newHarness(t)
	h.Click("Add emulator")
	typeName(t, h, "X")
	h.Shell.Navigate("/")
	h.Settle()
	assert.Equal(t, "/emulators", h.Shell.Route())
	assert.True(t, h.Shell.Confirming().Open())
}

func TestUnsetFromDevice(t *testing.T) {
	h := newHarness(t)
	_, err := h.Shell.Emus().Save(services.EmulatorSetup{Name: "Eden", Exe: "/bin/true"})
	require.NoError(t, err)
	h.Open("/emulators")
	h.Click("Remove from this device…")
	h.Click("Remove")
	h.Settle()
	assert.NotContains(t, h.Texts(), "Remove from this device…")
}

// Choosing a type fills in its usual arguments and shows the data folder.
func TestChoosingATypeFillsTheForm(t *testing.T) {
	h := newHarness(t)
	h.Click("Add emulator")
	assert.NotContains(t, h.Texts(), "Left empty, the usual folder shown above is used.")
	h.Click("Not listed")
	require.NotEmpty(t, h.Shell.Emus().Presets())
	first := h.Shell.Emus().Presets()[0]
	h.Click(first.Name)
	h.Settle()
	assert.Contains(t, h.Texts(), first.Name+"", "the type is chosen")
	assert.Contains(t, h.Texts(), "Data folder on this device: "+first.RootHint, "the type adds the data folder field")
	assert.NotContains(t, h.Texts(), "Usual for this type: "+first.Args, "empty arguments were filled in, so no hint")
}

// The pad reaches the form's buttons and a dialog starts on its safe choice.
func TestPadReachesTheForm(t *testing.T) {
	h := newHarness(t)
	h.Click("Add emulator")
	h.Shell.Navigator().Press(kit.PadDown)
	h.Settle()
	assert.False(t, h.Tester.Focused("Library"), "the first press starts on the page")
}
