package services

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/akfaiz/carryover/internal/library"
)

func TestSteamRunURL(t *testing.T) {
	shortcut := GameView{StoreAppID: "620", Device: library.DeviceConfig{SteamAppID: "3000000000"}}
	assert.Equal(t, "steam://rungameid/12884901888033554432", SteamRunURL(shortcut), "a shortcut wins over the store app")
	assert.Equal(t, "steam://rungameid/620", SteamRunURL(GameView{StoreAppID: "620"}))
	assert.Empty(t, SteamRunURL(GameView{}))
	assert.Empty(t, SteamRunURL(GameView{StoreAppID: "not-a-number"}))
}
