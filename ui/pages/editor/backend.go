package editor

import (
	"github.com/akfaiz/carryover/internal/saves/pathvar"
	"github.com/akfaiz/carryover/services"
	"github.com/akfaiz/carryover/ui/envcore"
)

// serviceBackend is the Backend of the application: the services behind env.Env (envcore.Services).
type serviceBackend struct{ u envcore.Services }

func (b serviceBackend) Tokens(gameID string) []pathvar.Token { return b.u.Lib().Tokens(gameID) }
func (b serviceBackend) HasSteamGridKey() bool                { return b.u.Lib().HasSteamGridKey() }
func (b serviceBackend) Emulators() []services.EmulatorView   { return b.u.Emus().List() }
func (b serviceBackend) Game(id string) (services.GameView, error) {
	return b.u.Lib().Get(id)
}
func (b serviceBackend) SteamLibrary() []services.SteamLibraryGame { return b.u.Lib().SteamLibrary() }
func (b serviceBackend) SearchSteam(term string) ([]services.StoreMatch, error) {
	return b.u.Lib().SearchSteam(term)
}
func (b serviceBackend) SuggestSaves(in services.SuggestInput) (services.SuggestResult, error) {
	return b.u.Lib().SuggestSaves(in)
}
func (b serviceBackend) GuessName(exe string) string { return b.u.Lib().GuessName(exe) }
func (b serviceBackend) HeroicGame(exe string) (string, bool) {
	return b.u.Lib().HeroicGame(exe)
}
func (b serviceBackend) LutrisGame(exe string) (string, bool) {
	return b.u.Lib().LutrisGame(exe)
}
func (b serviceBackend) LutrisLibrary() []services.LutrisLibraryGame {
	return b.u.Lib().LutrisLibrary()
}
func (b serviceBackend) BottlesGame(exe string) (string, bool) {
	return b.u.Lib().BottlesGame(exe)
}

func (b serviceBackend) BottlesLibrary() []services.BottlesLibraryGame {
	return b.u.Lib().BottlesLibrary()
}

func (b serviceBackend) HeroicLibrary() []services.HeroicLibraryGame {
	return b.u.Lib().HeroicLibrary()
}
func (b serviceBackend) GuessRomName(emulatorID, rom string) string {
	return b.u.Emus().GuessRomName(emulatorID, rom)
}
func (b serviceBackend) SuggestEmuSaves(emulatorID string, in services.EmuSuggestInput) (services.EmuSuggestResult, error) {
	return b.u.Emus().SuggestSaves(emulatorID, in)
}
func (b serviceBackend) SaveGame(in services.GameInput) (services.GameView, error) {
	return b.u.Lib().Save(in)
}
func (b serviceBackend) SetCover(id, src string) error {
	_, err := b.u.Lib().SetCover(id, src)
	return err
}
func (b serviceBackend) SetCoverFromURL(id, url string) error {
	_, err := b.u.Lib().SetCoverFromURL(id, url)
	return err
}
func (b serviceBackend) SetArt(id string, kind services.ArtKind, src string) error {
	_, err := b.u.Lib().SetArt(id, string(kind), src)
	return err
}
func (b serviceBackend) SetArtFromURL(id string, kind services.ArtKind, url string) error {
	_, err := b.u.Lib().SetArtFromURL(id, string(kind), url)
	return err
}
func (b serviceBackend) ClearArt(id string, kind services.ArtKind) error {
	_, err := b.u.Lib().ClearArt(id, string(kind))
	return err
}
func (b serviceBackend) SearchGridGames(term string) ([]services.ArtGame, error) {
	return b.u.Lib().SearchGridGames(term)
}
func (b serviceBackend) GridArt(kind services.ArtKind, gridGameID int, steamAppID string) ([]services.ArtGrid, error) {
	return b.u.Lib().GridArt(string(kind), gridGameID, steamAppID)
}
