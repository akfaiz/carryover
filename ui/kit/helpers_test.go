package kit

import (
	"testing"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
	"github.com/stretchr/testify/require"
)

// uiQueue captures the work sent to OnUI, so a test runs it on its own
// goroutine, like the real UI thread.
type uiQueue chan func()

// useQueue routes OnUI into a queue for the rest of the test.
func useQueue(t *testing.T) uiQueue {
	t.Helper()
	test.NewApp()
	q := make(uiQueue, 64)
	prev := OnUI
	OnUI = func(f func()) { q <- f }
	t.Cleanup(func() { OnUI = prev })
	return q
}

// run runs the next queued UI work, failing if none arrives.
func (q uiQueue) run(t *testing.T) {
	t.Helper()
	select {
	case f := <-q:
		f()
	case <-time.After(5 * time.Second):
		require.Fail(t, "no UI work was queued")
	}
}

// walk calls f for o and every object under it.
func walk(o fyne.CanvasObject, f func(fyne.CanvasObject)) {
	f(o)
	switch x := o.(type) {
	case *widget.Label:
		// its renderer only repeats the text in a canvas.Text
	case *fyne.Container:
		for _, c := range x.Objects {
			walk(c, f)
		}
	case fyne.Widget:
		for _, c := range test.WidgetRenderer(x).Objects() {
			walk(c, f)
		}
	}
}

// all returns every object of type T under root.
func all[T fyne.CanvasObject](root fyne.CanvasObject) []T {
	var out []T
	walk(root, func(o fyne.CanvasObject) {
		if x, ok := o.(T); ok {
			out = append(out, x)
		}
	})
	return out
}
