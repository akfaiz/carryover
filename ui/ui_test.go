package ui

import (
	"fmt"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/akfaiz/carryover/internal/app/appdir"
	"github.com/akfaiz/carryover/internal/app/singleinstance"
	"github.com/akfaiz/carryover/internal/core"
	"github.com/akfaiz/carryover/services"
	"github.com/akfaiz/carryover/ui/internal/uitest"
)

// newHarness builds the whole application's shell: the real pages plugged in
// by appConfig.
func newHarness(t *testing.T) *uitest.Harness {
	t.Helper()
	h := uitest.New(t, appConfig())
	h.Tester.SetSize(1000, 900)
	h.Settle()
	return h
}

func TestAppConfigPlugsEverythingIn(t *testing.T) {
	cfg := appConfig()
	assert.NotEmpty(t, cfg.Icon)
	assert.NotNil(t, cfg.Page)
	assert.NotNil(t, cfg.AddGame)
	assert.NotNil(t, cfg.SyncProblems)
}

// Every route builds the page it names; anything unknown is the library.
func TestPageForRoutes(t *testing.T) {
	h := newHarness(t)
	heroic := "/new/heroic/legendary/" + url.PathEscape("Some Game/1")
	cases := map[string]string{
		"/":                          "*library.Page",
		"":                           "*library.Page",
		"/nowhere":                   "*library.Page",
		"/game":                      "*library.Page", // no id: not a game page
		"/game/abc":                  "*game.Page",
		"/edit/abc":                  "*editor.Page",
		"/edit":                      "*library.Page",
		"/new":                       "*editor.Page",
		"/new/steam/620":             "*editor.Page",
		"/new/steam":                 "*editor.Page", // plain "new"
		"/new/bottles/My%20Games/id": "*editor.Page",
		"/new/lutris/cfg":            "*editor.Page",
		"/new/emulator":              "*editor.Page",
		heroic:                       "*editor.Page",
		"/new/heroic/legendary":      "*editor.Page", // no app name: plain "new"
		"/emulators":                 "*emulators.Page",
		"/cloud":                     "*cloud.Page",
		"/trash":                     "*trash.Page",
		"/settings":                  "*settings.Page",
	}
	for route, want := range cases {
		assert.Equal(t, want, fmt.Sprintf("%T", pageFor(h.Shell, route)), route)
	}
}

// A Heroic app name that isn't valid escaping is used as it is.
func TestPageForHeroicWithBadEscape(t *testing.T) {
	h := newHarness(t)
	assert.Equal(t, "*editor.Page", fmt.Sprintf("%T", pageFor(h.Shell, "/new/heroic/legendary/100%zz")))
}

// The shell opens each of the application's pages without trouble.
func TestShellOpensEveryPage(t *testing.T) {
	h := newHarness(t)
	g := h.AddGame("Some Game", t.TempDir())
	for _, route := range []string{
		"/", "/emulators", "/cloud", "/trash", "/settings",
		"/game/" + g.ID, "/edit/" + g.ID, "/new", "/new/emulator",
	} {
		h.Open(route)
		assert.Equal(t, route, h.Shell.Route())
		assert.NotEmpty(t, h.Texts(), route)
	}
}

func TestShowSyncProblemsToasts(t *testing.T) {
	h := newHarness(t)
	showSyncProblems(h.Shell, 3, []services.SyncResult{{Name: "A"}, {Name: "B"}})
	h.Settle()
	assert.Contains(t, h.Texts(), "2 games need attention")

	showSyncProblems(h.Shell, 1, []services.SyncResult{{Name: "A"}})
	h.Settle()
	assert.Contains(t, h.Texts(), "1 game needs attention")
}

func TestNoticeTextIsCarryoverAndTheGame(t *testing.T) {
	t.Setenv(appdir.EnvHome, t.TempDir())
	c, err := core.New()
	require.NoError(t, err)
	title, body := NoticeText(c, core.Notice{Kind: core.NoticeUploaded, Title: "Some Game"})
	assert.Equal(t, "Carryover", title)
	assert.Contains(t, body, "Some Game")
}

func TestStartBuildsTheShell(t *testing.T) {
	t.Setenv(appdir.EnvHome, t.TempDir())
	c, err := core.New()
	require.NoError(t, err)
	assert.NotNil(t, start(c))
}

// A second launch asks the running instance to show itself and exits without
// opening a window.
func TestRunWithAnInstanceAlreadyRunning(t *testing.T) {
	t.Setenv(appdir.EnvHome, t.TempDir())
	c, err := core.New()
	require.NoError(t, err)
	inst, err := singleinstance.Acquire(c.Dirs.StateDir())
	require.NoError(t, err)
	defer inst.Close()
	shown := make(chan struct{}, 1)
	inst.OnShow(func() { shown <- struct{}{} })

	assert.Equal(t, 0, Run(c, "", core.RunOptions{}))
	<-shown
}
