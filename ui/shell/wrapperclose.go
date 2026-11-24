package shell

import (
	"log"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"syscall"
)

// wrapperClose decides what closing the session window does. Once the game
// has started, closing doesn't quit: the upload after playing and the release
// of the cloud lock happen in this process. Steam's "Stop" on Windows closes
// this window (WM_CLOSE) while the game runs, and on macOS asks the app to
// quit, which reaches here as a close request too; the session then finishes
// and quits by itself (signals: holdSignals). Before the game starts, closing
// quits.
type wrapperClose struct {
	gameID  string
	quit    func()
	started atomic.Bool
	// finished is set once the session is over: the wrapper's own Quit then
	// goes through the close handler, which would otherwise veto it.
	finished atomic.Bool
	hold     sync.Once
}

// vetoes reports whether a close request must be refused: the game runs, or
// ran, and the session has not finished yet.
func (w *wrapperClose) vetoes() bool { return w.started.Load() && !w.finished.Load() }

// finish says the session is over, so closing the window may quit.
func (w *wrapperClose) finish() { w.finished.Store(true) }

// holdSignals keeps a SIGINT, SIGTERM or SIGHUP from quitting the wrapper once
// the game runs. MyGo catches SIGINT and SIGTERM and quits the app,
// past the close handler: on macOS, Steam's Stop asks the app to quit and
// then signals it, so the wrapper quit before uploading. signal.Ignore undoes
// every earlier Notify (MyGo's too, which is installed once the app has
// started, long before the game does); the signals are then only
// logged, and the session finishes when the game exits. If Steam kills the
// wrapper instead, the session watcher finishes it.
func (w *wrapperClose) holdSignals() {
	w.hold.Do(func() {
		sigs := []os.Signal{syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP}
		signal.Ignore(sigs...)
		ch := make(chan os.Signal, 4)
		signal.Notify(ch, sigs...)
		go func() {
			for s := range ch {
				log.Printf("run %s: received %v during the session (Steam's Stop?): finishing the session first", w.gameID, s)
			}
		}()
	})
}

func (w *wrapperClose) request() {
	if !w.started.Load() {
		w.quit()
		return
	}
	log.Printf("run %s: window close requested during the session (Steam's Stop?): finishing the session first", w.gameID)
}
