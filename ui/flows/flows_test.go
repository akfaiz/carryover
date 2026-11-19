package flows_test

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/widget"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/akfaiz/carryover/services"
	"github.com/akfaiz/carryover/ui/env"
	"github.com/akfaiz/carryover/ui/flows"
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

func TestGridKeyDialog(t *testing.T) {
	h := newHarness(t)
	assert.NotEmpty(t, flows.GridKeyHelp())
	saved := false
	flows.ShowGridKeyDialog(h.Shell, func() { saved = true })
	h.Settle()
	assert.Contains(t, uitest.ButtonTexts(h), "Save")
	entries := uitest.OverlayWidgets[*widget.Entry](h)
	require.NotEmpty(t, entries)
	entries[0].SetText("abc")
	uitest.TapOverlayButton(t, h, "Save")
	h.Settle()
	assert.True(t, saved)
	assert.Equal(t, "abc", h.Shell.Sync().GetSettings().SteamGridDBKey)
}

func TestOpenSteamWithoutLink(t *testing.T) {
	h := newHarness(t)
	flows.OpenSteam(h.Shell, services.GameView{Name: "G"})
	h.Settle()
	assert.NotEmpty(t, h.Toasts())
}
