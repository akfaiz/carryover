// Package store holds the state the whole GUI shares: the games and settings,
// the status and transfer progress of play sessions and syncs, and the progress
// of "Sync all". It is the one place that listens to services.Bus for them.
//
// All of it is read and written on the UI goroutine only (the toolkit's, handed
// in to New as onUI; the store itself imports no GUI toolkit). The fields are
// exported for reading; change them through the methods, which tell the
// subscribers what changed. Pages and view models subscribe to topics instead
// of being called by whoever changed the state.
package store

import (
	"github.com/akfaiz/carryover/internal/app/appdir"
	"github.com/akfaiz/carryover/internal/cloudsync/syncer"
	"github.com/akfaiz/carryover/services"
)

// Topic says which part of the state changed.
type Topic uint8

const (
	Library  Topic = iota // games, settings or the trash count were reloaded
	Status                // the status of a play session changed
	Progress              // a sync transfer moved on
	SyncAll               // "Sync all" started, moved on or ended
)

// SyncAllState is the "Sync all" run, which outlives the page that started it.
type SyncAllState struct {
	Running    bool
	Cancelling bool // asked to stop; waiting for it to end
	Progress   services.SyncAllProgress
}

// Loader reads what a refresh shows. It runs off the UI goroutine.
type Loader struct {
	Games      func() []services.GameView
	Settings   func() appdir.Settings
	TrashCount func() int
}

// Store is the shared state. See the package comment for the threading rules.
type Store struct {
	Games      []services.GameView
	Settings   appdir.Settings
	Loaded     bool // the first refresh has finished
	TrashCount int  // deleted games and emulators that can be restored

	// Status is the play-session status by game id, and by sync key for the
	// session of one emulator profile. Progress is the latest transfer event by
	// sync key.
	Status   map[string]services.Status
	Progress map[string]syncer.Event
	SyncAll  SyncAllState

	load Loader
	onUI func(func()) // runs work on the UI goroutine
	// seq numbers the refreshes: only the newest one's result is stored, so a
	// slow older one can't replace fresher data.
	seq  int
	subs []subscription
	next int
}

type subscription struct {
	id int
	fn func(Topic)
}

// New returns an empty store that refreshes through load. onUI runs work on
// the UI goroutine (kit.OnUI in the GUI); nil runs it in place, for tests.
func New(load Loader, onUI func(func())) *Store {
	if onUI == nil {
		onUI = func(f func()) { f() }
	}
	return &Store{
		Status:   map[string]services.Status{},
		Progress: map[string]syncer.Event{},
		load:     load,
		onUI:     onUI,
	}
}

// Attach feeds the store from the services' events. The callbacks come on any
// goroutine; the store hops to the UI goroutine itself.
func (s *Store) Attach(bus *services.Bus) {
	bus.OnLibrary(func(string) { s.onUI(s.Refresh) })
	bus.OnStatus(func(st services.Status) {
		s.onUI(func() {
			s.Status[st.GameID] = st
			if st.Key != "" {
				s.Status[st.Key] = st // the session of one emulator profile
			}
			s.Publish(Status)
		})
	})
	bus.OnSync(func(ev syncer.Event) {
		s.onUI(func() {
			s.Progress[ev.GameID] = ev
			s.Publish(Progress)
		})
	})
	bus.OnSyncAll(func(p services.SyncAllProgress) {
		s.onUI(func() {
			s.SyncAll.Progress = p
			s.Publish(SyncAll)
		})
	})
}

// Subscribe calls fn on the UI goroutine whenever a topic is published, and
// returns the function that stops it. A page subscribes when it opens and
// unsubscribes when it is left.
func (s *Store) Subscribe(fn func(Topic)) (unsubscribe func()) {
	s.next++
	id := s.next
	s.subs = append(s.subs, subscription{id, fn})
	return func() {
		for i, sub := range s.subs {
			if sub.id == id {
				s.subs = append(s.subs[:i:i], s.subs[i+1:]...)
				return
			}
		}
	}
}

// Publish tells the subscribers that a topic changed. Subscribers added or
// removed while it runs take effect from the next call.
func (s *Store) Publish(t Topic) {
	for _, sub := range append([]subscription(nil), s.subs...) {
		sub.fn(t)
	}
}

// Refresh reloads the games, settings and trash count in the background and
// publishes Library once they are in.
func (s *Store) Refresh() {
	s.seq++
	seq := s.seq
	go func() {
		games := s.load.Games()
		st := s.load.Settings()
		trash := s.load.TrashCount()
		s.onUI(func() {
			if seq != s.seq {
				return // a newer refresh is on its way; this result may be older
			}
			s.Games, s.Settings, s.Loaded, s.TrashCount = games, st, true, trash
			s.Publish(Library)
		})
	}()
}

// Game returns a game of the library by id.
func (s *Store) Game(id string) (services.GameView, bool) {
	for _, g := range s.Games {
		if g.ID == id {
			return g, true
		}
	}
	return services.GameView{}, false
}

// ClearProgress forgets the last transfer event of a sync key, so a new sync
// starts from an empty bar.
func (s *Store) ClearProgress(key string) { delete(s.Progress, key) }

// SetTrashCount records how many items the trash holds. It publishes nothing:
// the trash page that read the number already shows it.
func (s *Store) SetTrashCount(n int) { s.TrashCount = n }

// BeginSyncAll marks a "Sync all" as running.
func (s *Store) BeginSyncAll() {
	s.SyncAll = SyncAllState{Running: true}
	s.Publish(SyncAll)
}

// SyncAllCancelling marks the running "Sync all" as asked to stop.
func (s *Store) SyncAllCancelling() {
	s.SyncAll.Cancelling = true
	s.Publish(SyncAll)
}

// EndSyncAll marks the "Sync all" as over, however it ended.
func (s *Store) EndSyncAll() {
	s.SyncAll.Running, s.SyncAll.Cancelling = false, false
	s.Publish(SyncAll)
}
