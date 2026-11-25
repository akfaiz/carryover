package kit

import (
	"sync/atomic"

	"github.com/egoist/mygo"
)

// OnUI runs f on the UI goroutine, then redraws every window, since f changed
// state the next frame shows. A var so tests can run work inline.
var OnUI = func(f func()) {
	mygo.RunOnMain(func() {
		f()
		Redraw()
	})
}

// Redraw asks every window for a new frame.
func Redraw() {
	for _, w := range mygo.Windows() {
		w.Invalidate()
	}
}

// inflight counts Async work that has not yet handed its result to the UI.
var inflight atomic.Int64

// Pending reports how many Async calls are still working. Tests wait for it to reach
// zero instead of guessing how long background work takes.
func Pending() int64 { return inflight.Load() }

// Async runs work off the UI thread and then done on it.
func Async[T any](work func() (T, error), done func(T, error)) {
	inflight.Add(1)
	go func() {
		defer inflight.Add(-1)
		v, err := work()
		OnUI(func() { done(v, err) })
	}()
}

// Async0 is Async for work without a result.
func Async0(work func() error, done func(error)) {
	inflight.Add(1)
	go func() {
		defer inflight.Add(-1)
		err := work()
		OnUI(func() { done(err) })
	}()
}
