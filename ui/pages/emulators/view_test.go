package emulators_test

import (
	"testing"

	"fyne.io/fyne/v2/widget"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/akfaiz/carryover/services"
	"github.com/akfaiz/carryover/ui/env"
	"github.com/akfaiz/carryover/ui/internal/uitest"
	"github.com/akfaiz/carryover/ui/pages/emulators"
	"github.com/akfaiz/carryover/ui/shell"
)

func newHarness(t *testing.T) *uitest.Harness {
	t.Helper()
	return uitest.New(t, shell.Config{
		Page:         func(e env.Env, _ string) env.Page { return emulators.New(e) },
		AddGame:      func(env.Env) {},
		SyncProblems: func(env.Env, int, []services.SyncResult) {},
	})
}

func TestEmptyList(t *testing.T) {
	h := newHarness(t)
	h.Open("/emulators")
	assert.Contains(t, uitest.ButtonTextsOnPage(h), "Add emulator")
}

func TestAddEmulatorThenDelete(t *testing.T) {
	h := newHarness(t)
	h.Open("/emulators")

	uitest.TapButton(t, h, "Add emulator")
	h.Settle()
	for _, e := range uitest.PageWidgets[*widget.Entry](h) {
		if e.PlaceHolder == "Emulator name (e.g. Eden)" {
			e.SetText("Eden")
		}
	}
	uitest.TapButton(t, h, "Save")
	h.Settle()
	require.Len(t, h.Shell.Emus().List(), 1)
	assert.Contains(t, uitest.ButtonTextsOnPage(h), "Edit")

	uitest.TapButton(t, h, "Delete…")
	h.Settle()
	uitest.TapOverlayButton(t, h, "Delete")
	h.Settle()
	assert.Empty(t, h.Shell.Emus().List())
}

func TestFormCancelAndDiscard(t *testing.T) {
	h := newHarness(t)
	h.Open("/emulators")
	uitest.TapButton(t, h, "Add emulator")
	h.Settle()
	uitest.TapButton(t, h, "Cancel")
	h.Settle()
	assert.Contains(t, uitest.ButtonTextsOnPage(h), "Add emulator")

	uitest.TapButton(t, h, "Add emulator")
	h.Settle()
	for _, e := range uitest.PageWidgets[*widget.Entry](h) {
		if e.PlaceHolder == "Emulator name (e.g. Eden)" {
			e.SetText("X")
		}
	}
	uitest.TapButton(t, h, "Cancel")
	h.Settle()
	uitest.TapOverlayButton(t, h, "Discard changes")
	h.Settle()
	assert.Contains(t, uitest.ButtonTextsOnPage(h), "Add emulator")
}

func TestUnsetFromDevice(t *testing.T) {
	h := newHarness(t)
	_, err := h.Shell.Emus().Save(services.EmulatorSetup{Name: "Eden", Exe: "/bin/true"})
	require.NoError(t, err)
	h.Open("/emulators")
	uitest.TapButton(t, h, "Remove from this device…")
	h.Settle()
	uitest.TapOverlayButton(t, h, "Remove")
	h.Settle()
	assert.NotContains(t, uitest.ButtonTextsOnPage(h), "Remove from this device…")
}
