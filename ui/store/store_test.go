package store

import (
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/akfaiz/carryover/internal/app/appdir"
	"github.com/akfaiz/carryover/internal/cloudsync/syncer"
	"github.com/akfaiz/carryover/services"
)

// uiQueue stands in for the UI goroutine: work handed to the store's onUI
// waits until the test runs it.
func uiQueue() (chan func(), func(func())) {
	q := make(chan func(), 8)
	return q, func(f func()) { q <- f }
}

func TestSubscribeAndUnsubscribe(t *testing.T) {
	s := New(Loader{}, nil)
	var got []Topic
	stop := s.Subscribe(func(tp Topic) { got = append(got, tp) })
	other := 0
	s.Subscribe(func(Topic) { other++ })

	s.Publish(Library)
	s.Publish(Status)
	stop()
	s.Publish(Progress)

	assert.Equal(t, []Topic{Library, Status}, got, "stopped subscriber hears nothing more")
	assert.Equal(t, 3, other, "the other subscriber is unaffected")
	stop() // stopping twice is harmless
}

func TestUnsubscribeWhilePublishing(t *testing.T) {
	s := New(Loader{}, nil)
	var stop func()
	calls := 0
	stop = s.Subscribe(func(Topic) { calls++; stop() })
	s.Subscribe(func(Topic) { calls++ })

	s.Publish(Library)
	s.Publish(Library)

	assert.Equal(t, 3, calls, "both hear the first, only the second the next")
}

func TestRefreshLoadsAndPublishes(t *testing.T) {
	q, onUI := uiQueue()
	s := New(Loader{
		Games:      func() []services.GameView { return []services.GameView{{ID: "a", Name: "A"}} },
		Settings:   func() appdir.Settings { return appdir.Settings{Remote: "drive"} },
		TrashCount: func() int { return 2 },
	}, onUI)
	var topics []Topic
	s.Subscribe(func(tp Topic) { topics = append(topics, tp) })

	s.Refresh()
	(<-q)()

	assert.True(t, s.Loaded)
	assert.Equal(t, "drive", s.Settings.Remote)
	assert.Equal(t, 2, s.TrashCount)
	assert.Equal(t, []Topic{Library}, topics)
	g, ok := s.Game("a")
	require.True(t, ok)
	assert.Equal(t, "A", g.Name)
	_, ok = s.Game("missing")
	assert.False(t, ok)
}

// A refresh that finishes after a newer one was started must not replace what
// the newer one stores.
func TestRefreshKeepsTheNewestResult(t *testing.T) {
	q, onUI := uiQueue()
	entered := make(chan struct{}, 2)
	release := make(chan struct{})
	var calls atomic.Int32
	s := New(Loader{
		Games: func() []services.GameView {
			n := calls.Add(1)
			entered <- struct{}{}
			if n == 1 {
				<-release // the first refresh is the slow one
				return []services.GameView{{ID: "old"}}
			}
			return []services.GameView{{ID: "new"}}
		},
		Settings:   func() appdir.Settings { return appdir.Settings{} },
		TrashCount: func() int { return 0 },
	}, onUI)
	published := 0
	s.Subscribe(func(Topic) { published++ })

	s.Refresh()
	<-entered // the first one is loading, held back
	s.Refresh()
	<-entered
	second := <-q
	close(release)
	first := <-q

	second()
	first() // late and stale
	assert.Equal(t, 1, published, "the stale result is dropped")
	_, ok := s.Game("new")
	assert.True(t, ok)
	_, ok = s.Game("old")
	assert.False(t, ok)
}

func TestSyncAllLifecycle(t *testing.T) {
	s := New(Loader{}, nil)
	var topics []Topic
	s.Subscribe(func(tp Topic) { topics = append(topics, tp) })
	s.SyncAll.Progress = services.SyncAllProgress{Done: 5, Total: 9} // left over from a previous run

	s.BeginSyncAll()
	assert.True(t, s.SyncAll.Running)
	assert.Zero(t, s.SyncAll.Progress, "a new run starts from zero")

	s.SyncAllCancelling()
	assert.True(t, s.SyncAll.Cancelling)

	s.EndSyncAll()
	assert.False(t, s.SyncAll.Running)
	assert.False(t, s.SyncAll.Cancelling)
	assert.Equal(t, []Topic{SyncAll, SyncAll, SyncAll}, topics)
}

func TestClearProgressAndTrashCount(t *testing.T) {
	s := New(Loader{}, nil)
	s.Progress["k"] = syncer.Event{}
	s.ClearProgress("k")
	assert.NotContains(t, s.Progress, "k")
	s.ClearProgress("missing") // harmless

	var heard int
	s.Subscribe(func(Topic) { heard++ })
	s.SetTrashCount(3)
	assert.Equal(t, 3, s.TrashCount)
	assert.Zero(t, heard, "SetTrashCount publishes nothing")
}
