package editor_test

import (
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/widget"
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

func (elsewhere) View() fyne.CanvasObject { return widget.NewLabel("elsewhere") }

// newHarness shows the game editor in a shell, in an isolated data dir. Any
// other route shows a stand-in page.
func newHarness(t *testing.T) *uitest.Harness {
	t.Helper()
	return uitest.New(t, shell.Config{
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
}

// exeEntry finds the executable field. Its placeholder is an example path in
// the running OS's style (C:\Games\A\Game.exe, /home/deck/Games/A/Game.exe or
// /Applications/GameA.app).
func exeEntry(t *testing.T, h *uitest.Harness) *widget.Entry {
	t.Helper()
	for _, e := range uitest.PageWidgets[*widget.Entry](h) {
		if strings.HasSuffix(e.PlaceHolder, "A/Game.exe") || strings.HasSuffix(e.PlaceHolder, `A\Game.exe`) ||
			strings.HasSuffix(e.PlaceHolder, "GameA.app") {
			return e
		}
	}
	require.Fail(t, "no executable entry found")
	return nil
}

// Only the name is required: a game can be added without a path, and another
// device sets its own later.
func TestEditorSaveNeedsAName(t *testing.T) {
	h := newHarness(t)
	h.Open("/new")
	saveDisabled := func() bool {
		return uitest.ButtonWith(t, uitest.PageWidgets[*widget.Button](h), "Save").Disabled()
	}
	assert.True(t, saveDisabled(), "an empty form can't be saved")

	uitest.EntryByPlaceholder(t, h, "Game name").SetText("   ")
	h.Settle()
	assert.True(t, saveDisabled(), "a blank name doesn't count")

	uitest.EntryByPlaceholder(t, h, "Game name").SetText("Game A")
	h.Settle()
	assert.False(t, saveDisabled(), "a name is enough")
}

func TestEditorCreatesGame(t *testing.T) {
	h := newHarness(t)
	h.Open("/new")

	uitest.EntryByPlaceholder(t, h, "Game name").SetText("Game A")
	exeEntry(t, h).SetText("/games/a/a.exe")
	uitest.EntryByPlaceholder(t, h, "-windowed").SetText("-windowed -nosplash")
	uitest.EntryByPlaceholder(t, h, "Game-Win64-Shipping.exe").SetText("Game-Win64-Shipping.exe")
	uitest.EntryByPlaceholder(t, h, "{appdata}/Game/Saves").SetText("{home}/GameA/Saves")
	uitest.EntryByPlaceholder(t, h, "*.sav").SetText("*.sav\n  *.dat \n")
	uitest.EntryByPlaceholder(t, h, "*.log\nshadercache/**").SetText("*.log")
	for _, e := range uitest.PageWidgets[*widget.Entry](h) {
		if e.MultiLine && e.PlaceHolder == "" { // the environment variables
			e.SetText("A=1\nB = two\nnot-a-pair")
		}
	}
	h.Settle()
	uitest.TapButton(t, h, "Save")
	h.Settle()

	games := h.Core.Library.List()
	require.Len(t, games, 1)
	g := games[0]
	assert.Equal(t, "Game A", g.Name)
	require.Len(t, g.Slots, 1)
	assert.Equal(t, "Saves", g.Slots[0].Label)
	assert.Equal(t, []string{"*.sav", "*.dat"}, g.Slots[0].Include, "blank lines and spaces are dropped")
	assert.Equal(t, []string{"*.log"}, g.Slots[0].Exclude)

	dev, ok := g.Device(h.Core.DeviceID())
	require.True(t, ok)
	assert.Equal(t, "/games/a/a.exe", dev.Exe)
	assert.Equal(t, "-windowed -nosplash", dev.Args)
	assert.Equal(t, "Game-Win64-Shipping.exe", dev.WaitProcess)
	assert.Equal(t, "{home}/GameA/Saves", dev.Paths[g.Slots[0].Key])
	assert.Equal(t, map[string]string{"A": "1", "B": " two"}, dev.Env, "lines without = are ignored; the key is trimmed, the value kept")

	assert.Contains(t, h.Toasts(), "Saved")
	assert.Contains(t, h.Shell.Route(), g.ID, "the game's page opens after saving")
}

func TestEditorSecondSaveLocation(t *testing.T) {
	h := newHarness(t)
	h.Open("/new")
	uitest.EntryByPlaceholder(t, h, "Game name").SetText("Game A")
	uitest.EntryByPlaceholder(t, h, "{appdata}/Game/Saves").SetText("{home}/one")
	uitest.TapButton(t, h, "Add")
	h.Settle()

	var paths []*widget.Entry
	for _, e := range uitest.PageWidgets[*widget.Entry](h) {
		if e.PlaceHolder == "{appdata}/Game/Saves" {
			paths = append(paths, e)
		}
	}
	require.Len(t, paths, 2, "Add creates another save location")
	paths[1].SetText("{home}/two")
	h.Settle()
	uitest.TapButton(t, h, "Save")
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
	h.Open("/new")
	uitest.EntryByPlaceholder(t, h, "Game name").SetText("Game A")
	uitest.EntryByPlaceholder(t, h, "{appdata}/Game/Saves").SetText("{home}/one")
	h.Settle()
	uitest.TapButton(t, h, "Save")
	h.Settle()
	g := h.Core.Library.List()[0]

	h.Open("/edit/" + g.ID)
	name := uitest.EntryByPlaceholder(t, h, "Game name")
	assert.Equal(t, "Game A", name.Text, "the form starts with the saved values")
	name.SetText("Game A (GOTY)")
	h.Settle()
	uitest.TapButton(t, h, "Save")
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
	uitest.EntryByPlaceholder(t, h, "Game name").SetText("Game A")
	h.Settle()

	h.Shell.Navigate("/settings")
	h.Settle()
	assert.Equal(t, "/new", h.Shell.Route(), "still on the editor")
	uitest.TapOverlayButton(t, h, "Keep editing")
	h.Settle()
	assert.Equal(t, "Game A", uitest.EntryByPlaceholder(t, h, "Game name").Text)

	h.Shell.Navigate("/settings")
	h.Settle()
	uitest.TapOverlayButton(t, h, "Discard changes")
	h.Settle()
	assert.Equal(t, "/settings", h.Shell.Route())
	assert.Empty(t, h.Core.Library.List())
}

// An emulator added from the game editor is set up in a dialog and joins the
// game, and what was typed in the editor stays.
func TestEditorAddsANewEmulatorInADialog(t *testing.T) {
	h := newHarness(t)
	h.Open("/new/emulator")
	uitest.EntryByPlaceholder(t, h, "Game name").SetText("Zelda")
	uitest.TapButton(t, h, "New emulator…")
	h.Settle()
	for _, e := range uitest.OverlayWidgets[*widget.Entry](h) {
		switch e.PlaceHolder {
		case "Emulator name (e.g. Eden)":
			e.SetText("Eden")
		case `C:\Emulators\Eden\eden.exe`, "/home/deck/Applications/eden.AppImage", "/Applications/Eden.app":
			e.SetText("eden.exe")
		}
	}
	uitest.TapOverlayButton(t, h, "Save")
	h.Settle()
	assert.Empty(t, h.Shell.Window().Canvas().Overlays().List(), "the dialog closed")
	assert.Equal(t, "/new/emulator", h.Shell.Route(), "still in the editor")
	e := h.Shell.Page().(*editor.Page)
	assert.Equal(t, "Zelda", e.Name())
	assert.Equal(t, []string{"Eden"}, e.EmulatorNames())
	require.Len(t, h.Shell.Emus().List(), 1)
}
