package session_test

import (
	"testing"

	"github.com/egoist/mygo/ui"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/akfaiz/carryover/internal/cloudsync/syncer"
	"github.com/akfaiz/carryover/services"
	"github.com/akfaiz/carryover/ui/env"
	"github.com/akfaiz/carryover/ui/internal/uitest"
	"github.com/akfaiz/carryover/ui/kit"
	"github.com/akfaiz/carryover/ui/pages/session"
	"github.com/akfaiz/carryover/ui/shell"
	"github.com/akfaiz/carryover/ui/store"
)

func newHarness(t *testing.T) (*uitest.Harness, env.Page) {
	t.Helper()
	var page env.Page
	h := uitest.New(t, shell.Config{
		Page:         func(e env.Env, _ string) env.Page { page = session.New(e); return page },
		AddGame:      func(env.Env) {},
		SyncProblems: func(env.Env, int, []services.SyncResult) {},
	})
	h.Open("/")
	require.NotNil(t, page)
	return h, page
}

func TestSessionPageRenders(t *testing.T) {
	h, page := newHarness(t)
	assert.Contains(t, h.Texts(), "Starting…")
	assert.Nil(t, page.(env.PadFocuser).DefaultFocus()) // Cancel starts hidden

	// no status for this game yet: nothing changes
	page.(env.Stateful).StateChanged(store.Status)
	h.Settle()
	assert.Contains(t, h.Texts(), "Starting…")
}

// A status for the wrapper's game drives the message, the progress bar and the
// Cancel button. The test wrapper has no game, so its status is under "".
func TestSessionPageFollowsTheStatus(t *testing.T) {
	h, page := newHarness(t)
	st := h.Shell.Store()
	set := func(s services.Status) {
		st.Status[""] = s
		page.(env.Stateful).StateChanged(store.Status)
		h.Settle()
	}

	// the sync before the game: Cancel is offered; the first pad press lands on it
	st.Progress[""] = syncer.Event{Message: "Downloading", Bytes: 10, TotalBytes: 100}
	set(services.Status{Phase: "sync", Message: "Checking the cloud…", BeforePlay: true})
	assert.Contains(t, h.Texts(), "Checking the cloud…")
	assert.Contains(t, h.Texts(), "Cancel and don't play")
	assert.Contains(t, h.Texts(), "Downloading · 10 B / 100 B")
	assert.NotNil(t, page.(env.PadFocuser).DefaultFocus())

	// the game runs: the bar and Cancel go
	set(services.Status{Phase: "playing", Message: "Playing"})
	assert.Contains(t, h.Texts(), "Playing")
	assert.NotContains(t, h.Texts(), "Cancel and don't play")
	assert.Nil(t, page.(env.PadFocuser).DefaultFocus())

	// the sync after the game cancels only the upload
	set(services.Status{Phase: "sync", Message: "Uploading saves"})
	assert.Contains(t, h.Texts(), "Cancel upload")

	// a failure is shown as one
	set(services.Status{Phase: "error", Message: "offline"})
	assert.Contains(t, h.Texts(), "offline")
}

// A dialog (a prompt) does not share the window with the session's text.
func TestSessionPageClearsForADialog(t *testing.T) {
	h, _ := newHarness(t)
	require.Contains(t, h.Texts(), "Starting…")

	d := h.Shell.Dialogs().Show(&kit.Dialog{Title: "A question", Body: func(*ui.Context, *kit.Navigator) {}})
	h.Settle()
	assert.Contains(t, h.Texts(), "A question")
	assert.NotContains(t, h.Texts(), "Starting…")
	assert.NotContains(t, h.Texts(), "Sync saves")

	d.Close()
	h.Settle()
	assert.Contains(t, h.Texts(), "Starting…")
}

// The pad reaches Cancel, and the first A only focuses it.
func TestSessionPadReachesCancel(t *testing.T) {
	h, page := newHarness(t)
	h.Shell.Store().Status[""] = services.Status{Phase: "sync", Message: "Uploading saves"}
	page.(env.Stateful).StateChanged(store.Status)
	h.Settle()
	require.Contains(t, h.Texts(), "Cancel upload")

	h.Shell.Navigator().Press(kit.PadAccept)
	h.Settle()
	assert.True(t, h.Tester.Focused("Cancel upload"), "the first A focuses Cancel")
	assert.Contains(t, h.Texts(), "Cancel upload", "and does not cancel")
}
