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
	"github.com/akfaiz/carryover/ui/kit"
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

// An empty trash says so once, not with a card per kind.
func TestTrashPageEmpty(t *testing.T) {
	h := newHarness(t)
	h.Open("/trash")
	assert.Contains(t, h.Texts(), "Trash is empty")
	assert.NotContains(t, h.Texts(), "Games")
	assert.NotContains(t, h.Texts(), "Leftover cloud saves")
	assert.Equal(t, 2, count(h.Texts(), "Emulators"), "only the sidebar's (text and name) says Emulators")
}

// count is how many of texts are s.
func count(texts []string, s string) int {
	n := 0
	for _, x := range texts {
		if x == s {
			n++
		}
	}
	return n
}

// Only the kinds with something deleted get a card.
func TestTrashPageShowsOnlyKindsWithItems(t *testing.T) {
	h := newHarness(t)
	g := h.AddGame("Game A", t.TempDir())
	require.NoError(t, h.Shell.Lib().Delete(g.ID))
	h.Open("/trash")
	assert.Contains(t, h.Texts(), "Games")
	assert.Contains(t, h.Texts(), "Game A")
	assert.Equal(t, 2, count(h.Texts(), "Emulators"), "no Emulators card: only the sidebar's")
	assert.NotContains(t, h.Texts(), "Trash is empty")
}

// "Delete permanently" asks first, and a game deleted for good can't be restored.
func TestTrashPageDeleteForever(t *testing.T) {
	h := newHarness(t)
	g := h.AddGame("Game A", t.TempDir())
	require.NoError(t, h.Shell.Lib().Delete(g.ID))

	h.Open("/trash")
	h.Click("Delete permanently…")
	h.Click("Cancel")
	require.Len(t, h.Core.Library.Trash(), 1)

	h.Click("Delete permanently…")
	assert.Contains(t, h.Texts(), "Also delete its cloud saves and backups")
	h.Click("Delete")
	h.Settle()
	assert.Empty(t, h.Core.Library.Trash())
	_, err := h.Core.Library.Restore(g.ID)
	require.ErrorIs(t, err, library.ErrNotRestorable)
}

// The checkbox in the dialog changes what it says it will delete.
func TestTrashPageDeleteWithCloudSaves(t *testing.T) {
	h := newHarness(t)
	g := h.AddGame("Game A", t.TempDir())
	require.NoError(t, h.Shell.Lib().Delete(g.ID))
	h.Open("/trash")
	h.Click("Delete permanently…")
	assert.Contains(t, h.Texts(), "The cloud saves stay and are listed under Leftover cloud saves below until you delete them.")
	h.Click("Also delete its cloud saves and backups")
	assert.NotContains(t, h.Texts(), "The cloud saves stay and are listed under Leftover cloud saves below until you delete them.")
	assert.Contains(t, h.Texts(), "Delete Game A permanently together with its cloud saves and backups? This cannot be undone.")
}

func TestTrashPageRestoresAGame(t *testing.T) {
	h := newHarness(t)
	g := h.AddGame("Game A", t.TempDir())
	require.NoError(t, h.Shell.Lib().Delete(g.ID))
	h.Open("/trash")
	h.Click("Restore")
	h.Settle()
	assert.Empty(t, h.Core.Library.Trash())
	_, ok := h.Shell.Store().Game(g.ID)
	assert.True(t, ok)
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
	h.Click("Restore")
	h.Settle()
	assert.Equal(t, "/edit/"+g.ID, h.Shell.Route(), "the editor opens to set the save location")
	v, err := h.Shell.Lib().Get(g.ID)
	require.NoError(t, err)
	assert.Equal(t, "Game A", v.Name)
}

// The pad reaches the page's buttons.
func TestTrashPagePadReachesButtons(t *testing.T) {
	h := newHarness(t)
	g := h.AddGame("Game A", t.TempDir())
	require.NoError(t, h.Shell.Lib().Delete(g.ID))
	h.Open("/trash")
	h.Shell.Navigator().Press(kit.PadDown)
	h.Settle()
	assert.True(t, h.Tester.Focused("Restore") || h.Tester.Focused("Delete permanently…"), "the first press lands on the page, not the sidebar")
}
