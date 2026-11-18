package ui

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/akfaiz/carryover/internal/core"
)

// Every kind of library art Carryover's own shortcut gets is embedded.
func TestSelfSteamArt(t *testing.T) {
	art := selfSteamArt()
	for _, kind := range core.SelfArtKinds {
		assert.NotEmpty(t, art[kind], kind)
	}
}
