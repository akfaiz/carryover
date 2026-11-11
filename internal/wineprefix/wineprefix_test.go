package wineprefix

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// mkPrefix creates a Wine prefix with drive_c/users/<user> and Public.
func mkPrefix(t *testing.T, dir, user string) {
	t.Helper()
	for _, u := range []string{user, "Public"} {
		if err := os.MkdirAll(filepath.Join(dir, "drive_c", "users", u), 0o755); err != nil {
			t.Fatal(err)
		}
	}
}

func writeFile(t *testing.T, p, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// jsonPath writes a path into a JSON string.
func jsonPath(p string) string { return strings.ReplaceAll(p, `\`, `\\`) }

func TestFind(t *testing.T) {
	home := t.TempDir()
	sd := t.TempDir()
	j := func(parts ...string) string { return filepath.Join(append([]string{home}, parts...)...) }

	// Heroic (Flatpak): a per-game Proton prefix named by its config, titled
	// from the GOG library cache.
	heroic := j(".var", "app", heroicFlatpak, "config", "heroic")
	hades := j("Games", "Heroic", "Prefixes", "default", "hades-prefix")
	mkPrefix(t, filepath.Join(hades, "pfx"), "steamuser")
	writeFile(t, filepath.Join(heroic, "GamesConfig", "1234.json"),
		`{"1234": {"winePrefix": "`+jsonPath(hades)+`", "wineVersion": {"type": "proton"}}, "version": "v0"}`)
	writeFile(t, filepath.Join(heroic, "store_cache", "gog_library.json"),
		`{"games": [{"app_name": "1234", "title": "Hades"}]}`)
	// Heroic's default folder: a prefix named after the game.
	mkPrefix(t, j("Games", "Heroic", "Prefixes", "default", "Celeste"), "deck")
	// Heroic on the SD card.
	mkPrefix(t, filepath.Join(sd, "Heroic", "Prefixes", "default", "Tunic"), "deck")

	// Lutris: the prefix from the game's yml, titled by its name.
	lutrisPrefix := j("Games", "the-witcher-3")
	mkPrefix(t, lutrisPrefix, "deck")
	writeFile(t, j(".local", "share", "lutris", "games", "the-witcher-3-1712345678.yml"),
		"game:\n  exe: "+filepath.Join(lutrisPrefix, "drive_c", "witcher3.exe")+"\n  prefix: "+lutrisPrefix+"\nname: The Witcher 3\ngame_slug: the-witcher-3\n")

	// Bottles: one bottle running two games.
	bottle := j(".local", "share", "bottles", "bottles", "Gaming")
	mkPrefix(t, bottle, "deck")
	writeFile(t, filepath.Join(bottle, "bottle.yml"),
		"Name: Gaming\nExternal_Programs:\n  a1:\n    name: Stardew Valley\n  b2:\n    name: Terraria\n")

	// Plain Wine, and folders that aren't prefixes.
	mkPrefix(t, j(".wine"), "deck")
	if err := os.MkdirAll(j("Games", "NotAPrefix"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(j("Games", "Empty", "drive_c"), 0o755); err != nil { // no user folder
		t.Fatal(err)
	}

	got := map[string]Prefix{}
	for _, p := range Find(Roots{Home: home, Drives: []string{sd}}) {
		got[p.Launcher+"|"+p.Name] = p
	}
	var keys []string
	for k := range got {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	want := []string{"bottles|Gaming", "heroic|Celeste", "heroic|Hades", "heroic|Tunic", "lutris|The Witcher 3", "wine|Wine"}
	if strings.Join(keys, ",") != strings.Join(want, ",") {
		t.Fatalf("prefixes = %v, want %v", keys, want)
	}

	if p := got["heroic|Hades"]; p.Path != filepath.Join(hades, "pfx") ||
		len(p.Users) != 1 || filepath.Base(p.Users[0]) != "steamuser" {
		t.Errorf("Hades prefix = %+v, want the pfx folder and its steamuser", p)
	}
	if p := got["bottles|Gaming"]; len(p.Games) != 2 {
		t.Errorf("bottle games = %v", p.Games)
	}
	for _, p := range got {
		for _, u := range p.Users {
			if filepath.Base(u) == "Public" {
				t.Errorf("%s lists the Public user folder", p.Name)
			}
		}
	}
}

func TestFindWithoutHome(t *testing.T) {
	if got := Find(Roots{}); got != nil {
		t.Fatalf("got %+v", got)
	}
}
