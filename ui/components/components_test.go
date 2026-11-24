package components_test

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/akfaiz/carryover/internal/cloudsync/syncer"
	"github.com/akfaiz/carryover/internal/saves/pathvar"
	"github.com/akfaiz/carryover/services"
	"github.com/akfaiz/carryover/ui/components"
	"github.com/akfaiz/carryover/ui/env"
	"github.com/akfaiz/carryover/ui/internal/uitest"
	"github.com/akfaiz/carryover/ui/kit"
	"github.com/akfaiz/carryover/ui/shell"
)

// probe is a page that draws whatever the test sets.
type probe struct{ draw func(c *ui.Context) }

func (p *probe) View(c *ui.Context) {
	if p.draw != nil {
		p.draw(c)
	}
}

// harness shows only the probe in a shell: the widgets are tested without any
// real page.
func harness(t *testing.T) (*uitest.Harness, *probe) {
	t.Helper()
	p := &probe{}
	h := uitest.New(t, shell.Config{
		Page:         func(env.Env, string) env.Page { return p },
		AddGame:      func(env.Env) {},
		SyncProblems: func(env.Env, int, []services.SyncResult) {},
	})
	return h, p
}

func TestStatusOfEveryState(t *testing.T) {
	cases := []struct {
		g    services.GameView
		text string
		kind int
	}{
		{services.GameView{Ready: true, Status: services.GameStatus{Running: true}}, "Running", kit.BadgeInfo},
		{services.GameView{}, "Needs setup", kit.BadgeWarn},
		{services.GameView{Ready: true, Status: services.GameStatus{Conflict: true}}, "Conflict", kit.BadgeErr},
		{services.GameView{Ready: true, Status: services.GameStatus{LastError: "boom"}}, "Sync error", kit.BadgeErr},
		{services.GameView{Ready: true, Status: services.GameStatus{LocalChanges: true}}, "Not uploaded", kit.BadgeWarn},
		{services.GameView{Ready: true, Status: services.GameStatus{LastSyncAt: "2026-01-02T03:04:05Z"}}, "Synced", kit.BadgeOK},
		{services.GameView{Ready: true}, "Not synced", kit.BadgeNone},
	}
	for _, tc := range cases {
		text, kind := components.StatusOf(tc.g)
		assert.Equal(t, tc.text, text)
		assert.Equal(t, tc.kind, kind, tc.text)
	}
}

func TestStatusBadgeShowsTheStatus(t *testing.T) {
	h, p := harness(t)
	p.draw = func(c *ui.Context) {
		components.StatusBadge(c, services.GameView{Ready: true, Status: services.GameStatus{Conflict: true}})
	}
	h.Settle()
	assert.Contains(t, h.Texts(), "Conflict")
}

func TestOnlyOnNamesThePlatforms(t *testing.T) {
	assert.Equal(t, "Android only", components.OnlyOn([]string{"android"}))
	assert.Equal(t, "Windows, Linux, macOS only", components.OnlyOn([]string{"windows", "linux", "darwin"}))
	assert.Equal(t, "plan9 only", components.OnlyOn([]string{"plan9"}))
}

func TestTimeAndDateFormats(t *testing.T) {
	assert.Equal(t, "never", components.TimeAgo(""))
	assert.Equal(t, "never", components.TimeAgo("not a time"))
	assert.Equal(t, "just now", components.TimeAgo(time.Now().Add(-5*time.Second).Format(time.RFC3339)))
	assert.Equal(t, "5m ago", components.TimeAgo(time.Now().Add(-5*time.Minute).Format(time.RFC3339)))
	assert.Equal(t, "3h ago", components.TimeAgo(time.Now().Add(-3*time.Hour).Format(time.RFC3339)))
	old := time.Now().AddDate(0, 0, -10)
	assert.Equal(t, old.Local().Format("2006-01-02"), components.TimeAgo(old.Format(time.RFC3339)))

	assert.Equal(t, "—", components.FmtDate(time.Time{}))
	d := time.Date(2026, 3, 4, 5, 6, 0, 0, time.Local)
	assert.Equal(t, "2026-03-04 05:06", components.FmtDate(d))
}

