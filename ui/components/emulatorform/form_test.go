package emulatorform_test

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/widget"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/akfaiz/carryover/services"
	"github.com/akfaiz/carryover/ui/components/emulatorform"
	"github.com/akfaiz/carryover/ui/env"
	"github.com/akfaiz/carryover/ui/internal/uitest"
	"github.com/akfaiz/carryover/ui/shell"
)

type blank struct{}

func (blank) View() fyne.CanvasObject { return widget.NewLabel("blank") }

func newHarness(t *testing.T) *uitest.Harness {
	t.Helper()
	return uitest.New(t, shell.Config{
		Page:         func(env.Env, string) env.Page { return blank{} },
		AddGame:      func(env.Env) {},
		SyncProblems: func(env.Env, int, []services.SyncResult) {},
	})
}

func TestFormAddAndSave(t *testing.T) {
	h := newHarness(t)
	var cancelled bool
	var saved *services.EmulatorView
	f := emulatorform.New(h.Shell, services.EmulatorSetup{}, nil,
		func() { cancelled = true }, func(ev services.EmulatorView) { saved = &ev })
	f.Relayout = func() {}
	assert.Equal(t, "Add emulator", f.Title())
	assert.False(t, f.Dirty())

	entries := uitest.WidgetsOf[*widget.Entry](f.Host)
	require.GreaterOrEqual(t, len(entries), 2)
	for _, e := range entries {
		if e.PlaceHolder == "Emulator name (e.g. Eden)" {
			e.SetText("Eden")
		}
	}
	assert.True(t, f.Dirty())

	// Cancel button
	uitest.ButtonWith(t, uitest.WidgetsOf[*widget.Button](f.Host), "Cancel").OnTapped()
	assert.True(t, cancelled)

	// Save stores the emulator and reports it.
	uitest.ButtonWith(t, uitest.WidgetsOf[*widget.Button](f.Host), "Save").OnTapped()
	h.Settle()
	require.NotNil(t, saved)
	assert.Equal(t, "Eden", saved.Name)
}

func TestFormTypeChoiceShowsDataFolder(t *testing.T) {
	h := newHarness(t)
	presets := h.Shell.Emus().Presets()
	if len(presets) == 0 {
		t.Skip("no presets")
	}
	f := emulatorform.New(h.Shell, services.EmulatorSetup{Name: "E", Exe: "/x", Preset: presets[0].ID}, nil, func() {}, func(services.EmulatorView) {})
	assert.NotNil(t, f.Host)
	assert.Equal(t, "Add emulator", f.Title())
}

func TestFormEdit(t *testing.T) {
	h := newHarness(t)
	f := emulatorform.New(h.Shell, services.EmulatorSetup{ID: "x", Name: "E"}, nil, func() {}, func(services.EmulatorView) {})
	assert.Equal(t, "Edit emulator", f.Title())
}

func TestShowDialog(t *testing.T) {
	h := newHarness(t)
	emulatorform.ShowDialog(h.Shell, func(services.EmulatorView) {})
	h.Settle()
	assert.Contains(t, uitest.ButtonTexts(h), "Cancel")
	uitest.TapOverlayButton(t, h, "Cancel")
	h.Settle()
	assert.NotContains(t, uitest.ButtonTexts(h), "Cancel")
}
