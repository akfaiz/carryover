package cloud_test

import (
	"os"
	"path/filepath"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
	"github.com/rclone/rclone/fs/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/akfaiz/carryover/services"
	"github.com/akfaiz/carryover/ui/env"
	"github.com/akfaiz/carryover/ui/internal/uitest"
	"github.com/akfaiz/carryover/ui/kit"
	"github.com/akfaiz/carryover/ui/pages/cloud"
	"github.com/akfaiz/carryover/ui/shell"
)

// newHarness shows only the Cloud page in a shell, in an isolated data dir.
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

	sel := uitest.PageWidgets[*widget.Select](h)
	require.Len(t, sel, 1)
	assert.Equal(t, "— none (sync disabled) —", sel[0].Selected)
	testBtn := uitest.ButtonWith(t, uitest.PageWidgets[*widget.Button](h), "Test connection")
	assert.True(t, testBtn.Disabled(), "no remote to test")

	sel[0].SetSelected("two (sftp)")
	assert.False(t, testBtn.Disabled(), "a remote is selected")
	var root *widget.Entry
	for _, e := range uitest.PageWidgets[*widget.Entry](h) {
		if e.Text == "Carryover" {
			root = e
		}
	}
	require.NotNil(t, root, "the folder field starts at the default")
	root.SetText(" /Games/Saves/ ")
	uitest.TapButton(t, h, "Use this location")
	h.Settle()

	got := h.Core.Settings.Get()
	assert.Equal(t, "two", got.Remote)
	assert.Equal(t, "Games/Saves", got.RemoteRoot, "the folder is trimmed of spaces and slashes")
	assert.Contains(t, h.Toasts(), "Cloud location saved")
}

func TestCloudBlankFolderFallsBackToDefault(t *testing.T) {
	h := newHarness(t)
	uitest.AddSFTPRemotes(t, "one")
	h.Open("/cloud")
	uitest.PageWidgets[*widget.Select](h)[0].SetSelected("one (sftp)")
	for _, e := range uitest.PageWidgets[*widget.Entry](h) {
		if e.Text == "Carryover" {
			e.SetText("  ")
		}
	}
	uitest.TapButton(t, h, "Use this location")
	h.Settle()
	assert.Equal(t, "Carryover", h.Core.Settings.Get().RemoteRoot)
}

func TestCloudDeleteRemoteAsksFirst(t *testing.T) {
	h := newHarness(t)
	uitest.AddSFTPRemotes(t, "one", "two")
	h.Open("/cloud")
	require.Len(t, h.Shell.Cloud().Remotes(), 2)

	// Each row has a Delete button; the first one is "one".
	deletes := 0
	var first *widget.Button
	for _, b := range uitest.PageWidgets[*widget.Button](h) {
		if b.Text == "Delete" {
			deletes++
			if first == nil {
				first = b
			}
		}
	}
	require.Equal(t, 2, deletes)

	// Cancelling keeps the remote.
	test.Tap(first)
	h.Settle()
	uitest.TapOverlayButton(t, h, "Cancel")
	h.Settle()
	assert.Len(t, h.Shell.Cloud().Remotes(), 2, "cancelled")

	// Confirming deletes it and the list updates.
	test.Tap(first)
	h.Settle()
	uitest.TapOverlayButton(t, h, "Delete")
	h.Settle()
	remotes := h.Shell.Cloud().Remotes()
	require.Len(t, remotes, 1)
	assert.Equal(t, "two", remotes[0].Name)
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

// The remote marked "Active" follows the saved sync location: when it is saved
// again from the page, the badge moves.
func TestCloudActiveBadgeFollowsSavedRemote(t *testing.T) {
	h := newHarness(t)
	uitest.AddSFTPRemotes(t, "one", "two")
	activeRow := func() string {
		var found string
		var walk func(o fyne.CanvasObject)
		walk = func(o fyne.CanvasObject) {
			switch x := o.(type) {
			case *fyne.Container:
				// A remote row: its name label next to an "active" badge.
				var name string
				for _, c := range x.Objects {
					if l, ok := c.(*widget.Label); ok && (l.Text == "one" || l.Text == "two") {
						name = l.Text
					}
				}
				if name != "" && uitest.HasText(x, "Active") {
					found = name
				}
				for _, c := range x.Objects {
					walk(c)
				}
			case fyne.Widget:
				for _, c := range test.WidgetRenderer(x).Objects() {
					walk(c)
				}
			}
		}
		walk(h.Shell.Window().Canvas().Content())
		return found
	}

	st := h.Core.Settings.Get()
	st.Remote = "one"
	require.NoError(t, h.Core.Settings.Update(st))
	h.Shell.Store().Refresh()
	h.Open("/cloud")
	require.Equal(t, "one", activeRow())

	// Pick "two" and save, as the user does.
	h.Shell.Page().(*cloud.Page).UseLocation("two")
	h.Settle()
	require.Equal(t, "two", activeRow())
}

// The wizard lists providers, takes a name and options, and creates the remote.
func TestCloudAddRemoteWizard(t *testing.T) {
	h := newHarness(t)
	h.Open("/cloud")
	uitest.TapButton(t, h, "＋ Add remote")
	h.Settle()
	assert.Contains(t, uitest.ButtonTexts(h), "Cancel")

	var sftp *kit.Tappable
	for _, tp := range uitest.OverlayWidgets[*kit.Tappable](h) {
		if uitest.HasText(tp, "sftp") {
			sftp = tp
		}
	}
	require.NotNil(t, sftp, "SFTP provider tile")
	test.Tap(sftp)
	h.Settle()
	assert.Contains(t, uitest.ButtonTexts(h), "Back")
	assert.Contains(t, uitest.ButtonTexts(h), "Show all options")

	uitest.TapOverlayButton(t, h, "Show all options")
	h.Settle()
	uitest.TapOverlayButton(t, h, "Back")
	h.Settle()
	test.Tap(sftp)
	h.Settle()
	uitest.TapOverlayButton(t, h, "Continue")
	h.Settle()
	assert.NotEmpty(t, h.Shell.Cloud().Remotes())
}

func TestCloudAddRemoteCancel(t *testing.T) {
	h := newHarness(t)
	h.Open("/cloud")
	uitest.TapButton(t, h, "＋ Add remote")
	h.Settle()
	uitest.TapOverlayButton(t, h, "Cancel")
	h.Settle()
	assert.NotContains(t, uitest.ButtonTexts(h), "Cancel")
}
