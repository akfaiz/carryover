package ui

import (
	"path/filepath"
	"testing"

	"fyne.io/fyne/v2/widget"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/akfaiz/carryover/services"
)

// The Steam shortcut of a game with emulators can be for the whole game or for
// any one of its emulators, not only the one shown on the page.
func TestSteamShortcutTargetsListEveryEmulator(t *testing.T) {
	t.Setenv("CARRYOVER_STEAM_ROOT", t.TempDir())
	h := newHarness(t)
	g := h.addGame("Game A", t.TempDir())
	var ids []string
	for _, name := range []string{"Eden", "Ryujinx"} {
		ev, err := h.u.emus.Save(services.EmulatorSetup{Name: name, Exe: name})
		require.NoError(t, err)
		ids = append(ids, ev.ID)
	}
	var in []services.GameEmulatorInput
	for _, id := range ids {
		in = append(in, services.GameEmulatorInput{EmulatorID: id, Slots: g.Slots, Rom: "g.nsp", Paths: map[string]string{"saves": filepath.Join(t.TempDir(), id)}})
	}
	_, err := h.u.lib.Save(services.GameInput{
		ID: g.ID, Name: g.Name, Slots: g.Slots, Device: services.DeviceInput{Paths: g.Device.Paths, Exe: g.Device.Exe}, Emulators: in,
	})
	require.NoError(t, err)
	h.u.refresh()
	h.settle()

	h.open("/game/" + g.ID)
	var shortcut *widget.Select
	for _, s := range widgetsOf[*widget.Select](h.u.page.view()) {
		for _, o := range s.Options {
			if o == "Only Eden" {
				shortcut = s
			}
		}
	}
	require.NotNil(t, shortcut, "no shortcut target selector")
	assert.Equal(t, []string{
		"The whole game: asks which emulator to use when it starts", "Only Eden", "Only Ryujinx",
	}, shortcut.Options)
	assert.Equal(t, shortcut.Options[0], shortcut.Selected, "one shortcut for the whole game is the default")
}
