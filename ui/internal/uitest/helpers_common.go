package uitest

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/rclone/rclone/fs/config"
	"github.com/stretchr/testify/require"

	"github.com/akfaiz/carryover/internal/cloudsync/cloud"
	"github.com/akfaiz/carryover/internal/library"
	"github.com/akfaiz/carryover/services"
)

// Helpers that set up the library and the cloud; they use only what both
// toolkits' Harness have (T, Core, Shell, Settle).

// AddGame adds a game with one save location at saves.
func (h *Harness) AddGame(name string, saves string) services.GameView {
	h.T.Helper()
	g, err := h.Shell.Lib().Save(services.GameInput{
		Name:  name,
		Slots: []library.SaveSlot{{Key: "saves", Label: "Saves"}},
		Device: services.DeviceInput{
			Paths: map[string]string{"saves": saves}, Exe: "/x/game.exe", LaunchMode: library.LaunchDirect,
		},
	})
	require.NoError(h.T, err)
	h.Shell.Store().Refresh()
	return g
}

// AddEmulatorGame adds a game with its own saves and two emulators.
func (h *Harness) AddEmulatorGame() (services.GameView, services.EmulatorView, services.EmulatorView) {
	h.T.Helper()
	eden, err := h.Shell.Emus().Save(services.EmulatorSetup{Name: "Eden", Exe: "eden"})
	require.NoError(h.T, err)
	ryu, err := h.Shell.Emus().Save(services.EmulatorSetup{Name: "Ryujinx", Exe: "ryujinx"})
	require.NoError(h.T, err)
	slots := []library.SaveSlot{{Key: "saves", Label: "Saves"}}
	g, err := h.Shell.Lib().Save(services.GameInput{
		Name: "Mario Kart", Slots: slots,
		Device: services.DeviceInput{Paths: map[string]string{"saves": h.T.TempDir()}, Exe: "/x/game.exe", LaunchMode: library.LaunchDirect},
		Emulators: []services.GameEmulatorInput{
			{EmulatorID: eden.ID, Slots: slots, Rom: "mk.nsp", Paths: map[string]string{"saves": h.T.TempDir()}},
			{EmulatorID: ryu.ID, Slots: slots, Rom: "mk.nsp", Paths: map[string]string{"saves": h.T.TempDir()}},
		},
	})
	require.NoError(h.T, err)
	h.Shell.Store().Refresh()
	h.Settle()
	return g, eden, ryu
}

// MustGame returns a game of the library, failing the test if it is missing.
func MustGame(t *testing.T, h *Harness, id string) services.GameView {
	t.Helper()
	g, ok := h.Shell.Store().Game(id)
	require.True(t, ok)
	return g
}

// UseLocalCloud points the app at a local folder acting as the cloud and
// returns it.
func (h *Harness) UseLocalCloud() string {
	h.T.Helper()
	config.FileSetValue("cloud", "type", "local")
	h.T.Cleanup(func() { config.DeleteRemote("cloud") })
	dir := filepath.Join(h.T.TempDir(), "cloud")
	st := h.Core.Settings.Get()
	st.Remote, st.RemoteRoot = "cloud", filepath.ToSlash(dir)
	require.NoError(h.T, h.Core.Settings.Update(st))
	h.Shell.Store().Refresh()
	return dir
}

// AddSFTPRemotes adds rclone remotes of type sftp. They never connect: the
// hosts don't exist. The caller removes them if it needs to.
func AddSFTPRemotes(t *testing.T, names ...string) {
	t.Helper()
	for _, name := range names {
		_, err := cloud.CreateRemote(context.Background(), name, "sftp", map[string]string{"host": name + ".invalid"})
		require.NoError(t, err)
	}
}
