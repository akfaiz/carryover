// Package uitest is the headless test harness for the GUI: a shell on Fyne's
// test driver in an isolated data dir, with the UI thread played by the test's
// own goroutine. Page packages use it to test their views without importing
// the application's root package (which imports them).
package uitest

import (
	"sync/atomic"
	"testing"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
	"github.com/stretchr/testify/require"

	"github.com/akfaiz/carryover/internal/app/appdir"
	"github.com/akfaiz/carryover/internal/core"
	"github.com/akfaiz/carryover/services"
	"github.com/akfaiz/carryover/ui/kit"
	"github.com/akfaiz/carryover/ui/shell"
)

// activeQueue is where UI work queued from other goroutines goes: the running
// test's queue. kit.OnUI is assigned once, here, and never again: toasts leave
// timers behind that read it after their test ended, so reassigning it per test
// would race with them. Late work lands in an old queue that nobody drains.
var activeQueue atomic.Pointer[chan func()]

func init() {
	kit.OnUI = func(f func()) {
		if q := activeQueue.Load(); q != nil {
			*q <- f
			return
		}
		fyne.Do(f)
	}
}

// UseQueue makes q the queue UI work goes to, for tests that build their own
// UI without a Harness.
func UseQueue(q *chan func()) { activeQueue.Store(q) }

// Harness is a shell on Fyne's headless test driver in an isolated data dir.
// UI work queued from other goroutines runs in Settle, on the test's own
// goroutine, like the real app's single UI thread.
type Harness struct {
	T     *testing.T
	Core  *core.Core
	Shell *shell.Shell
	queue chan func()
}

// New builds a shell with cfg and opens its main window (1000x700).
func New(t *testing.T, cfg shell.Config) *Harness {
	t.Helper()
	t.Setenv(appdir.EnvHome, t.TempDir())
	c, err := core.New()
	require.NoError(t, err)
	h := &Harness{T: t, Core: c, queue: make(chan func(), 1024)}
	UseQueue(&h.queue)
	h.Shell = shell.New(c, test.NewApp(), services.NewBus(), cfg)
	h.Shell.BuildMain()
	h.Shell.Window().Resize(fyne.NewSize(1000, 700))
	return h
}

// Settle runs queued UI work until things go quiet.
func (h *Harness) Settle() {
	h.T.Helper()
	for idle := time.After(300 * time.Millisecond); ; {
		select {
		case f := <-h.queue:
			f()
			idle = time.After(300 * time.Millisecond)
			continue
		case <-idle:
		}
		break
	}
}

// Open goes to a route and settles.
func (h *Harness) Open(route string) {
	h.T.Helper()
	h.Shell.Navigate(route)
	h.Settle()
}

// Toasts lists the texts of the toasts on screen.
func (h *Harness) Toasts() []string {
	var out []string
	for _, l := range WidgetsOf[*widget.Label](h.Shell.Toasts().Box) {
		out = append(out, l.Text)
	}
	return out
}

// WidgetsOf returns every widget of type T under root, in tree order.
func WidgetsOf[T fyne.CanvasObject](root fyne.CanvasObject) []T {
	var out []T
	var walk func(o fyne.CanvasObject)
	walk = func(o fyne.CanvasObject) {
		if x, ok := o.(T); ok {
			out = append(out, x)
		}
		switch x := o.(type) {
		case *fyne.Container:
			for _, c := range x.Objects {
				walk(c)
			}
		case fyne.Widget:
			for _, c := range test.WidgetRenderer(x).Objects() {
				walk(c)
			}
		}
	}
	walk(root)
	return out
}

// PageWidgets returns the widgets of type T in the window.
func PageWidgets[T fyne.CanvasObject](h *Harness) []T {
	return WidgetsOf[T](h.Shell.Window().Canvas().Content())
}

// OverlayWidgets returns the widgets of type T in open dialogs and popups.
func OverlayWidgets[T fyne.CanvasObject](h *Harness) []T {
	var out []T
	for _, o := range h.Shell.Window().Canvas().Overlays().List() {
		out = append(out, WidgetsOf[T](o)...)
	}
	return out
}
