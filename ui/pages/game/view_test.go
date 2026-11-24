package game_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/akfaiz/carryover/services"
	"github.com/akfaiz/carryover/ui/env"
	"github.com/akfaiz/carryover/ui/internal/uitest"
	"github.com/akfaiz/carryover/ui/kit"
	"github.com/akfaiz/carryover/ui/pages/game"
	"github.com/akfaiz/carryover/ui/shell"
)

// elsewhere stands in for every page but the game's: tests only need to see
// where the game page navigated to.
type elsewhere struct{}

func (elsewhere) View(c *ui.Context) { ui.Text(c, "elsewhere") }

// newHarness shows the game page in a shell, in an isolated data dir. Any other
// route shows a stand-in page.
func newHarness(t *testing.T) *uitest.Harness {
	t.Helper()
	return uitest.New(t, shell.Config{
		Page: func(e env.Env, route string) env.Page {
			if id, ok := strings.CutPrefix(route, "/game/"); ok {
				return game.New(e, id)
			}
			return elsewhere{}
		},
		AddGame:      func(env.Env) {},
		SyncProblems: func(env.Env, int, []services.SyncResult) {},
	})
}

func askDelete(t *testing.T, h *uitest.Harness) {
	t.Helper()
	h.Click("⋯")
	h.Click("Delete from library…")
}

func TestGamePageDeleteAsksFirst(t *testing.T) {
	h := newHarness(t)
	g := h.AddGame("Game A", t.TempDir())
	h.Open("/")
	h.Open("/game/" + g.ID)

	askDelete(t, h)
	h.Click("Cancel") // the confirmation can be backed out of
	require.Len(t, h.Core.Library.List(), 1)
	assert.Equal(t, "/game/"+g.ID, h.Shell.Route())

	askDelete(t, h)
	h.Click("Delete")
	h.Settle()
	assert.Empty(t, h.Core.Library.List())
	assert.Equal(t, "/", h.Shell.Route(), "the game's page is gone")
}

func TestGamePageSyncAndBackup(t *testing.T) {
	h := newHarness(t)
	cloudDir := h.UseLocalCloud()
	saves := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(saves, "a.sav"), []byte("save"), 0o644))
	g := h.AddGame("Game A", saves)
	h.Open("/game/" + g.ID)

	h.Click("Sync now")
	h.Settle()
	assert.FileExists(t, filepath.Join(cloudDir, "games", g.ID, "current", "saves", "a.sav"), "the saves were uploaded")

	// Back up now stores a pinned backup.
	h.Open("/game/" + g.ID)
	h.Click("Back up now")
	h.Settle()
	backups, err := h.Shell.Sync().Backups(g.ID)
	require.NoError(t, err)
	require.Len(t, backups, 1)
	assert.Equal(t, "manual", backups[0].Kind)
	assert.True(t, backups[0].Pinned)
}

// The game page has a switcher with one button per way to play the game; the
// saves and backups of the selected one are shown.
func TestGamePageSwitchesBetweenEmulators(t *testing.T) {
	h := newHarness(t)
	g, eden, _ := h.AddEmulatorGame()
	h.Open("/game/" + g.ID)

	for _, name := range []string{"Without an emulator", "Eden", "Ryujinx"} {
		assert.Contains(t, h.Texts(), name, "a switcher button for %s", name)
	}
	gp := h.Shell.Page().(*game.Page)
	assert.Empty(t, gp.SelectedProfile(uitest.MustGame(t, h, g.ID)), "starts on the first")
	h.Click("Eden")
	assert.Equal(t, eden.ID, gp.SelectedProfile(uitest.MustGame(t, h, g.ID)))
}

// A plain game (no emulators) has no switcher.
func TestGamePageWithoutEmulatorsHasNoTags(t *testing.T) {
	h := newHarness(t)
	g := h.AddGame("Game A", t.TempDir())
	h.Open("/game/" + g.ID)
	texts := strings.Join(h.Texts(), "\n")
	assert.NotContains(t, texts, "Without an emulator")
	assert.NotContains(t, texts, "Saves of")
	assert.Contains(t, texts, "Saves")
}

// A conflict found by "Sync now" is compared with the cloud at once, so the
// buttons to keep either side are right there.
func TestGamePageConflictShowsTheChoice(t *testing.T) {
	h := newHarness(t)
	h.UseLocalCloud()
	saves := t.TempDir()
	file := filepath.Join(saves, "a.sav")
	require.NoError(t, os.WriteFile(file, []byte("one"), 0o644))
	g := h.AddGame("Game A", saves)
	_, err := h.Shell.Sync().Sync(g.ID, "")
	require.NoError(t, err)

	// Another device uploads a change, and this one changes too.
	st, err := h.Core.States.Load(g.ID)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(file, []byte("two-local"), 0o644))
	_, err = h.Shell.Sync().Sync(g.ID, "local") // the cloud now has "two-local"
	require.NoError(t, err)
	require.NoError(t, h.Core.States.Save(g.ID, st)) // forget that sync: both sides look changed
	require.NoError(t, os.WriteFile(file, []byte("three"), 0o644))

	h.Open("/game/" + g.ID)
	h.Click("Sync now")
	h.Settle()
	assert.Contains(t, h.Texts(), "Keep this device's saves")
	assert.Contains(t, h.Texts(), "Keep cloud saves")
}

// The delete confirmation says where kept cloud saves can be found again, and
// drops the line once they are being deleted too.
func TestDeleteConfirmationExplainsKeptSaves(t *testing.T) {
	h := newHarness(t)
	g := h.AddGame("Game A", t.TempDir())
	h.Open("/game/" + g.ID)
	askDelete(t, h)
	kept := "The cloud saves stay until you delete them. Once it is gone for good, they are listed under Trash → Leftover cloud saves."
	assert.Contains(t, h.Texts(), kept)
	h.Click("Also delete its cloud saves and backups")
	assert.NotContains(t, h.Texts(), kept)
}

// The pad starts on Back, which only goes back, never on Play.
func TestGamePagePadStartsOnBack(t *testing.T) {
	h := newHarness(t)
	g := h.AddGame("Game A", t.TempDir())
	h.Open("/")
	h.Open("/game/" + g.ID)
	h.Shell.Navigator().Press(kit.PadDown)
	h.Settle()
	assert.False(t, h.Tester.Focused("Play"), "a stray A can't start the game")
	h.Shell.Navigator().Press(kit.PadAccept)
	h.Settle()
	assert.Equal(t, "/", h.Shell.Route(), "A on Back goes back")
}
