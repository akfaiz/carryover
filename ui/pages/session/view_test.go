package session_test

import (
	"testing"

	"fyne.io/fyne/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/akfaiz/carryover/internal/cloudsync/syncer"
	"github.com/akfaiz/carryover/services"
	"github.com/akfaiz/carryover/ui/env"
	"github.com/akfaiz/carryover/ui/internal/uitest"
	"github.com/akfaiz/carryover/ui/pages/session"
	"github.com/akfaiz/carryover/ui/shell"
	"github.com/akfaiz/carryover/ui/store"
)

func TestSessionPageRenders(t *testing.T) {
	var page env.Page
	h := uitest.New(t, shell.Config{
		Page:         func(e env.Env, _ string) env.Page { page = session.New(e); return page },
		AddGame:      func(env.Env) {},
		SyncProblems: func(env.Env, int, []services.SyncResult) {},
	})
	h.Open("/")
	require.NotNil(t, page)
	assert.Contains(t, uitest.LabelTexts(h), "Starting…")

	type focuser interface{ DefaultFocus() fyne.Focusable }
	assert.Nil(t, page.(focuser).DefaultFocus()) // Cancel starts hidden

	// no status for this game yet: nothing changes
	page.(interface{ StateChanged(store.Topic) }).StateChanged(store.Status)
	assert.Contains(t, uitest.LabelTexts(h), "Starting…")
}

// A status for the wrapper's game drives the message, the progress bar and the
// Cancel button. The test wrapper has no game, so its status is under "".
func TestSessionPageFollowsTheStatus(t *testing.T) {
	var page env.Page
	h := uitest.New(t, shell.Config{
		Page:         func(e env.Env, _ string) env.Page { page = session.New(e); return page },
		AddGame:      func(env.Env) {},
		SyncProblems: func(env.Env, int, []services.SyncResult) {},
	})
	h.Open("/")
	st := h.Shell.Store()
	set := func(s services.Status) {
		st.Status[""] = s
		page.(env.Stateful).StateChanged(store.Status)
		h.Settle()
	}

	// the sync before the game: Cancel is offered and takes the pad focus
	st.Progress[""] = syncer.Event{Message: "Downloading", Bytes: 10, TotalBytes: 100}
	set(services.Status{Phase: "sync", Message: "Checking the cloud…", BeforePlay: true})
	assert.Contains(t, uitest.LabelTexts(h), "Checking the cloud…")
	assert.Contains(t, uitest.ButtonTextsOnPage(h), "Cancel and don't play")
	assert.NotNil(t, page.(env.PadFocuser).DefaultFocus())

	// the game runs: the bar and Cancel go
	set(services.Status{Phase: "playing", Message: "Playing"})
	assert.Contains(t, uitest.LabelTexts(h), "Playing")
	assert.Nil(t, page.(env.PadFocuser).DefaultFocus())

	// the sync after the game cancels only the upload
	set(services.Status{Phase: "sync", Message: "Uploading saves"})
	assert.Contains(t, uitest.ButtonTextsOnPage(h), "Cancel upload")

	// a failure is shown as one
	set(services.Status{Phase: "error", Message: "offline"})
	assert.Contains(t, uitest.LabelTexts(h), "offline")
}
