package ui

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/akfaiz/carryover/internal/library"
	"github.com/akfaiz/carryover/services"
)

func TestPlayKindOf(t *testing.T) {
	native := services.GameView{Slots: []library.SaveSlot{{Key: "saves"}}}
	var emulated services.GameView // nothing of its own
	assert.Equal(t, kindDirect, playKindOf(&native, false))
	assert.Equal(t, kindBoth, playKindOf(&native, true))
	assert.Equal(t, kindEmulator, playKindOf(&emulated, true))
	assert.Equal(t, kindDirect, playKindOf(&emulated, false))
}
