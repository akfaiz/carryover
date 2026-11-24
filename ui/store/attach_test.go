package store_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/akfaiz/carryover/services"
	"github.com/akfaiz/carryover/ui/env"
	"github.com/akfaiz/carryover/ui/internal/uitest"
	"github.com/akfaiz/carryover/ui/shell"
	"github.com/akfaiz/carryover/ui/store"
)

// The store follows the services' events: a library change reloads the
// games, and a sync of everything reports its progress and each game's
// transfer.
func TestStoreFollowsTheServicesEvents(t *testing.T) {
	h := uitest.New(t, shell.Config{
		Page:         func(env.Env, string) env.Page { return nil },
		AddGame:      func(env.Env) {},
		SyncProblems: func(env.Env, int, []services.SyncResult) {},
	})
	h.UseLocalCloud()
	g := h.AddGame("Game A", t.TempDir())
	st := h.Shell.Store()

	var topics []store.Topic
	st.Subscribe(func(tp store.Topic) { topics = append(topics, tp) })

	_, err := h.Shell.Sync().SyncAll()
	require.NoError(t, err)
	h.Settle()

	assert.Contains(t, topics, store.Library, "the library was reloaded")
	assert.Contains(t, topics, store.SyncAll, "the progress of 'Sync all' reached the store")
	assert.Equal(t, 1, st.SyncAll.Progress.Total)
	_, ok := st.Game(g.ID)
	assert.True(t, ok)
}
