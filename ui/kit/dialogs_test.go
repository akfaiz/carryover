package kit

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// A dialog is as wide as it asks for, but never wider than the window less a
// margin on each side.
func TestDialogWidthFitsTheWindow(t *testing.T) {
	assert.Equal(t, dialogWideWidth, (&Dialog{Wide: true}).width(2000))
	assert.Equal(t, float32(886-2*dialogMargin), (&Dialog{Wide: true}).width(886))
	assert.Equal(t, dialogWidth, (&Dialog{}).width(886))
	assert.Equal(t, float32(560), (&Dialog{Width: 560}).width(900))
	assert.Equal(t, float32(0), (&Dialog{}).width(10), "never negative")
}
