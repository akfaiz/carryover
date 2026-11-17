//go:build !windows

package ui

import (
	"os"
	"os/signal"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// Once the game runs, SIGTERM no longer reaches a handler installed before
// (Fyne's, which quits the app): the wrapper outlives Steam's Stop.
func TestWrapperHoldsSignals(t *testing.T) {
	defer signal.Reset(syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)
	fyneQuit := make(chan os.Signal, 1)
	signal.Notify(fyneQuit, syscall.SIGTERM) // like Fyne's catchTerm

	w := &wrapperClose{gameID: "g", quit: func() {}}
	w.holdSignals()
	w.holdSignals() // once only
	require.NoError(t, syscall.Kill(os.Getpid(), syscall.SIGTERM))

	select {
	case s := <-fyneQuit:
		t.Fatalf("the earlier handler still got %v", s)
	case <-time.After(300 * time.Millisecond):
	}
}
