package services

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/akfaiz/carryover/internal/appdir"
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
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(c.Dirs.CoversDir(), 0o755); err != nil {
		t.Fatal(err)
	}
	g, err := c.Library.Save(library.Game{Name: "Game A", Slots: []library.SaveSlot{{Key: "saves"}}}, c.DeviceID())
	if err != nil {
		t.Fatal(err)
	}
	return NewLibraryService(c, NewBus(), nil), c, g.ID
}

func writeImage(t *testing.T, name string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(p, pngBytes, 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestSetCoverCopiesAndReplaces(t *testing.T) {
	svc, c, id := newLibEnv(t)

	v, err := svc.SetCover(id, writeImage(t, "c.png"))
	if err != nil {
		t.Fatal(err)
	}
	g, _ := c.Library.Get(id)
	if g.Cover != id+".png" || v.ID != id {
		t.Fatalf("cover = %q", g.Cover)
	}
	if _, err := os.Stat(filepath.Join(c.Dirs.CoversDir(), g.Cover)); err != nil {
		t.Fatalf("cover file missing: %v", err)
	}

	// A different format replaces the old file.
	if _, err := svc.SetCover(id, writeImage(t, "c.jpg")); err != nil {
		t.Fatal(err)
	}
	g, _ = c.Library.Get(id)
	if g.Cover != id+".jpg" {
		t.Fatalf("cover = %q, want the .jpg", g.Cover)
	}
	if _, err := os.Stat(filepath.Join(c.Dirs.CoversDir(), id+".png")); !os.IsNotExist(err) {
		t.Fatal("the replaced cover file was kept")
	}

	// Re-selecting the stored cover itself keeps it.
	if _, err := svc.SetCover(id, filepath.Join(c.Dirs.CoversDir(), id+".jpg")); err != nil {
		t.Fatalf("SetCover(stored cover) = %v", err)
	}
}

func TestSetCoverErrors(t *testing.T) {
	svc, _, id := newLibEnv(t)
	if _, err := svc.SetCover(id, filepath.Join(t.TempDir(), "x.gif")); err == nil {
		t.Error("a .gif cover: want an error")
	}
	if _, err := svc.SetCover(id, filepath.Join(t.TempDir(), "missing.png")); err == nil {
		t.Error("a missing file: want an error")
	}
	if _, err := svc.SetCover("no-such-game", writeImage(t, "c.png")); err == nil {
		t.Error("an unknown game: want an error")
	}
}

func TestArtworkSetAndClear(t *testing.T) {
	svc, c, id := newLibEnv(t)

	if _, err := svc.SetArt(id, "hero", writeImage(t, "h.png")); err != nil {
		t.Fatal(err)
	}
	g, _ := c.Library.Get(id)
	name := g.Art["hero"]
	if name != id+"_hero.png" {
		t.Fatalf("art = %v", g.Art)
	}
	if _, err := os.Stat(filepath.Join(c.Dirs.CoversDir(), name)); err != nil {
		t.Fatalf("artwork file missing: %v", err)
	}

	// Another kind doesn't disturb it.
	if _, err := svc.SetArt(id, "logo", writeImage(t, "l.png")); err != nil {
		t.Fatal(err)
	}
	g, _ = c.Library.Get(id)
	if len(g.Art) != 2 {
		t.Fatalf("art = %v, want hero and logo", g.Art)
	}

	if _, err := svc.ClearArt(id, "hero"); err != nil {
		t.Fatal(err)
	}
	g, _ = c.Library.Get(id)
	if _, ok := g.Art["hero"]; ok || g.Art["logo"] == "" {
		t.Fatalf("art after clearing hero = %v", g.Art)
	}
	if _, err := os.Stat(filepath.Join(c.Dirs.CoversDir(), name)); !os.IsNotExist(err) {
		t.Fatal("cleared artwork file was kept")
	}
}

func TestArtworkErrors(t *testing.T) {
	svc, _, id := newLibEnv(t)
	img := writeImage(t, "a.png")
	if _, err := svc.SetArt(id, "poster", img); err == nil || !strings.Contains(err.Error(), "unknown artwork kind") {
		t.Errorf("SetArt with an unknown kind = %v", err)
	}
	if _, err := svc.SetArt(id, "hero", filepath.Join(t.TempDir(), "a.gif")); err == nil {
		t.Error("SetArt with a .gif: want an error")
	}
	if _, err := svc.SetArt(id, "hero", filepath.Join(t.TempDir(), "missing.png")); err == nil {
		t.Error("SetArt with a missing file: want an error")
	}
	if _, err := svc.SetArt("no-such-game", "hero", img); err == nil {
		t.Error("SetArt for an unknown game: want an error")
	}
	if _, err := svc.ClearArt(id, "poster"); err == nil {
		t.Error("ClearArt with an unknown kind: want an error")
	}
	if _, err := svc.ClearArt("no-such-game", "hero"); err == nil {
		t.Error("ClearArt for an unknown game: want an error")
	}
	// Only SteamGridDB's own https hosts are ever downloaded.
	if _, err := svc.SetArtFromURL(id, "hero", "http://example.com/a.png"); err == nil {
		t.Error("SetArtFromURL with a foreign URL: want an error")
	}
	if _, err := svc.SetCoverFromURL(id, "https://example.com/a.png"); err == nil {
		t.Error("SetCoverFromURL with a foreign URL: want an error")
	}
	if _, err := svc.SetArtFromURL(id, "poster", "https://cdn2.steamgriddb.com/a.png"); err == nil {
		t.Error("SetArtFromURL with an unknown kind: want an error")
	}
	if _, err := svc.SetCoverFromURL("no-such-game", "https://cdn2.steamgriddb.com/a.png"); err == nil {
		t.Error("SetCoverFromURL for an unknown game: want an error")
	}
}

func TestSteamGridKeyHandling(t *testing.T) {
	svc, c, _ := newLibEnv(t)
	if svc.HasSteamGridKey() {
		t.Fatal("HasSteamGridKey without a key")
	}
	if _, err := svc.SearchGridGames("zelda"); err == nil {
		t.Error("SearchGridGames without a key: want an error")
	}
	if _, err := svc.GridArt("cover", 1, ""); err == nil {
		t.Error("GridArt without a key: want an error")
	}
	if err := svc.TestSteamGridKey("   "); err == nil {
		t.Error("TestSteamGridKey with a blank key: want an error")
	}

	st := c.Settings.Get()
	st.SteamGridDBKey = "secret"
	if err := c.Settings.Update(st); err != nil {
		t.Fatal(err)
	}
	if !svc.HasSteamGridKey() {
		t.Fatal("HasSteamGridKey after setting a key")
	}
	// A blank search needs no request.
	if got, err := svc.SearchGridGames("  "); err != nil || got == nil || len(got) != 0 {
		t.Fatalf("SearchGridGames(blank) = %v, %v", got, err)
	}
}

func TestSteamSearchTooShort(t *testing.T) {
	svc, _, _ := newLibEnv(t)
	got, err := svc.SearchSteam(" a ")
	if err != nil || got == nil || len(got) != 0 {
		t.Fatalf("SearchSteam(short) = %v, %v; want an empty list and no request", got, err)
	}
}

func TestFetchSteamCoverNeedsLink(t *testing.T) {
	svc, _, id := newLibEnv(t)
	if _, err := svc.FetchSteamCover(id); err == nil || !strings.Contains(err.Error(), "link the game") {
		t.Fatalf("FetchSteamCover on an unlinked game = %v", err)
	}
	if _, err := svc.FetchSteamCover("no-such-game"); err == nil {
		t.Fatal("FetchSteamCover for an unknown game: want an error")
	}
}

func TestSteamLibraryMarksLinkedGames(t *testing.T) {
	svc, c, id := newLibEnv(t)
	steamapps := filepath.Join(c.SteamRoot(), "steamapps")
	if err := os.MkdirAll(steamapps, 0o755); err != nil {
		t.Fatal(err)
	}
	for id, name := range map[string]string{"367520": "Hollow Knight", "105600": "Terraria"} {
		acf := "\"AppState\"\n{\n\t\"appid\"\t\t\"" + id + "\"\n\t\"name\"\t\t\"" + name + "\"\n\t\"installdir\"\t\t\"" + name + "\"\n}\n"
		if err := os.WriteFile(filepath.Join(steamapps, "appmanifest_"+id+".acf"), []byte(acf), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	g, _ := c.Library.Get(id)
	g.StoreAppID = "105600"
	if _, err := c.Library.Save(g, ""); err != nil {
		t.Fatal(err)
	}

	got := svc.SteamLibrary()
	if len(got) != 2 {
		t.Fatalf("SteamLibrary = %+v", got)
	}
	linked := map[string]string{}
	for _, a := range got {
		linked[a.AppID] = a.GameID
	}
	if linked["105600"] != id || linked["367520"] != "" {
		t.Fatalf("linked games = %v", linked)
	}
}
