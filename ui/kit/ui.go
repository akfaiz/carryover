package kit

import (
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

// Async runs work off the UI thread and then done on it.
func Async[T any](work func() (T, error), done func(T, error)) {
	go func() {
		v, err := work()
		OnUI(func() { done(v, err) })
	}()
}

// Async0 is Async for work without a result.
func Async0(work func() error, done func(error)) {
	go func() {
		err := work()
		OnUI(func() { done(err) })
	}()
}