func TestHelpUnfoldsAndFolds(t *testing.T) {
	h, p := harness(t)
	p.draw = func(c *ui.Context) {
		ui.Column(c).Children(func() {
			components.Help(c, h.Shell, "Short summary", "The first detail", "The second detail")
			components.Help(c, h.Shell, "No details here")
		})
	}
	h.Settle()
	assert.Contains(t, h.Texts(), "Short summary")
	assert.Contains(t, h.Texts(), "No details here")
	assert.NotContains(t, h.Texts(), "The first detail")

	h.Click("More")
	assert.Contains(t, h.Texts(), "The first detail")
	assert.Contains(t, h.Texts(), "The second detail")
	h.Click("Less")
	assert.NotContains(t, h.Texts(), "The first detail")
}

func TestActionMenuRunsTheChosenItem(t *testing.T) {
	h, _ := harness(t)
	var ran []string
	components.ActionMenu(h.Shell, "Game actions", []components.MenuItem{
		{Key: "one", Label: "First thing", Icon: kit.IconPlay, Action: func() { ran = append(ran, "one") }},
		{Key: "two", Label: "Remove it", Danger: true, Action: func() { ran = append(ran, "two") }},
		{Key: "three", Label: "Can't do", Disabled: true, Action: func() { ran = append(ran, "three") }},
	})
	h.Settle()
	assert.Contains(t, h.Texts(), "Game actions")
	assert.True(t, h.Shell.Dialogs().Open())

	h.Click("Remove it")
	assert.Equal(t, []string{"two"}, ran)
	assert.False(t, h.Shell.Dialogs().Open(), "choosing an item closes the menu")
}

func TestConfirmDangerAsksThenActs(t *testing.T) {
	h, _ := harness(t)
	var got []bool
	components.ConfirmDanger(h.Shell, components.ConfirmOpts{
		Title: "Delete game?", Message: "It goes to the trash.",
		Check: "Also delete saves", Checked: "Saves go too", KeptHint: "Saves stay in the cloud.",
		OnConfirm: func(checked bool) { got = append(got, checked) },
	})
	h.Settle()
	assert.Contains(t, h.Texts(), "Delete game?")
	assert.Contains(t, h.Texts(), "Saves stay in the cloud.")
	h.Click("Also delete saves")
	assert.Contains(t, h.Texts(), "Saves go too")
	h.Click("Delete")
	assert.Equal(t, []bool{true}, got)

	components.ConfirmDanger(h.Shell, components.ConfirmOpts{Title: "Wipe?", Message: "All of it.", Action: "Wipe all"})
	h.Settle()
	assert.Contains(t, h.Texts(), "Wipe all")
	h.Click("Cancel")
	assert.False(t, h.Shell.Confirming().Open())
}

func TestConfirmDiscardLeaves(t *testing.T) {
	h, _ := harness(t)
	left := 0
	components.ConfirmDiscard(h.Shell, func() { left++ })
	h.Settle()
	assert.Contains(t, h.Texts(), "Discard changes?")
	h.Click("Keep editing")
	assert.Zero(t, left)

	components.ConfirmDiscard(h.Shell, func() { left++ })
	h.Settle()
	h.Click("Discard changes")
	assert.Equal(t, 1, left)
}

func TestRemoveButtonRuns(t *testing.T) {
	h, p := harness(t)
	removed := 0
	p.draw = func(c *ui.Context) { components.RemoveButton(c, h.Shell, "r1", func() { removed++ }) }
	h.Settle()
	h.Click("Remove")
	assert.Equal(t, 1, removed)
}

func TestCopyFieldShowsTheText(t *testing.T) {
	h, p := harness(t)
	p.draw = func(c *ui.Context) { components.CopyField(c, h.Shell, "steam://run/1") }
	h.Settle()
	assert.Contains(t, h.Texts(), "steam://run/1")
	assert.Contains(t, h.Texts(), "Copy")
}

