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

// The sidebar's logo is the one made for the theme that shows.
func TestShellLogoFollowsTheTheme(t *testing.T) {
	square := func(color string) []byte {
		return []byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 8 8"><rect width="8" height="8" fill="` + color + `"/></svg>`)
	}
	h := uitest.New(t, shell.Config{
		LogoDark:     square("#ff0000"),
		LogoLight:    square("#00ff00"),
		Page:         func(_ env.Env, route string) env.Page { return stand{route} },
		AddGame:      func(env.Env) {},
		SyncProblems: func(env.Env, int, []services.SyncResult) {},
	})
	middle := func() (r, g uint32) {
		box, ok := h.Tester.Find("Carryover logo")
		require.True(t, ok)
		r, g, _, _ = h.Tester.Image().At(int(box.X+box.W/2), int(box.Y+box.H/2)).RGBA()
		return r >> 8, g >> 8
	}
	before := kit.LightOn
	t.Cleanup(func() { h.Shell.ApplyTheme(map[bool]string{true: kit.ThemeLight, false: kit.ThemeDark}[before]) })

	h.Shell.ApplyTheme(kit.ThemeDark)
	h.Settle()
	r, g := middle()
	assert.Equal(t, [2]uint32{255, 0}, [2]uint32{r, g}, "dark")

	h.Shell.ApplyTheme(kit.ThemeLight)
	h.Settle()
	r, g = middle()
	assert.Equal(t, [2]uint32{0, 255}, [2]uint32{r, g}, "light")
}
