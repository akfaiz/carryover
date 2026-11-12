package services

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/akfaiz/carryover/internal/app/appdir"
	"github.com/akfaiz/carryover/internal/core"
	"github.com/akfaiz/carryover/internal/library"
)

// A tiny but valid PNG (1x1), enough for the service to treat it as an image.
var pngBytes = []byte{
	0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a, 0, 0, 0, 0x0d, 'I', 'H', 'D', 'R',
	0, 0, 0, 1, 0, 0, 0, 1, 8, 6, 0, 0, 0, 0x1f, 0x15, 0xc4, 0x89,
}

// libEnv is an isolated data dir without a cloud remote (so nothing syncs in
// the background) and one game.
func newLibEnv(t *testing.T) (*LibraryService, *core.Core, string) {
	t.Helper()
	t.Setenv(appdir.EnvHome, t.TempDir())
	t.Setenv("CARRYOVER_STEAM_ROOT", t.TempDir())
	c, err := core.New()
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(c.Dirs.CoversDir(), 0o755))
	g, err := c.Library.Save(library.Game{Name: "Game A", Slots: []library.SaveSlot{{Key: "saves"}}}, c.DeviceID())
	require.NoError(t, err)
	return NewLibraryService(c, NewBus(), nil), c, g.ID
}

func writeImage(t *testing.T, name string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), name)
	require.NoError(t, os.WriteFile(p, pngBytes, 0o644))
	return p
}

func TestSetCoverCopiesAndReplaces(t *testing.T) {
	svc, c, id := newLibEnv(t)

	v, err := svc.SetCover(id, writeImage(t, "c.png"))
	require.NoError(t, err)
	g, _ := c.Library.Get(id)
	require.Equal(t, id+".png", g.Cover)
	require.Equal(t, id, v.ID)
	require.FileExists(t, filepath.Join(c.Dirs.CoversDir(), g.Cover), "cover file missing")

	// A different format replaces the old file.
	_, err = svc.SetCover(id, writeImage(t, "c.jpg"))
	require.NoError(t, err)
	g, _ = c.Library.Get(id)
	require.Equal(t, id+".jpg", g.Cover)
	require.NoFileExists(t, filepath.Join(c.Dirs.CoversDir(), id+".png"), "the replaced cover file was kept")

	// Re-selecting the stored cover itself keeps it.
	_, err = svc.SetCover(id, filepath.Join(c.Dirs.CoversDir(), id+".jpg"))
	require.NoError(t, err, "SetCover(stored cover)")
}

func TestSetCoverErrors(t *testing.T) {
	svc, _, id := newLibEnv(t)
	_, err := svc.SetCover(id, filepath.Join(t.TempDir(), "x.gif"))
	assert.Error(t, err, "a .gif cover")
	_, err = svc.SetCover(id, filepath.Join(t.TempDir(), "missing.png"))
	assert.Error(t, err, "a missing file")
	_, err = svc.SetCover("no-such-game", writeImage(t, "c.png"))
	assert.Error(t, err, "an unknown game")
}

func TestArtworkSetAndClear(t *testing.T) {
	svc, c, id := newLibEnv(t)

	_, err := svc.SetArt(id, "hero", writeImage(t, "h.png"))
	require.NoError(t, err)
	g, _ := c.Library.Get(id)
	name := g.Art["hero"]
	require.Equal(t, id+"_hero.png", name)
	require.FileExists(t, filepath.Join(c.Dirs.CoversDir(), name), "artwork file missing")

	// Another kind doesn't disturb it.
	_, err = svc.SetArt(id, "logo", writeImage(t, "l.png"))
	require.NoError(t, err)
	g, _ = c.Library.Get(id)
	require.Len(t, g.Art, 2)

	_, err = svc.ClearArt(id, "hero")
	require.NoError(t, err)
	g, _ = c.Library.Get(id)
	require.NotContains(t, g.Art, "hero", "art after clearing hero")
	require.NotEmpty(t, g.Art["logo"])
	require.NoFileExists(t, filepath.Join(c.Dirs.CoversDir(), name), "cleared artwork file was kept")
}

