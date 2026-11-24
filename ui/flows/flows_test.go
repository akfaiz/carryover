package flows_test

import (
	"testing"

	"github.com/egoist/mygo/ui"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/akfaiz/carryover/services"
	"github.com/akfaiz/carryover/ui/env"
	"github.com/akfaiz/carryover/ui/flows"
	"github.com/akfaiz/carryover/ui/internal/uitest"
	"github.com/akfaiz/carryover/ui/shell"
)

type blank struct{}

func (blank) View(c *ui.Context) { ui.Text(c, "blank") }

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
	assert.Contains(t, h.Texts(), "SteamGridDB API key")
	require.NoError(t, h.Tester.Click("Paste your key"))
	h.Tester.Type("abc")
	h.Settle()
	h.Click("Save")
	h.Settle()
	assert.True(t, saved)
	assert.Equal(t, "abc", h.Shell.Sync().GetSettings().SteamGridDBKey)
	assert.False(t, h.Shell.Dialogs().Open())
}

func TestAddGameOffersTheKindsOfGame(t *testing.T) {
	h := newHarness(t)
	flows.ShowAddGame(h.Shell)
	h.Settle()
	for _, want := range []string{"Add a game", "From Steam", "PC game", "Emulated game"} {
		assert.Contains(t, h.Texts(), want)
	}
	h.Click("PC game")
	assert.Equal(t, "/new", h.Shell.Route())
	assert.False(t, h.Shell.Dialogs().Open())
}

func TestAddGameEmulated(t *testing.T) {
	h := newHarness(t)
	flows.ShowAddGame(h.Shell)
	h.Settle()
	h.Click("Emulated game")
	assert.Equal(t, "/new/emulator", h.Shell.Route())
}

// An import with nothing installed says so, and Close closes it.
func TestSteamImportWithNothingInstalled(t *testing.T) {
	// A Steam with no games, whatever is installed on the machine running this.
	t.Setenv("CARRYOVER_STEAM_ROOT", t.TempDir())
	h := newHarness(t)
	flows.ShowSteamImport(h.Shell)
	h.Settle()
	assert.Contains(t, h.Texts(), "Add from Steam library")
	assert.Contains(t, h.Texts(), "No installed Steam games found on this device.")
	h.Click("Close")
	assert.False(t, h.Shell.Dialogs().Open())
}

func TestOpenSteamWithoutLink(t *testing.T) {
	h := newHarness(t)
	flows.OpenSteam(h.Shell, services.GameView{Name: "G"})
	h.Settle()
	assert.Contains(t, h.Texts(), "Steam can't start this game yet: add it to Steam on the game's page.")
}
