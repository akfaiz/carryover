package shell_test

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/akfaiz/carryover/internal/core"
	"github.com/akfaiz/carryover/ui/kit"
	"github.com/akfaiz/carryover/ui/shell"
)

// The env.Env accessors hand pages the application's parts.
func TestShellEnvAccessors(t *testing.T) {
	h := newHarness(t)
	s := h.Shell
	assert.NotNil(t, s.Sync())
	assert.NotNil(t, s.Cloud())
	assert.NotNil(t, s.Steam())
	assert.NotNil(t, s.Emus())
	assert.NotNil(t, s.Sess())
	assert.NotNil(t, s.Page())
	assert.NotNil(t, s.NavButtons())
	assert.NotNil(t, s.Activity())
	assert.NotNil(t, s.Prompts())
	assert.NotNil(t, s.Sidebar())

	help := s.HelpOpen()
	help["x"] = true
	assert.True(t, s.HelpOpen()["x"], "remembered")
	s.Redraw() // the stand-in page isn't redrawable: nothing happens
	s.FocusForPad()
}

func TestShellReplaceAndBack(t *testing.T) {
	h := newHarness(t)
	h.Open("/cloud")
	h.Open("/trash")
	h.Shell.Replace("/emulators")
	h.Settle()
	assert.Equal(t, "/emulators", h.Shell.Route())
	h.Shell.Back()
	h.Settle()
	assert.Equal(t, "/cloud", h.Shell.Route(), "Back skips the replaced route")
}

func TestShellToasts(t *testing.T) {
	h := newHarness(t)
	h.Shell.ToastErr(nil)
	assert.Empty(t, h.Toasts())
	h.Shell.ToastErr(errors.New("boom"))
	h.Shell.Toast("fine", kit.ToastOK)
	h.Settle()
	assert.Contains(t, h.Toasts(), "boom")
	assert.Contains(t, h.Toasts(), "fine")
}

func TestShellOpenFolderMissing(t *testing.T) {
	h := newHarness(t)
	h.Shell.OpenFolder(t.TempDir() + "/does/not/exist")
	h.Settle()
	assert.NotEmpty(t, h.Toasts())
}

func TestShellApplyTheme(t *testing.T) {
	h := newHarness(t)
	t.Cleanup(func() { kit.SetPalette(kit.ThemeDark) })
	h.Shell.ApplyTheme(kit.ThemeLight)
	assert.True(t, kit.LightOn)
	h.Shell.ApplyTheme(kit.ThemeLight) // unchanged: nothing to rebuild
	h.Shell.ApplyTheme(kit.ThemeDark)
	assert.False(t, kit.LightOn)
}

func TestShellSyncAll(t *testing.T) {
	h := newHarness(t)
	h.UseLocalCloud()
	h.AddGame("Game A", t.TempDir())
	h.Shell.SyncAll()
	// Settle stops after a short idle spell, which a sync can outlast on a
	// busy machine (the whole suite in parallel): wait for it to say it ended.
	for deadline := time.Now().Add(10 * time.Second); h.Shell.Store().SyncAll.Running && time.Now().Before(deadline); {
		h.Settle()
	}
	assert.False(t, h.Shell.Store().SyncAll.Running, "finished")
}

func TestShellAddGameUsesConfig(t *testing.T) {
	h := newHarness(t)
	h.Shell.AddGame() // the config's AddGame is a no-op stand-in
}

func TestNoticeText(t *testing.T) {
	h := newHarness(t)
	title, body := shell.NoticeText(h.Core, core.Notice{Kind: core.NoticeUploaded, Title: "G"})
	assert.Equal(t, "Carryover", title)
	assert.Contains(t, body, "G")
	_, body = shell.NoticeText(h.Core, core.Notice{Kind: core.NoticeConflict, Title: "G"})
	assert.Contains(t, body, "G")
	_, body = shell.NoticeText(h.Core, core.Notice{Kind: core.NoticeFailed, Title: "G", Err: "oops"})
	require.Contains(t, body, "oops")
}
