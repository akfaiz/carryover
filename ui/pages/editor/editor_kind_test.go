package editor

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/akfaiz/carryover/services"
)

func TestPlayKindOf(t *testing.T) {
	native := services.GameView{Slots: []services.SaveSlot{{Key: "saves"}}}
	var emulated services.GameView // nothing of its own
	assert.Equal(t, KindDirect, playKindOf(&native, false))
	assert.Equal(t, KindBoth, playKindOf(&native, true))
	assert.Equal(t, KindEmulator, playKindOf(&emulated, true))
	assert.Equal(t, KindDirect, playKindOf(&emulated, false))
}
