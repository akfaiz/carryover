package ui

import (
	"embed"

	"github.com/akfaiz/carryover/internal/core"
)

// steamArt is Carryover's own Steam library art, drawn by
// `go run ./ui/assets/steam/gen`.
//
//go:embed assets/steam/*.png
var steamArt embed.FS

// selfSteamArt returns Carryover's library art by kind (core.SelfArtKinds).
func selfSteamArt() map[string][]byte {
	art := map[string][]byte{"icon": appIcon}
	for _, kind := range core.SelfArtKinds {
		if b, err := steamArt.ReadFile("assets/steam/" + kind + ".png"); err == nil {
			art[kind] = b
		}
	}
	return art
}