func TestArtworkErrors(t *testing.T) {
	svc, _, id := newLibEnv(t)
	img := writeImage(t, "a.png")
	_, err := svc.SetArt(id, "poster", img)
	assert.ErrorContains(t, err, "unknown artwork kind", "SetArt with an unknown kind")
	_, err = svc.SetArt(id, "hero", filepath.Join(t.TempDir(), "a.gif"))
	assert.Error(t, err, "SetArt with a .gif")
	_, err = svc.SetArt(id, "hero", filepath.Join(t.TempDir(), "missing.png"))
	assert.Error(t, err, "SetArt with a missing file")
	_, err = svc.SetArt("no-such-game", "hero", img)
	assert.Error(t, err, "SetArt for an unknown game")
	_, err = svc.ClearArt(id, "poster")
	assert.Error(t, err, "ClearArt with an unknown kind")
	_, err = svc.ClearArt("no-such-game", "hero")
	assert.Error(t, err, "ClearArt for an unknown game")
	// Only SteamGridDB's own https hosts are ever downloaded.
	_, err = svc.SetArtFromURL(id, "hero", "http://example.com/a.png")
	assert.Error(t, err, "SetArtFromURL with a foreign URL")
	_, err = svc.SetCoverFromURL(id, "https://example.com/a.png")
	assert.Error(t, err, "SetCoverFromURL with a foreign URL")
	_, err = svc.SetArtFromURL(id, "poster", "https://cdn2.steamgriddb.com/a.png")
	assert.Error(t, err, "SetArtFromURL with an unknown kind")
	_, err = svc.SetCoverFromURL("no-such-game", "https://cdn2.steamgriddb.com/a.png")
	assert.Error(t, err, "SetCoverFromURL for an unknown game")
}

func TestSteamGridKeyHandling(t *testing.T) {
	svc, c, _ := newLibEnv(t)
	require.False(t, svc.HasSteamGridKey(), "HasSteamGridKey without a key")
	_, err := svc.SearchGridGames("zelda")
	assert.Error(t, err, "SearchGridGames without a key")
	_, err = svc.GridArt("cover", 1, "")
	assert.Error(t, err, "GridArt without a key")
	assert.Error(t, svc.TestSteamGridKey("   "), "TestSteamGridKey with a blank key")

	st := c.Settings.Get()
	st.SteamGridDBKey = "secret"
	require.NoError(t, c.Settings.Update(st))
	require.True(t, svc.HasSteamGridKey(), "HasSteamGridKey after setting a key")
	// A blank search needs no request.
	got, err := svc.SearchGridGames("  ")
	require.NoError(t, err)
	require.NotNil(t, got)
	require.Empty(t, got, "SearchGridGames(blank)")
}

func TestSteamSearchTooShort(t *testing.T) {
	svc, _, _ := newLibEnv(t)
	got, err := svc.SearchSteam(" a ")
	require.NoError(t, err)
	require.NotNil(t, got)
	require.Empty(t, got, "SearchSteam(short): want an empty list and no request")
}

func TestFetchSteamCoverNeedsLink(t *testing.T) {
	svc, _, id := newLibEnv(t)
	_, err := svc.FetchSteamCover(id)
	require.ErrorContains(t, err, "link the game", "FetchSteamCover on an unlinked game")
	_, err = svc.FetchSteamCover("no-such-game")
	require.Error(t, err, "FetchSteamCover for an unknown game")
}

func TestSteamLibraryMarksLinkedGames(t *testing.T) {
	svc, c, id := newLibEnv(t)
	steamapps := filepath.Join(c.SteamRoot(), "steamapps")
	require.NoError(t, os.MkdirAll(steamapps, 0o755))
	for id, name := range map[string]string{"367520": "Hollow Knight", "105600": "Terraria"} {
		acf := "\"AppState\"\n{\n\t\"appid\"\t\t\"" + id + "\"\n\t\"name\"\t\t\"" + name + "\"\n\t\"installdir\"\t\t\"" + name + "\"\n}\n"
		require.NoError(t, os.WriteFile(filepath.Join(steamapps, "appmanifest_"+id+".acf"), []byte(acf), 0o644))
	}
	g, _ := c.Library.Get(id)
	g.StoreAppID = "105600"
	_, err := c.Library.Save(g, "")
	require.NoError(t, err)

	got := svc.SteamLibrary()
	require.Len(t, got, 2)
	linked := map[string]string{}
	for _, a := range got {
		linked[a.AppID] = a.GameID
	}
	require.Equal(t, id, linked["105600"], "linked games = %v", linked)
	require.Empty(t, linked["367520"], "linked games = %v", linked)
}
