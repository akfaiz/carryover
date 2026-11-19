package shell_test

import (
	"strings"
	"testing"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/akfaiz/carryover/internal/core"
	"github.com/akfaiz/carryover/services"
	"github.com/akfaiz/carryover/ui/env"
	"github.com/akfaiz/carryover/ui/flows"
	"github.com/akfaiz/carryover/ui/internal/uitest"
	"github.com/akfaiz/carryover/ui/kit"
	"github.com/akfaiz/carryover/ui/shell"
)

// elsewhere stands in for every page: these tests are about the window.
type elsewhere struct{}

func (elsewhere) View() fyne.CanvasObject { return widget.NewLabel("elsewhere") }

// newHarness shows a shell with stand-in pages, in an isolated data dir.
func newHarness(t *testing.T) *uitest.Harness {
	t.Helper()
	return uitest.New(t, shell.Config{
		Page:         func(env.Env, string) env.Page { return elsewhere{} },
		AddGame:      func(env.Env) {},
		SyncProblems: func(env.Env, int, []services.SyncResult) {},
	})
}

// LB and RB step through the sidebar; B goes back from a game to the library.
func TestGamepadSectionsAndBack(t *testing.T) {
	h := newHarness(t)
	g := h.AddGame("Celeste", t.TempDir())
	h.Settle()

	h.Shell.Navigator().Apply(kit.PadTabNext)
	assert.Equal(t, "/emulators", h.Shell.Route())
	h.Shell.Navigator().Apply(kit.PadTabNext)
	assert.Equal(t, "/cloud", h.Shell.Route())
	h.Shell.Navigator().Apply(kit.PadTabPrev)
	h.Shell.Navigator().Apply(kit.PadTabPrev)
	assert.Equal(t, "/", h.Shell.Route())
	h.Shell.Navigator().Apply(kit.PadTabPrev)
	assert.Equal(t, "/settings", h.Shell.Route(), "goes round")

	h.Open("/")
	h.Open("/game/" + g.ID)
	h.Shell.Navigator().Apply(kit.PadBack)
	assert.Equal(t, "/", h.Shell.Route())
	h.Shell.Navigator().Apply(kit.PadBack)
	assert.Equal(t, "/", h.Shell.Route(), "B on the library stays there")
}

// A "Sync all" problem list and other dialogs close with B, not the page.
func TestGamepadClosesDialogs(t *testing.T) {
	h := newHarness(t)
	h.Open("/settings")
	flows.ShowAddGame(h.Shell)
	require.NotEmpty(t, h.Shell.Window().Canvas().Overlays().List())
	h.Shell.Navigator().Apply(kit.PadBack)
	assert.Empty(t, h.Shell.Window().Canvas().Overlays().List())
	assert.Equal(t, "/settings", h.Shell.Route())
}

// The View button switches full screen and remembers it, apart for Carryover
// started by Steam and started by hand.
func TestGamepadViewSwitchesFullScreen(t *testing.T) {
	t.Setenv("SteamGameId", "")
	t.Setenv("SteamClientLaunch", "")
	t.Setenv("XDG_CURRENT_DESKTOP", "")
	t.Setenv("GAMESCOPE_WAYLAND_DISPLAY", "")
	h := newHarness(t)
	prefs := h.Shell.App().Preferences()
	require.False(t, h.Shell.Window().FullScreen())
	h.Shell.Navigator().Apply(kit.PadView)
	assert.True(t, h.Shell.Window().FullScreen())
	assert.True(t, prefs.Bool(shell.PrefFullScreen))
	assert.False(t, prefs.Bool(shell.PrefFullScreenSteam), "started by hand: Steam's choice stays")
	h.Shell.Navigator().Apply(kit.PadView)
	assert.False(t, h.Shell.Window().FullScreen())
	assert.False(t, prefs.Bool(shell.PrefFullScreen))

	t.Setenv("SteamGameId", "123")
	t.Setenv("SteamTenfoot", "1")
	h.Shell.Window().SetFullScreen(false)
	h.Shell.ApplyFullScreen()
	assert.True(t, h.Shell.Window().FullScreen(), "from Big Picture it starts full screen")
	h.Shell.ToggleFullScreen()
	h.Shell.ApplyFullScreen()
	assert.False(t, h.Shell.Window().FullScreen(), "until switched off")
}

