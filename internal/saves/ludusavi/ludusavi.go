// Package ludusavi suggests save locations for games using an embedded index
// built from the Ludusavi manifest (https://github.com/mtkennerly/ludusavi-manifest,
// MIT; data compiled from PCGamingWiki). Regenerate the index with
// `go run ./internal/saves/ludusavi/gen`.
package ludusavi

import (
	"bytes"
	"compress/gzip"
	_ "embed"
	"encoding/json"
	"strings"
	"sync"
	"unicode"
)

// OS bits for File.OS. Zero means the path applies on every OS.
const (
	OSWindows uint8 = 1 << iota
	OSLinux
	OSMac
)

// File is one save location template, e.g. "<winAppData>/Game/Saves".
type File struct {
	Path string `json:"p"`
	OS   uint8  `json:"o,omitempty"`
	// MSStore marks paths used only by the Microsoft Store / Game Pass version.
	MSStore bool `json:"m,omitempty"`
}

// Game is a manifest entry with its save locations.
type Game struct {
	Name    string `json:"n"`
	SteamID string `json:"s,omitempty"`
	Files   []File `json:"f"`
}

//go:embed index.json.gz
var indexGz []byte

var (
	loadOnce sync.Once
	games    []Game
	bySteam  map[string]int
	byName   map[string]int
	loadErr  error
)

func load() {
	zr, err := gzip.NewReader(bytes.NewReader(indexGz))
	if err != nil {
		loadErr = err
		return
	}
	defer zr.Close()
	if err := json.NewDecoder(zr).Decode(&games); err != nil {
		loadErr = err
		return
	}
	bySteam = make(map[string]int, len(games))
	byName = make(map[string]int, len(games))
	for i, g := range games {
		if g.SteamID != "" {
			if _, dup := bySteam[g.SteamID]; !dup {
				bySteam[g.SteamID] = i
			}
		}
		byName[normalize(g.Name)] = i
	}
}

// normalize folds case, punctuation and trademark symbols so "DARK SOULS™ III"
// matches "Dark Souls III".
func normalize(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// SameTitle reports whether two game titles match once case, punctuation
// and trademark symbols are ignored.
func SameTitle(a, b string) bool {
	na := normalize(a)
	return na != "" && na == normalize(b)
}

// Find returns the manifest entry for a game, preferring an exact Steam app id
// match and falling back to the normalised name.
func Find(steamID, name string) (*Game, error) {
	loadOnce.Do(load)
	if loadErr != nil {
		return nil, loadErr
	}
	if i, ok := bySteam[steamID]; ok && steamID != "" {
		return &games[i], nil
	}
	if n := normalize(name); n != "" {
		if i, ok := byName[n]; ok {
			return &games[i], nil
		}
	}
	return nil, nil
}

// Count reports how many games the index knows.
func Count() int {
	loadOnce.Do(load)
	return len(games)
}
