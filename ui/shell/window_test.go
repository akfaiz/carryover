package shell_test

import (
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/akfaiz/carryover/internal/core"
	"github.com/akfaiz/carryover/services"
	"github.com/akfaiz/carryover/ui/env"
	"github.com/akfaiz/carryover/ui/internal/uitest"
	"github.com/akfaiz/carryover/ui/kit"
	"github.com/akfaiz/carryover/ui/shell"
)

// padPage is a page that answers the gamepad's X, Y and first press.
type padPage struct {
	route        string
	h            ui.Handle
	scroll       ui.ScrollState
	nav          func() *kit.Navigator
	searched     int
	synced       int
	defaultAsked int
	accepted     int
}

func (p *padPage) View(c *ui.Context) {
	ui.Column(c).Children(func() {
		ui.Text(c, "page "+p.route)
		p.nav().Item(&p.h, ui.Button(c.Key("go"), "Go"), func() { p.accepted++ })
	})
}
func (p *padPage) FocusSearch()             { p.searched++ }
func (p *padPage) SyncNow()                 { p.synced++ }
func (p *padPage) DefaultFocus() *ui.Handle { p.defaultAsked++; return &p.h }
func (p *padPage) Scroll() *ui.ScrollState  { return &p.scroll }

func newPadHarness(t *testing.T, cfg shell.Config) (*uitest.Harness, map[string]*padPage) {
	t.Helper()
	pages := map[string]*padPage{}
	cfg.Page = func(e env.Env, route string) env.Page {
		if p, ok := pages[route]; ok {
			return p
		}
		p := &padPage{route: route, nav: e.Navigator}
		pages[route] = p
		return p
	}
	if cfg.AddGame == nil {
		cfg.AddGame = func(env.Env) {}
	}
	if cfg.SyncProblems == nil {
		cfg.SyncProblems = func(env.Env, int, []services.SyncResult) {}
	}
	h := uitest.New(t, cfg)
	return h, pages
}

func press(h *uitest.Harness, a kit.PadAction) {
	h.Shell.Navigator().Press(a)
	h.Settle()
}

// The gamepad's buttons reach the open page: X searches, Y syncs, the first
// press starts on the page's own default control and A presses it.
func TestGamepadButtonsReachThePage(t *testing.T) {
	h, pages := newPadHarness(t, shell.Config{})
	page := pages["/"]
	require.NotNil(t, page)

	press(h, kit.PadSearch)
	assert.Equal(t, 1, page.searched)
	press(h, kit.PadSync)
	assert.Equal(t, 1, page.synced)

	press(h, kit.PadDown)
	assert.Positive(t, page.defaultAsked)
	press(h, kit.PadAccept)
	assert.Equal(t, 1, page.accepted)
}

// LB and RB step through the sidebar's places, wrapping at both ends; B goes
// back except on the library.
func TestGamepadShoulderButtonsStepSections(t *testing.T) {
	h, _ := newPadHarness(t, shell.Config{})
	press(h, kit.PadTabNext)
	assert.Equal(t, "/emulators", h.Shell.Route())
	press(h, kit.PadTabNext)
	assert.Equal(t, "/cloud", h.Shell.Route())
	press(h, kit.PadTabPrev)
	assert.Equal(t, "/emulators", h.Shell.Route())
	assert.Equal(t, []string{"/", "/emulators", "/cloud"}, h.Shell.History())

	press(h, kit.PadBack)
	assert.Equal(t, "/cloud", h.Shell.Route(), "B goes back through the history")
	h.Shell.Replace("/")
	h.Settle()
	press(h, kit.PadBack)
	assert.Equal(t, "/", h.Shell.Route(), "B does nothing on the library")

	press(h, kit.PadTabPrev)
	assert.Equal(t, "/settings", h.Shell.Route(), "LB wraps to the last place")
	press(h, kit.PadTabNext)
	assert.Equal(t, "/", h.Shell.Route(), "RB wraps to the first")
}

func TestEnvAccessorsOfTheWindow(t *testing.T) {
	added := 0
	h, _ := newPadHarness(t, shell.Config{AddGame: func(env.Env) { added++ }})
	s := h.Shell
	assert.NotNil(t, s.Lib())
	assert.Nil(t, s.Window(), "the tests have no window")
	assert.Empty(t, s.History())

	s.AddGame()
	assert.Equal(t, 1, added)

	ok := 0
	s.Confirm(kit.ConfirmOptions{Title: "Sure?", Body: "Really", Cancel: "No", OK: "Yes"}, func() { ok++ })
	h.Settle()
	assert.Contains(t, h.Texts(), "Sure?")
	h.Click("Yes")
	assert.Equal(t, 1, ok)
}