// exitButton is the sidebar's Exit button, or nil.
func exitButton(h *uitest.Harness) *widget.Button {
	for _, b := range uitest.WidgetsOf[*widget.Button](h.Shell.Sidebar()) {
		if b.Text == "Exit" {
			return b
		}
	}
	return nil
}

// Exit is in the sidebar where the window's close button can't be reached:
// full screen, Game Mode, started by Steam. It closes like the window's close
// button: asking first while a sync runs.
func TestExitButton(t *testing.T) {
	t.Setenv("SteamGameId", "")
	t.Setenv("SteamClientLaunch", "")
	t.Setenv("XDG_CURRENT_DESKTOP", "")
	t.Setenv("GAMESCOPE_WAYLAND_DISPLAY", "")
	h := newHarness(t)
	h.Settle()
	assert.Nil(t, exitButton(h), "a desktop window has its own close button")

	h.Shell.ToggleFullScreen()
	exit := exitButton(h)
	require.NotNil(t, exit, "full screen")
	assert.Contains(t, kit.Focusables(h.Shell.Window().Content()), fyne.Focusable(exit), "the D-pad reaches it")

	h.Shell.Store().SyncAll.Running = true
	test.Tap(exit)
	assert.Contains(t, uitest.OverlayText(h), "A game is still running or a sync is in progress.", "asks while a sync runs")
	h.Shell.Navigator().Apply(kit.PadBack)
	assert.Empty(t, h.Shell.Window().Canvas().Overlays().List(), "B keeps Carryover open")
	h.Shell.Store().SyncAll.Running = false

	h.Shell.ToggleFullScreen()
	assert.Nil(t, exitButton(h), "back in a window")
	t.Setenv("SteamGameId", "123")
	h.Shell.RefreshSidebar()
	assert.NotNil(t, exitButton(h), "started by Steam")
}

// Notices the session watcher saved (no notification could be shown) appear
// as toasts when Carryover opens, once.
func TestShowNotices(t *testing.T) {
	h := newHarness(t)
	now := time.Now()
	require.NoError(t, h.Core.SaveNotice(core.Notice{Time: now, Key: "a", Title: "Zelda", Kind: core.NoticeUploaded}))
	require.NoError(t, h.Core.SaveNotice(core.Notice{Time: now.Add(time.Second), Key: "b", Title: "Hades", Kind: core.NoticeFailed, Err: "offline"}))

	h.Shell.ShowNotices()
	h.Settle()
	toasts := h.Toasts()
	require.Len(t, toasts, 2)
	assert.True(t, strings.HasPrefix(toasts[0], "Zelda: ") || strings.HasPrefix(toasts[1], "Zelda: "), toasts)
	assert.Contains(t, strings.Join(toasts, "\n"), "offline")
	require.Len(t, h.Shell.Toasts().BGs, 2)

	h.Shell.ShowNotices() // already shown: nothing new
	h.Settle()
	assert.Len(t, h.Toasts(), 2)
	assert.Empty(t, h.Core.TakeNotices())
}

// Closing the launcher while a sync or a play session runs would skip the
// upload and the release of the cloud lock, so it asks first.
func TestCloseAsksWhileBusy(t *testing.T) {
	h := newHarness(t)
	u := h.Shell

	require.False(t, u.Busy())
	u.Store().SyncAll.Running = true
	require.True(t, u.Busy())
	u.ConfirmClose()
	require.Len(t, u.Window().Canvas().Overlays().List(), 1, "a confirmation while a sync runs")
}
