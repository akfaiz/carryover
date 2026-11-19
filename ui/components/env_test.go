package components_test

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/widget"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/akfaiz/carryover/internal/saves/pathvar"
	"github.com/akfaiz/carryover/services"
	"github.com/akfaiz/carryover/ui/components"
	"github.com/akfaiz/carryover/ui/env"
	"github.com/akfaiz/carryover/ui/internal/uitest"
	"github.com/akfaiz/carryover/ui/shell"
)

type blank struct{}

func (blank) View() fyne.CanvasObject { return widget.NewLabel("blank") }

func newHarness(t *testing.T) *uitest.Harness {
	t.Helper()
	return uitest.New(t, shell.Config{
		Page:         func(e env.Env, _ string) env.Page { return blank{} },
		AddGame:      func(env.Env) {},
		SyncProblems: func(env.Env, int, []services.SyncResult) {},
	})
}

func TestConfirmDanger(t *testing.T) {
	h := newHarness(t)
	var got *bool
	components.ConfirmDanger(h.Shell, components.ConfirmOpts{
		Title: "Delete?", Message: "Gone", Check: "More", Checked: "Everything", KeptHint: "kept",
		OnConfirm: func(c bool) { got = &c },
	})
	h.Settle()
	uitest.TapOverlayButton(t, h, "Cancel")
	h.Settle()
	assert.Nil(t, got)

	components.ConfirmDanger(h.Shell, components.ConfirmOpts{Title: "Delete?", Message: "Gone", OnConfirm: func(c bool) { got = &c }})
	h.Settle()
	uitest.TapOverlayButton(t, h, "Delete")
	h.Settle()
	require.NotNil(t, got)
	assert.False(t, *got)
}

func TestConfirmDiscard(t *testing.T) {
	h := newHarness(t)
	left := 0
	components.ConfirmDiscard(h.Shell, func() { left++ })
	h.Settle()
	uitest.TapOverlayButton(t, h, "Keep editing")
	h.Settle()
	assert.Zero(t, left)

	components.ConfirmDiscard(h.Shell, func() { left++ })
	h.Settle()
	uitest.TapOverlayButton(t, h, "Discard changes")
	h.Settle()
	assert.Equal(t, 1, left)
}

func TestShowCustomAndCopyField(t *testing.T) {
	h := newHarness(t)
	d := components.ShowCustom(h.Shell, "Title", widget.NewLabel("body"), 400)
	h.Settle()
	assert.Contains(t, uitest.OverlayText(h), "body")
	d.Hide()

	f := components.CopyField(h.Shell, "secret")
	assert.NotNil(t, f)
	uitest.ButtonWith(t, uitest.WidgetsOf[*widget.Button](f), "Copy").OnTapped()
	h.Settle()
	assert.Equal(t, "secret", h.Shell.App().Clipboard().Content())
}

func TestHelp(t *testing.T) {
	h := newHarness(t)
	assert.NotNil(t, components.Help(h.Shell, "summary", "more details"))
}

func TestGridKeyDialog(t *testing.T) {
	h := newHarness(t)
	assert.NotEmpty(t, components.GridKeyHelp())
	saved := false
	components.ShowGridKeyDialog(h.Shell, func() { saved = true })
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
	components.OpenSteam(h.Shell, services.GameView{Name: "G"})
	h.Settle()
	assert.NotEmpty(t, h.Toasts())
}

func TestRemoveButton(t *testing.T) {
	newHarness(t)
	called := false
	b := components.RemoveButton(func() { called = true })
	assert.Equal(t, widget.DangerImportance, b.Importance)
	b.OnTapped()
	assert.True(t, called)
}

func TestPathInput(t *testing.T) {
	h := newHarness(t)
	var changed string
	p := components.NewPathInput(h.Shell, "", []pathvar.Token{{Token: "{home}", Value: "/h"}, {Token: "{x:}", Description: "d"}},
		"", "path", func(s string) { changed = s })
	require.NotNil(t, p)
	var _ fyne.CanvasObject = p
	entries := uitest.WidgetsOf[*widget.Entry](p)
	require.NotEmpty(t, entries)
	entries[0].SetText("/tmp/x")
	assert.Equal(t, "/tmp/x", changed)
	h.Settle()
}
