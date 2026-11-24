package shell

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// Closing is refused while the game runs, but not once the session has ended:
// the wrapper's own Quit closes its window through the same handler.
func TestWrapperCloseVetoes(t *testing.T) {
	w := &wrapperClose{gameID: "g", quit: func() {}}
	assert.False(t, w.vetoes(), "before the game starts, closing quits")
	w.started.Store(true)
	assert.True(t, w.vetoes(), "while the game runs, the session finishes first")
	w.finish()
	assert.False(t, w.vetoes(), "after the session, the wrapper may quit")
}
