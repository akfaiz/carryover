package editor

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/akfaiz/carryover/internal/saves/pathvar"
	"github.com/akfaiz/carryover/services"
)

// fakeBackend records the calls the ViewModel makes and fails the ones named.
type fakeBackend struct {
	calls []string
	fail  map[string]error // by call name
	game  services.GameView
}

func (f *fakeBackend) do(name string) error {
	f.calls = append(f.calls, name)
	return f.fail[name]
}

func (f *fakeBackend) Tokens(string) []pathvar.Token { f.calls = append(f.calls, "tokens"); return nil }
func (f *fakeBackend) HasSteamGridKey() bool         { return true }
func (f *fakeBackend) Emulators() []services.EmulatorView {
	return []services.EmulatorView{{ID: "e1", Name: "Eden"}}
}
func (f *fakeBackend) Game(id string) (services.GameView, error) {
	return f.game, f.do("game " + id)
}
func (f *fakeBackend) SteamLibrary() []services.SteamLibraryGame { return nil }
func (f *fakeBackend) SearchSteam(string) ([]services.StoreMatch, error) {
	return nil, nil
}
func (f *fakeBackend) SuggestSaves(services.SuggestInput) (services.SuggestResult, error) {
	return services.SuggestResult{}, nil
}
func (f *fakeBackend) GuessName(string) string                     { return "" }
func (f *fakeBackend) HeroicGame(string) (string, bool)            { return "", false }
func (f *fakeBackend) LutrisGame(string) (string, bool)            { return "", false }
func (f *fakeBackend) LutrisLibrary() []services.LutrisLibraryGame { return nil }
func (f *fakeBackend) GuessRomName(string, string) string          { return "" }
func (f *fakeBackend) SuggestEmuSaves(string, services.EmuSuggestInput) (services.EmuSuggestResult, error) {
	return services.EmuSuggestResult{}, nil
}
func (f *fakeBackend) SaveGame(in services.GameInput) (services.GameView, error) {
	return services.GameView{ID: "g1", Name: in.Name}, f.do("save")
}
func (f *fakeBackend) SetCover(id, src string) error { return f.do("cover " + id + " " + src) }
func (f *fakeBackend) SetCoverFromURL(id, url string) error {
	return f.do("cover-url " + id + " " + url)
}
func (f *fakeBackend) SetArt(id string, k services.ArtKind, src string) error {
	return f.do("art " + id + " " + string(k) + " " + src)
}
func (f *fakeBackend) SetArtFromURL(id string, k services.ArtKind, url string) error {
	return f.do("art-url " + id + " " + string(k) + " " + url)
}
func (f *fakeBackend) ClearArt(id string, k services.ArtKind) error {
	return f.do("clear " + id + " " + string(k))
}

func TestLoadNewAndExistingGame(t *testing.T) {
	be := &fakeBackend{game: services.GameView{ID: "g1", Name: "Celeste"}}

	l, err := newViewModel("", "", be).Load()
	require.NoError(t, err)
	assert.Nil(t, l.Game, "a new game has nothing to load")
	assert.True(t, l.HasKey)
	assert.Len(t, l.Emulators, 1)

	l, err = newViewModel("g1", "", be).Load()
	require.NoError(t, err)
	require.NotNil(t, l.Game)
	assert.Equal(t, "Celeste", l.Game.Name)

	be.fail = map[string]error{"game g1": errors.New("gone")}
	_, err = newViewModel("g1", "", be).Load()
	assert.EqualError(t, err, "gone")
}

func TestSubmitStoresTheGameThenCoverAndArt(t *testing.T) {
	be := &fakeBackend{}
	vm := newViewModel("", "", be)
	jobs := map[services.ArtKind]artEdit{
		services.ArtHero: {src: "/h.png"},
		services.ArtLogo: {clear: true},
	}

	out, err := vm.Submit(services.GameInput{Name: "G"}, "/c.png", nil, jobs)
	require.NoError(t, err)
	assert.Equal(t, "g1", out.Game.ID)
	assert.NoError(t, out.CoverErr)
	assert.NoError(t, out.ArtErr)
	assert.Equal(t, "save", be.calls[0])
	assert.Equal(t, "cover g1 /c.png", be.calls[1])
	assert.ElementsMatch(t, []string{"art g1 hero /h.png", "clear g1 logo"}, be.calls[2:])
}

func TestSubmitStopsWhenTheGameCannotBeSaved(t *testing.T) {
	be := &fakeBackend{fail: map[string]error{"save": errors.New("disk full")}}
	_, err := newViewModel("", "", be).Submit(services.GameInput{}, "/c.png", nil, nil)
	assert.EqualError(t, err, "disk full")
	assert.Equal(t, []string{"save"}, be.calls, "no cover is tried")
}

func TestSubmitReportsCoverAndArtErrorsWithoutLosingTheGame(t *testing.T) {
	be := &fakeBackend{fail: map[string]error{
		"cover-url g1 http://x/c.png": errors.New("404"),
		"art g1 hero /h.png":          errors.New("bad image"),
	}}
	jobs := map[services.ArtKind]artEdit{
		services.ArtHero: {src: "/h.png"},
		services.ArtLogo: {src: "/l.png"},
	}
	out, err := newViewModel("", "", be).Submit(services.GameInput{Name: "G"}, "", &services.ArtGrid{URL: "http://x/c.png"}, jobs)
	require.NoError(t, err, "the game itself was saved")
	assert.EqualError(t, out.CoverErr, "404")
	assert.EqualError(t, out.ArtErr, "bad image")
	assert.Contains(t, be.calls, "art g1 logo /l.png", "one bad image doesn't stop the others")
}

func TestSubmitCoverFileErrorAborts(t *testing.T) {
	be := &fakeBackend{fail: map[string]error{"cover g1 /c.png": errors.New("unreadable")}}
	_, err := newViewModel("", "", be).Submit(services.GameInput{}, "/c.png", nil, nil)
	assert.EqualError(t, err, "unreadable")
}

func (f *fakeBackend) SearchGridGames(string) ([]services.ArtGame, error) { return nil, nil }
func (f *fakeBackend) GridArt(services.ArtKind, int, string) ([]services.ArtGrid, error) {
	return nil, nil
}
