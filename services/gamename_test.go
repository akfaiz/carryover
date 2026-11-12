package services

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGuessGameName(t *testing.T) {
	// A tiny stand-in for the Ludusavi manifest.
	manifest := map[string]string{"celeste": "Celeste", "hollowknight": "Hollow Knight", "eldenring": "ELDEN RING"}
	known := func(name string) string {
		key := strings.ToLower(strings.NewReplacer(" ", "", "_", "", "-", "").Replace(name))
		return manifest[key]
	}
	p := filepath.Join
	cases := []struct{ exe, want string }{
		// Exe name is a known game: use the manifest title.
		{p("C:", "Games", "Stuff", "Celeste.exe"), "Celeste"},
		{p("C:", "Games", "Whatever", "hollow_knight.exe"), "Hollow Knight"},
		// Exe name unknown: use the folder, preferring the manifest title.
		{p("C:", "Games", "ELDEN RING", "Game", "start_protected_game.exe"), "ELDEN RING"},
		{p("D:", "Games", "Some Indie Game", "Launcher.exe"), "Some Indie Game"},
		{p("D:", "Games", "My Game", "MyGame", "Binaries", "Win64", "MyGame-Win64-Shipping.exe"), "MyGame"},
		{p("/home", "deck", "Games", "Neat Thing", "bin", "x86_64", "neat.x86_64"), "Neat Thing"},
		// No useful folder: fall back to the exe name, unless it's generic.
		{p("C:", "bin", "coolgame.exe"), "coolgame"},
		{p("C:", "Win64", "Launcher.exe"), ""},
		{"", ""},
	}
	for _, c := range cases {
		assert.Equal(t, c.want, guessGameName(c.exe, known), "guessGameName(%q)", c.exe)
	}
}

// The real Ludusavi index gives the official title for a known exe name.
func TestGuessNameUsesLudusavi(t *testing.T) {
	s := &LibraryService{}
	assert.Equal(t, "Celeste", s.GuessName(filepath.Join("C:", "x", "Unknown Folder Name", "Celeste.exe")))
}
