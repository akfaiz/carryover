package game

import (
	"testing"

	"fyne.io/fyne/v2/test"
	"github.com/stretchr/testify/assert"

	"github.com/akfaiz/carryover/services"
)

func TestPlanText(t *testing.T) {
	test.NewApp()
	for _, a := range []string{"none", "push", "pull", "conflict"} {
		assert.NotEmpty(t, planText(a), a)
	}
	assert.Empty(t, planText("other"))
}

func TestOtherDevices(t *testing.T) {
	test.NewApp()
	g := services.GameView{
		Slots:        []services.SaveSlot{{Key: "saves", Label: "Saves"}},
		OtherDevices: []services.DevicePaths{{DeviceName: "deck", Paths: map[string]string{}}},
	}
	assert.Nil(t, otherDevicesCard(g), "no paths to show")
	assert.NotNil(t, otherDevicesLine(g))
	g.OtherDevices[0].Paths["saves"] = "/home/deck/saves"
	assert.NotNil(t, otherDevicesCard(g))
}
