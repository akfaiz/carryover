package components_test

import (
	"os"
	"path/filepath"
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

// A dialog opens at the folder of the path already set: tokens expanded, a
// missing file or folder replaced by the nearest folder above it.
func TestBrowseFromFindsTheFolderOfAPath(t *testing.T) {
	h := newHarness(t)
	dir := t.TempDir()
	from := func(raw string) string {
		got := "unset"
		components.BrowseFrom(h.Shell, "", raw, func(d string) { got = d })
		h.Settle()
		return got
	}
	assert.Equal(t, dir, from(filepath.Join(dir, "missing", "game.exe")))
	home, err := os.UserHomeDir()
	require.NoError(t, err)
	assert.Equal(t, home, from("{home}/no-such-folder-carryover"), "tokens are expanded")
	assert.Empty(t, from(""), "nothing set: the dialog's default")
	assert.Empty(t, from("{nosuchtoken}/x"), "a path that can't be expanded")
}
