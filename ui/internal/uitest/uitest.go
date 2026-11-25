// Package uitest is the headless test harness for the GUI: a shell driven by
// MyGo's ui.Tester in an isolated data dir, with the UI thread played by the
// test's own goroutine. Page packages use it to test their views without
// importing the application's root package (which imports them).
package uitest

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/stretchr/testify/require"

	"github.com/akfaiz/carryover/internal/app/appdir"
	"github.com/akfaiz/carryover/internal/core"
	"github.com/akfaiz/carryover/services"
	"github.com/akfaiz/carryover/ui/kit"
	"github.com/akfaiz/carryover/ui/shell"
)

// activeQueue is where UI work queued from other goroutines goes: the running
// test's queue. kit.OnUI is assigned once, here, and never again: late work
// from an earlier test lands in an old queue that nobody drains.
var activeQueue atomic.Pointer[chan func()]

func init() {
	kit.OnUI = func(f func()) {
		if q := activeQueue.Load(); q != nil {
			*q <- f
		}
	}
}

// UseQueue makes q the queue UI work goes to, for tests that build their own
// UI without a Harness.
func UseQueue(q *chan func()) { activeQueue.Store(q) }

// Harness is a shell with no window in an isolated data dir, drawn by a
// Tester. UI work queued from other goroutines runs in Settle, on the test's
// own goroutine, like the real app's single UI thread.
type Harness struct {
	T      *testing.T
	Core   *core.Core
	Shell  *shell.Shell
	Tester *ui.Tester
	queue  chan func()
}

// New builds a shell with cfg and draws it at 1000x700.
func New(t *testing.T, cfg shell.Config) *Harness {
	t.Helper()
	t.Setenv(appdir.EnvHome, t.TempDir())
	c, err := core.New()
	require.NoError(t, err)
	h := &Harness{T: t, Core: c, queue: make(chan func(), 1024)}
	UseQueue(&h.queue)
	h.Shell = shell.New(c, services.NewBus(), cfg)
	h.Shell.Init()
	h.Tester = ui.NewTester(h.Shell.ViewFunc(), 1000, 700)
	h.Settle()
	// Cleanups run last-in first-out: this one runs before the data dir is removed, so
	// a refresh a service call started (a saved game publishes a library event) has
	// finished writing by then, instead of racing the removal.
	t.Cleanup(h.Settle)
	return h
}

// Settle runs queued UI work until things go quiet, then draws a frame. Quiet means
// no background work is still reading (kit.Async, a store refresh) and nothing came
// in for a moment, so a slow machine can't end it early.
func (h *Harness) Settle() {
	h.T.Helper()
	limit := time.After(30 * time.Second)
	for idle := time.After(100 * time.Millisecond); ; {
		select {
		case f := <-h.queue:
			f()
			idle = time.After(100 * time.Millisecond)
			continue
		case <-limit:
		case <-idle:
			if kit.Pending() > 0 || h.Shell.Store().Loading() > 0 {
				idle = time.After(10 * time.Millisecond)
				continue
			}
		}
		break
	}
	h.Tester.Frame()
	h.Tester.Frame()
}

// Open goes to a route and settles.
func (h *Harness) Open(route string) {
	h.T.Helper()
	h.Shell.Navigate(route)
	h.Settle()
}

// Texts lists every text on screen.
func (h *Harness) Texts() []string { return h.Tester.Texts() }

// Click clicks the control with the given label, then settles.
func (h *Harness) Click(label string) {
	h.T.Helper()
	require.NoError(h.T, h.Tester.Click(label))
	h.Settle()
}
