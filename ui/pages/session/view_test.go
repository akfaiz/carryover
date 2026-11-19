package session_test

import (
	"testing"

	"fyne.io/fyne/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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