func TestSaveSidesMarkTheNewerAndKeepOne(t *testing.T) {
	h, p := harness(t)
	now := time.Now()
	local := syncer.Summary{Files: 2, Bytes: 2048, Newest: now}
	cloud := syncer.Summary{Files: 1, Bytes: 10, Newest: now.Add(-time.Hour), DeviceName: "Steam Deck"}
	var kept []string
	p.draw = func(c *ui.Context) {
		components.SaveSides(c, h.Shell.Navigator(), "t", local, cloud, true, func(side string) { kept = append(kept, side) })
	}
	h.Settle()
	for _, want := range []string{"This device", "Cloud", "Newer", "from Steam Deck", "Keep this device's saves", "Keep cloud saves"} {
		assert.Contains(t, h.Texts(), want)
	}
	h.Click("Keep cloud saves")
	assert.Equal(t, []string{"cloud"}, kept)

	// The cloud side is the newer one, or there is no cloud side yet, or no
	// way to keep one.
	cloud.Newest = now.Add(time.Hour)
	h.Settle()
	assert.Contains(t, h.Texts(), "Newer")
	p.draw = func(c *ui.Context) {
		components.SaveSides(c, h.Shell.Navigator(), "t", local, syncer.Summary{}, false, nil)
	}
	h.Settle()
	assert.Contains(t, h.Texts(), "no Carryover saves for this game yet")
	assert.NotContains(t, h.Texts(), "Keep cloud saves")
}

func TestPathInputTypesInsertsAndPreviews(t *testing.T) {
	h, p := harness(t)
	var changes []string
	tokens := []pathvar.Token{
		{Token: "{documents}", Description: "Documents folder"},
		{Token: "{steam}/<appid>", Value: "C:/Steam"},
	}
	in := components.NewPathInput(h.Shell, "game-1", tokens, "", "Where the saves live", func(s string) { changes = append(changes, s) })
	p.draw = func(c *ui.Context) { in.View(c, "saves") }
	h.Settle()
	assert.Contains(t, h.Texts(), "Tokens")
	assert.Contains(t, h.Texts(), "Browse…")

	// Typing is noticed once per frame.
	in.SetText("{documents}/Game")
	h.Settle()
	assert.Equal(t, []string{"{documents}/Game"}, changes)
	assert.Eventually(t, func() bool { h.Settle(); return len(h.Texts()) > 0 }, time.Second, 50*time.Millisecond)

	// A value that arrives from outside is followed, one the user typed isn't.
	in.Follow("/from/suggestion")
	assert.Equal(t, "/from/suggestion", in.Text)
	in.Follow("/from/suggestion")
	assert.Equal(t, "/from/suggestion", in.Text)

	h.Click("Tokens")
	assert.True(t, h.Shell.Dialogs().Open())
	assert.Contains(t, h.Texts(), "{documents}   Documents folder")
	h.Click("{steam}/<appid>   C:/Steam")
	assert.False(t, h.Shell.Dialogs().Open())
	assert.Equal(t, "/from/suggestion{steam}/", in.Text, "<appid> is left for the user")
}

func TestPathInputWithoutTokensHasNoTokenButton(t *testing.T) {
	h, p := harness(t)
	in := components.NewPathInput(h.Shell, "g", nil, "", "", nil)
	p.draw = func(c *ui.Context) { in.View(c, "k") }
	h.Settle()
	assert.NotContains(t, h.Texts(), "Tokens")
	assert.Contains(t, h.Texts(), "Browse…")
}

func TestBrowseFromFindsTheFolderToOpenAt(t *testing.T) {
	h, _ := harness(t)
	dir := t.TempDir()
	got := make(chan string, 4)
	show := func(d string) { got <- d }

	components.BrowseFrom(h.Shell, "g", "  ", show)
	assert.Equal(t, "", <-got, "nothing typed: the dialog's own default")

	components.BrowseFrom(h.Shell, "g", dir, show)
	require.Equal(t, dir, wait(t, h, got))

	components.BrowseFrom(h.Shell, "g", "{no-such-token}/x", show)
	assert.Equal(t, "", wait(t, h, got), "a path that can't be expanded")
}

// wait settles the UI until the channel has something.
func wait(t *testing.T, h *uitest.Harness, ch chan string) string {
	t.Helper()
	for range 50 {
		h.Settle()
		select {
		case v := <-ch:
			return v
		default:
		}
	}
	t.Fatal("nothing arrived")
	return ""
}
