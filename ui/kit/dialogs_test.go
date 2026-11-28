package kit

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// A dialog is as wide as it asks for, but never wider than the window less a
// margin on each side.
func TestDialogWidthFitsTheWindow(t *testing.T) {
	assert.InDelta(t, dialogWideWidth, (&Dialog{Wide: true}).width(2000), 0.001)
	assert.InDelta(t, 886-2*dialogMargin, (&Dialog{Wide: true}).width(886), 0.001)
	assert.InDelta(t, dialogWidth, (&Dialog{}).width(886), 0.001)
	assert.InDelta(t, 560, (&Dialog{Width: 560}).width(900), 0.001)
	assert.InDelta(t, 0, (&Dialog{}).width(10), 0.001, "never negative")
}
