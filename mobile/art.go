package mobile

import (
	"github.com/akfaiz/carryover/internal/app/msg"
)

// TestSteamGridKey checks a SteamGridDB API key without saving it (the app
// saves it in the settings, field steamGridDbKey, once it works).
func TestSteamGridKey(key string) (err error) {
	defer wire(&err)
	s, err := get()
	if err != nil {
		return err
	}
	return s.library.TestSteamGridKey(key)
}

// SearchGridGames finds games on SteamGridDB by name ([]steamgrid.Game) as
// JSON. It needs the API key in the settings.
func SearchGridGames(term string) (_ string, err error) {
	defer wire(&err)
	s, err := get()
	if err != nil {
		return "", err
	}
	return encode(s.library.SearchGridGames(term))
}

// GridArt lists SteamGridDB artwork ([]steamgrid.Grid) as JSON, best scored
// first, for a SteamGridDB game id: kind is "cover" (the portrait capsule) or
// "hero" (the background of the game's page).
func GridArt(kind string, gridGameID int) (_ string, err error) {
	defer wire(&err)
	s, err := get()
	if err != nil {
		return "", err
	}
	if gridGameID <= 0 {
		return "", msg.EditChooseGame.Err()
	}
	switch kind {
	case "cover", "hero":
	default:
		return "", msg.LibraryUnknownArtwork.Err("Kind", kind)
	}
	return encode(s.library.GridArt(kind, gridGameID, ""))
}

// SetCoverFromURL downloads a SteamGridDB image and makes it the game's cover;
// it returns the saved services.GameView as JSON.
func SetCoverFromURL(id, imageURL string) (_ string, err error) {
	defer wire(&err)
	s, err := get()
	if err != nil {
		return "", err
	}
	return encode(s.library.SetCoverFromURL(id, imageURL))
}

// SetBackgroundFromURL downloads a SteamGridDB image and makes it the
// background (hero) of the game's page; it returns the saved services.GameView.
func SetBackgroundFromURL(id, imageURL string) (_ string, err error) {
	defer wire(&err)
	s, err := get()
	if err != nil {
		return "", err
	}
	return encode(s.library.SetArtFromURL(id, "hero", imageURL))
}

// ClearBackground removes the background of the game's page; it returns the
// saved services.GameView.
func ClearBackground(id string) (_ string, err error) {
	defer wire(&err)
	s, err := get()
	if err != nil {
		return "", err
	}
	return encode(s.library.ClearArt(id, "hero"))
}
