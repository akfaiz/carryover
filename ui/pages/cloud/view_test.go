package cloud_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"
	"github.com/rclone/rclone/fs/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/akfaiz/carryover/services"
	"github.com/akfaiz/carryover/ui/env"
	"github.com/akfaiz/carryover/ui/internal/uitest"
	"github.com/akfaiz/carryover/ui/pages/cloud"
	"github.com/akfaiz/carryover/ui/shell"
)

func newHarness(t *testing.T) *uitest.Harness {
	t.Helper()
	return uitest.New(t, shell.Config{
		Page:         func(e env.Env, _ string) env.Page { return cloud.New(e) },
		AddGame:      func(env.Env) {},
		SyncProblems: func(env.Env, int, []services.SyncResult) {},
	})
}

func TestCloudSaveLocation(t *testing.T) {
	h := newHarness(t)
	uitest.AddSFTPRemotes(t, "one", "two")
	h.Open("/cloud")
	assert.Contains(t, h.Texts(), "— none (sync disabled) —")

	// The folder starts at the default; choose a remote, edit the folder, save.
	h.Click("— none (sync disabled) —")
	h.Click("two (sftp)")
	h.Shell.Navigator().Handle("cloud-root").Focus() // the folder field, at its default
	h.Settle()
	h.Tester.Key(ui.Cmd, ui.KeyA)
	h.Tester.Type(" /Games/Saves/ ")
	h.Settle()
	h.Click("Use this location")

	got := h.Core.Settings.Get()
	assert.Equal(t, "two", got.Remote)
	assert.Equal(t, "Games/Saves", got.RemoteRoot, "the folder is trimmed of spaces and slashes")
}

func TestCloudDeleteRemoteAsksFirst(t *testing.T) {
	h := newHarness(t)
	uitest.AddSFTPRemotes(t, "one", "two")
	h.Open("/cloud")
	require.Len(t, h.Shell.Cloud().Remotes(), 2)

	// Cancelling keeps the remote.
	h.Click("Delete")
	h.Click("Cancel")
	assert.Len(t, h.Shell.Cloud().Remotes(), 2, "cancelled")

	// Confirming deletes it and the list updates.
	h.Click("Delete")
	h.Click("Delete")
	h.Settle()
	assert.Len(t, h.Shell.Cloud().Remotes(), 1)
}

// The first remote set up becomes the sync location by itself.
func TestCloudFirstRemoteBecomesTheLocation(t *testing.T) {
	h := newHarness(t)
	conf := filepath.Join(t.TempDir(), "rclone.conf")
	require.NoError(t, os.WriteFile(conf, []byte("[mine]\ntype = sftp\nhost = mine.invalid\n"), 0o644))
	t.Cleanup(func() { config.DeleteRemote("mine") })
	h.Open("/cloud")
	h.Shell.Page().(*cloud.Page).ImportConf(conf)
	h.Settle()
	got := h.Core.Settings.Get()
	assert.Equal(t, "mine", got.Remote)
	assert.Equal(t, "Carryover", got.RemoteRoot)
}

// The wizard lists providers, takes a name and options, and creates the remote.
func TestCloudAddRemoteWizard(t *testing.T) {
	h := newHarness(t)
	h.Open("/cloud")
	h.Click("Add remote")
	assert.Contains(t, h.Texts(), "Choose storage provider")
	h.Click("SSH/SFTP")
	assert.Contains(t, h.Texts(), "Show all options")

	h.Click("Show all options")
	h.Click("Back")
	assert.Contains(t, h.Texts(), "Choose storage provider")
	h.Click("SSH/SFTP")
	h.Click("Continue")
	h.Settle()
	assert.NotEmpty(t, h.Shell.Cloud().Remotes())
}

func TestCloudAddRemoteCancel(t *testing.T) {
	h := newHarness(t)
	h.Open("/cloud")
	h.Click("Add remote")
	h.Click("Cancel")
	assert.False(t, h.Shell.Dialogs().Open())
}
