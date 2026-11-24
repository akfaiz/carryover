package editor_test

import (
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/akfaiz/carryover/services"
	"github.com/akfaiz/carryover/ui/env"
	"github.com/akfaiz/carryover/ui/internal/uitest"
	"github.com/akfaiz/carryover/ui/pages/editor"
	"github.com/akfaiz/carryover/ui/shell"
)

// elsewhere stands in for every page but the editor: tests only need to see
// that the editor navigated to it.
type elsewhere struct{}

func (elsewhere) View(c *ui.Context) { ui.Text(c, "elsewhere") }

// newHarness shows the game editor in a shell, in an isolated data dir. Any
// other route shows a stand-in page.
func newHarness(t *testing.T) *uitest.Harness {
	t.Helper()
	h := uitest.New(t, shell.Config{
		Page: func(e env.Env, route string) env.Page {
			switch {
			case route == "/new":
				return editor.New(e, "", "")
			case route == "/new/emulator":
				return editor.NewEmulated(e)
			case strings.HasPrefix(route, "/edit/"):
				return editor.New(e, strings.TrimPrefix(route, "/edit/"), "")
			}
			return elsewhere{}
		},
		AddGame:      func(env.Env) {},
		SyncProblems: func(env.Env, int, []services.SyncResult) {},
	})
	h.Tester.SetSize(1000, 2600) // tall: the whole form is in view, nothing to scroll to
	return h
}

// typeInto clicks the empty field showing placeholder and types text.
func typeInto(t *testing.T, h *uitest.Harness, placeholder, text string) {
	t.Helper()
	require.NoError(t, h.Tester.Click(placeholder))
	h.Tester.Type(text)
	h.Settle()
}

// Only the name is required: a game can be added without a path, and another
// device sets its own later.
func TestEditorSaveNeedsAName(t *testing.T) {
	h := newHarness(t)
	h.Open("/new")
	info := func() ui.AccessInfo {
		a, ok := h.Tester.AccessByRole(ui.RoleButton, "Save")
		require.True(t, ok)
		return a
	}
	assert.True(t, info().Disabled, "an empty form can't be saved")
	typeInto(t, h, "Game name", "   ")
	assert.True(t, info().Disabled, "a blank name doesn't count")
	typeInto(t, h, "Game name", "A")
	assert.False(t, info().Disabled, "a name is enough")
}

func TestEditorCreatesGame(t *testing.T) {
	h := newHarness(t)
	h.Open("/new")
	typeInto(t, h, "Game name", "Game A")
	typeInto(t, h, "-windowed", "-windowed -nosplash")
	typeInto(t, h, "{appdata}/Game/Saves", "{home}/GameA/Saves")
	h.Click("Save")
	h.Settle()

	games := h.Core.Library.List()
	require.Len(t, games, 1)
	g := games[0]
	assert.Equal(t, "Game A", g.Name)
	require.Len(t, g.Slots, 1)
	assert.Equal(t, "Saves", g.Slots[0].Label)
	dev, ok := g.Device(h.Core.DeviceID())
	require.True(t, ok)
	assert.Equal(t, "-windowed -nosplash", dev.Args)
	assert.Equal(t, "{home}/GameA/Saves", dev.Paths[g.Slots[0].Key])
	assert.Contains(t, h.Shell.Route(), g.ID, "the game's page opens after saving")
}

func TestEditorSecondSaveLocation(t *testing.T) {
	h := newHarness(t)
	h.Open("/new")
	typeInto(t, h, "Game name", "Game A")
	typeInto(t, h, "{appdata}/Game/Saves", "{home}/one")
	h.Click("Add")
	// Both locations show the same placeholder; the empty one is the second.
	typeInto(t, h, "{appdata}/Game/Saves", "{home}/two")
	h.Click("Save")
	h.Settle()

	games := h.Core.Library.List()
	require.Len(t, games, 1)
	require.Len(t, games[0].Slots, 2)
	assert.NotEqual(t, games[0].Slots[0].Key, games[0].Slots[1].Key, "each location has its own key")
	dev, _ := games[0].Device(h.Core.DeviceID())
	assert.Len(t, dev.Paths, 2)
}

func TestEditorEditsExistingGame(t *testing.T) {
	h := newHarness(t)
	g := h.AddGame("Game A", t.TempDir())
	h.Open("/edit/" + g.ID)
	assert.Contains(t, h.Texts(), "Edit game")
	h.Shell.Navigator().Handle([2]any{"editor-field", "name"}).Focus() // the name field, holding the saved name
	h.Settle()
	h.Tester.Key(ui.Cmd, ui.KeyA)
	h.Tester.Type("Game A (GOTY)")
	h.Settle()
	h.Click("Save")
	h.Settle()

	games := h.Core.Library.List()
	require.Len(t, games, 1, "editing doesn't add a game")
	assert.Equal(t, "Game A (GOTY)", games[0].Name)
	assert.Equal(t, g.ID, games[0].ID)
}

// Leaving the editor with unsaved changes asks first.
func TestEditorAsksBeforeDiscarding(t *testing.T) {
	h := newHarness(t)
	h.Open("/")
	h.Open("/new")
	typeInto(t, h, "Game name", "Game A")

	h.Shell.Navigate("/settings")
	h.Settle()
	assert.Equal(t, "/new", h.Shell.Route(), "still on the editor")
	h.Click("Keep editing")
	assert.Equal(t, "/new", h.Shell.Route())

	h.Shell.Navigate("/settings")
	h.Settle()
	h.Click("Discard changes")
	assert.Equal(t, "/settings", h.Shell.Route())
	assert.Empty(t, h.Core.Library.List())
}

// An emulated game shows the emulators card and no PC launch or saves.
func TestEditorEmulatedGameShowsEmulators(t *testing.T) {
	h := newHarness(t)
	h.Open("/new/emulator")
	assert.Contains(t, h.Texts(), "Emulators")
	assert.Contains(t, h.Texts(), "New emulator…")
	assert.NotContains(t, h.Texts(), "Launch on this device")
	h.Click("PC or Steam game")
	assert.Contains(t, h.Texts(), "Launch on this device")
	assert.NotContains(t, h.Texts(), "New emulator…")
}

// An emulator added from the game editor is set up in a dialog and joins the
// game, and what was typed in the editor stays.
func TestEditorAddsANewEmulatorInADialog(t *testing.T) {
	h := newHarness(t)
	h.Open("/new/emulator")
	typeInto(t, h, "Game name", "Zelda")
	h.Click("New emulator…")
	assert.True(t, h.Shell.Dialogs().Open())
	typeInto(t, h, "Emulator name (e.g. Eden)", "Eden")
	h.Click("Save")
	h.Settle()
	assert.False(t, h.Shell.Dialogs().Open(), "the dialog closed")
	assert.Equal(t, "/new/emulator", h.Shell.Route(), "still in the editor")
	e := h.Shell.Page().(*editor.Page)
	assert.Equal(t, "Zelda", e.Name())
	assert.Equal(t, []string{"Eden"}, e.EmulatorNames())
	require.Len(t, h.Shell.Emus().List(), 1)
}
