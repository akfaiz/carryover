package flows_test

import (
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/akfaiz/carryover/services"
	"github.com/akfaiz/carryover/ui/env"
	"github.com/akfaiz/carryover/ui/flows"
	"github.com/akfaiz/carryover/ui/internal/uitest"
	"github.com/akfaiz/carryover/ui/kit"
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

// The dialog explains only the kind of game under the pointer or holding the
// focus, and keeps the room for it so that nothing moves.
func TestAddGameExplainsOnlyThePointedOrFocusedKind(t *testing.T) {
	const pc, emulated = "PC game: any other game", "Emulated game: a game file"
	h := newHarness(t)
	flows.ShowAddGame(h.Shell)
	h.Settle()

	shown := func(prefix string) bool {
		for _, s := range h.Texts() {
			if strings.HasPrefix(s, prefix) {
				return true
			}
		}
		return false
	}
	find := func(s string) ui.Rect {
		r, ok := h.Tester.Find(s)
		require.True(t, ok, s)
		return r
	}
	assert.False(t, shown("From Steam:"))
	assert.False(t, shown(pc))
	assert.False(t, shown(emulated))
	before := find("Cancel")

	r := find("PC game")
	h.Tester.Move(r.X+r.W/2, r.Y+r.H/2)
	h.Settle()
	assert.True(t, shown(pc))
	assert.False(t, shown(emulated))
	assert.Equal(t, before, find("Cancel"), "nothing moves")

	h.Tester.Move(1, 1)
	h.Settle()
	assert.False(t, shown(pc))

	// Cancel has the focus as the dialog opens: the D-pad reaches the tiles.
	for _, step := range []struct {
		press       kit.PadAction
		shown, gone string
	}{{kit.PadLeft, emulated, pc}, {kit.PadLeft, pc, emulated}} {
		h.Shell.Navigator().Press(step.press)
		h.Tester.Frame()
		h.Settle()
		assert.True(t, shown(step.shown), step.shown)
		assert.False(t, shown(step.gone), step.gone)
	}
	assert.Equal(t, before, find("Cancel"), "nothing moves")
}