// A folder that doesn't exist is reported, not opened.
func TestOpenFolderThatIsMissingToasts(t *testing.T) {
	h, _ := newPadHarness(t, shell.Config{})
	h.Shell.OpenFolder(t.TempDir() + "/missing")
	h.Settle()
	toasted := slices.ContainsFunc(h.Texts(), func(tx string) bool {
		return strings.Contains(tx, "missing") || strings.Contains(tx, "exist")
	})
	assert.True(t, toasted, "an error toast shows: %v", h.Texts())
}

// "Sync all" with a cloud and a game syncs it and says so; started again while
// it runs, it asks to cancel instead.
func TestSyncAllSyncsAndReports(t *testing.T) {
	h, _ := newPadHarness(t, shell.Config{})
	h.UseLocalCloud()
	h.AddGame("Game A", t.TempDir())
	h.Shell.Store().Refresh()
	h.Settle()

	h.Shell.SyncAll()
	assert.True(t, h.Shell.Store().SyncAll.Running)
	h.Shell.SyncAll() // while it runs: a cancel request
	for range 40 {
		h.Settle()
		if !h.Shell.Store().SyncAll.Running {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	assert.False(t, h.Shell.Store().SyncAll.Running)
}

func TestSyncAllWithoutACloudSaysWhy(t *testing.T) {
	h, _ := newPadHarness(t, shell.Config{})
	h.Shell.SyncAll()
	for range 40 {
		h.Settle()
		if !h.Shell.Store().SyncAll.Running {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	assert.False(t, h.Shell.Store().SyncAll.Running)
}

// While "Sync all" runs the sidebar shows it on every page but the library.
func TestSidebarShowsBackgroundWork(t *testing.T) {
	h, _ := newPadHarness(t, shell.Config{})
	h.Open("/cloud")
	h.Shell.Store().BeginSyncAll()
	h.Settle()
	assert.Contains(t, h.Texts(), "Syncing…")
	h.Click("Syncing…")
	assert.Equal(t, "/", h.Shell.Route())
	h.Shell.Store().EndSyncAll()
	h.Settle()
	assert.NotContains(t, h.Texts(), "Syncing…")
}

// What a session watcher left behind while no notification could be shown is
// toasted once.
func TestShowNoticesToastsWhatWasSaved(t *testing.T) {
	h, _ := newPadHarness(t, shell.Config{})
	h.Shell.ShowNotices() // nothing saved yet
	h.Settle()

	require.NoError(t, h.Core.SaveNotice(core.Notice{Time: time.Now(), Key: "k", Title: "Game A", Kind: core.NoticeUploaded}))
	require.NoError(t, h.Core.SaveNotice(core.Notice{Time: time.Now().Add(time.Second), Key: "k", Title: "Game B", Kind: core.NoticeFailed, Err: "disk full"}))
	h.Shell.ShowNotices()
	var got []string
	require.Eventually(t, func() bool {
		h.Settle()
		got = h.Texts()
		return slices.Contains(got, "Game A: saves uploaded after Steam closed the game.")
	}, 3*time.Second, 50*time.Millisecond)
	assert.Contains(t, got, "Game B: saves could not be uploaded after Steam closed the game: disk full")
}

func TestNoticeTextInTheChosenLanguage(t *testing.T) {
	h, _ := newPadHarness(t, shell.Config{})
	title, body := shell.NoticeText(h.Core, core.Notice{Kind: core.NoticeConflict, Title: "Game A"})
	assert.Equal(t, "Carryover", title)
	assert.Contains(t, body, "Game A")
	_, body = shell.NoticeText(h.Core, core.Notice{Kind: core.NoticeOffline, Title: "Game A"})
	assert.Contains(t, body, "Game A")
	_, body = shell.NoticeText(h.Core, core.Notice{Kind: core.NoticeFailed, Title: "Game A", Err: errors.New("disk full").Error()})
	assert.Contains(t, body, "disk full")
}

// Pages told about a change in the shared state see it by topic.
func TestShellKeepsThePlayingGamesInTheSidebar(t *testing.T) {
	h, _ := newPadHarness(t, shell.Config{})
	g := h.AddGame("Some Game", t.TempDir())
	h.Shell.Store().Refresh()
	h.Settle()
	st := h.Shell.Store()
	st.Status[g.ID] = services.Status{GameID: g.ID, Phase: "playing", Running: true}
	h.Open("/cloud")
	assert.Contains(t, h.Texts(), "Playing Some Game")
	h.Click("Playing Some Game")
	assert.Equal(t, "/game/"+g.ID, h.Shell.Route())
}

func TestEscapeGoesBack(t *testing.T) {
	h, _ := newPadHarness(t, shell.Config{})
	h.Open("/cloud")
	h.Tester.Key(0, ui.KeyEscape)
	h.Settle()
	assert.Equal(t, "/", h.Shell.Route())
}
