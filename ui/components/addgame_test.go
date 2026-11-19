package components_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/akfaiz/carryover/ui/components"
	"github.com/akfaiz/carryover/ui/internal/uitest"
)

func TestShowAddGame(t *testing.T) {
	h := newHarness(t)
	components.ShowAddGame(h.Shell)
	h.Settle()
	assert.Contains(t, uitest.OverlayText(h), "Add a game")
	uitest.TapOverlayButton(t, h, "Cancel")
	h.Settle()

	components.ShowAddGame(h.Shell)
	h.Settle()
	uitest.TapChoice(t, h, "PC game")
	h.Settle()
}

func TestShowSteamImport(t *testing.T) {
	h := newHarness(t)
	components.ShowSteamImport(h.Shell)
	h.Settle()
	uitest.TapOverlayButton(t, h, "Close")
	h.Settle()
}
