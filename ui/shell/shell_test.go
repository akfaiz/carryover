package shell_test

import (
	"errors"
	"testing"

	"github.com/egoist/mygo/ui"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/akfaiz/carryover/services"
	"github.com/akfaiz/carryover/ui/env"
	"github.com/akfaiz/carryover/ui/internal/uitest"
	"github.com/akfaiz/carryover/ui/kit"
	"github.com/akfaiz/carryover/ui/shell"
)

// stand is a page that only says where it is.
type stand struct{ route string }

func (s stand) View(c *ui.Context) { ui.Text(c, "page "+s.route) }

func newHarness(t *testing.T) *uitest.Harness {
	t.Helper()
	return uitest.New(t, shell.Config{
		Page:         func(_ env.Env, route string) env.Page { return stand{route} },
		AddGame:      func(env.Env) {},
		SyncProblems: func(env.Env, int, []services.SyncResult) {},
	})
}

func TestShellShowsTheSidebarAndThePage(t *testing.T) {
	h := newHarness(t)
	for _, want := range []string{"Carryover", "Library", "Emulators", "Cloud", "Trash", "Settings", "page /"} {
		assert.Contains(t, h.Texts(), want)
	}
}

func TestShellNavigatesFromTheSidebar(t *testing.T) {
	h := newHarness(t)
	h.Click("Cloud")
	assert.Equal(t, "/cloud", h.Shell.Route())
	assert.Contains(t, h.Texts(), "page /cloud")
}

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
	assert.NotNil(t, s.Prompts())
	assert.NotNil(t, s.Navigator())
	assert.NotNil(t, s.Dialogs())
	assert.NotNil(t, s.Prefs())

	help := s.HelpOpen()
	help["x"] = true
	assert.True(t, s.HelpOpen()["x"], "remembered")
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
	h.Settle()
	h.Shell.ToastErr(errors.New("boom"))
	h.Shell.Toast("fine", kit.ToastOK)
	h.Settle()
	assert.Contains(t, h.Texts(), "boom")
	assert.Contains(t, h.Texts(), "fine")
}

// Guarded pages ask before the shell leaves them.
type dirty struct{ stand }

func (dirty) Dirty() bool { return true }

func TestShellAsksBeforeLeavingADirtyPage(t *testing.T) {
	h := uitest.New(t, shell.Config{
		Page: func(_ env.Env, route string) env.Page {
			if route == "/edit/x" {
				return dirty{stand{route}}
			}
			return stand{route}
		},
		AddGame:      func(env.Env) {},
		SyncProblems: func(env.Env, int, []services.SyncResult) {},
	})
	h.Open("/edit/x")
	h.Shell.Navigate("/cloud")
	h.Settle()
	assert.Equal(t, "/edit/x", h.Shell.Route())
	assert.True(t, h.Shell.Confirming().Open())
	h.Click("Keep editing")
	assert.Equal(t, "/edit/x", h.Shell.Route())

	h.Shell.Navigate("/cloud")
	h.Settle()
	h.Click("Discard changes")
	assert.Equal(t, "/cloud", h.Shell.Route())
}

func TestShellThemeFollowsTheSetting(t *testing.T) {
	h := newHarness(t)
	kit.SetPalette(kit.ThemeDark)
	h.Shell.ApplyTheme(kit.ThemeLight)
	h.Settle()
	assert.True(t, kit.LightOn)
	h.Shell.ApplyTheme(kit.ThemeDark)
	h.Settle()
	assert.False(t, kit.LightOn)
}

func TestShellTheNoCloudButtonGoesToCloud(t *testing.T) {
	h := newHarness(t)
	h.Shell.Store().Refresh()
	h.Settle()
	require.Contains(t, h.Texts(), "No cloud configured")
	h.Click("No cloud configured")
	assert.Equal(t, "/cloud", h.Shell.Route())
}
