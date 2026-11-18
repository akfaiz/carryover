package trash_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/akfaiz/carryover/internal/library"
	"github.com/akfaiz/carryover/services"
	"github.com/akfaiz/carryover/ui/env"
	"github.com/akfaiz/carryover/ui/internal/uitest"
	"github.com/akfaiz/carryover/ui/pages/trash"
	"github.com/akfaiz/carryover/ui/shell"
)

// newHarness shows only the Trash page in a shell, in an isolated data dir.
func newHarness(t *testing.T) *uitest.Harness {
	t.Helper()
	return uitest.New(t, shell.Config{
		Page:         func(e env.Env, _ string) env.Page { return trash.New(e) },
		AddGame:      func(env.Env) {},
		SyncProblems: func(env.Env, int, []services.SyncResult) {},
	})
}

// "Delete permanently" asks first, and a game deleted for good can't be restored.
func TestTrashPageDeleteForever(t *testing.T) {
	h := newHarness(t)
	g := h.AddGame("Game A", t.TempDir())
	require.NoError(t, h.Shell.Lib().Delete(g.ID))

	h.Open("/trash")
	uitest.TapButton(t, h, "Delete permanently…")
	h.Settle()
	uitest.TapOverlayButton(t, h, "Cancel")
	h.Settle()
	require.Len(t, h.Core.Library.Trash(), 1)

	uitest.TapButton(t, h, "Delete permanently…")
	h.Settle()
	uitest.TapOverlayButton(t, h, "Delete")
	h.Settle()
	assert.Empty(t, h.Core.Library.Trash())
	_, err := h.Core.Library.Restore(g.ID)
	require.ErrorIs(t, err, library.ErrNotRestorable)
}

// Cloud saves no game uses are listed and can be restored into the editor.
func TestTrashPageLeftoverSaves(t *testing.T) {
	h := newHarness(t)
	h.UseLocalCloud()
	saves := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(saves, "a.sav"), []byte("save"), 0o644))
	g := h.AddGame("Game A", saves)
	_, err := h.Shell.Sync().SyncAll()
	require.NoError(t, err)
	require.NoError(t, h.Shell.Lib().DeleteForever(g.ID)) // keeps the cloud saves

	h.Open("/trash")
	h.Settle()
	uitest.TapButton(t, h, "Restore")
	h.Settle()
	assert.Equal(t, "/edit/"+g.ID, h.Shell.Route(), "the editor opens to set the save location")
	v, err := h.Shell.Lib().Get(g.ID)
	require.NoError(t, err)
	assert.Equal(t, "Game A", v.Name)
}
