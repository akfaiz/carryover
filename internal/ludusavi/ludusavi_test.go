package ludusavi

import (
	"reflect"
	"testing"
)

func TestIndexLoadsAndFinds(t *testing.T) {
	if n := Count(); n < 10000 {
		t.Fatalf("index has %d games, expected the full manifest", n)
	}
	g, err := Find("367520", "")
	if err != nil || g == nil || g.Name != "Hollow Knight" {
		t.Fatalf("by steam id: %+v %v", g, err)
	}
	g, _ = Find("", "HOLLOW KNIGHT™")
	if g == nil || g.Name != "Hollow Knight" {
		t.Fatalf("by name: %+v", g)
	}
	if g, _ := Find("", "definitely not a real game xyz"); g != nil {
		t.Fatalf("unexpected match %+v", g)
	}
}

func TestSuggestPerDevice(t *testing.T) {
	g := &Game{Name: "Test", Files: []File{
		{Path: "<home>/AppData/LocalLow/Studio/Test/*.dat", OS: OSWindows},
		{Path: "<home>/AppData/LocalLow/Studio/Test/*.bak", OS: OSWindows},
		{Path: "<winDocuments>/My Games/Test/Saves"},
		{Path: "<xdgConfig>/unity3d/Studio/Test", OS: OSLinux},
		{Path: "<base>/save/slot1.sav"},
		{Path: "<root>/userdata/<storeUserId>/123/remote"},
	}}

	win := Suggest(g, Env{GOOS: "windows"})
	want := []Suggestion{
		{Path: "{home}/AppData/LocalLow/Studio/Test", Include: []string{"*.dat", "*.dat/**", "*.bak", "*.bak/**"}, Kind: KindWindows,
			Sources: []string{"<home>/AppData/LocalLow/Studio/Test/*.dat", "<home>/AppData/LocalLow/Studio/Test/*.bak"}},
		{Path: "{documents}/My Games/Test/Saves", Kind: KindWindows, Sources: []string{"<winDocuments>/My Games/Test/Saves"}},
	}
	if !reflect.DeepEqual(win, want) {
		t.Fatalf("windows:\n got %+v\nwant %+v", win, want)
	}

	deck := Suggest(g, Env{GOOS: "linux", Proton: "{proton:shortcut}", InstallDir: "/games/Test", SteamRoot: "{steam}"})
	paths := map[string]string{}
	for _, s := range deck {
		paths[s.Path] = s.Kind
	}
	for p, kind := range map[string]string{
		"{proton:shortcut}/AppData/LocalLow/Studio/Test":  KindProton,
		"{proton:shortcut}/Documents/My Games/Test/Saves": KindProton,
		"{xdgconfig}/unity3d/Studio/Test":                 KindNative,
		"/games/Test/save":                                KindInstall,
		"{steam}/userdata":                                KindSteamCloud,
	} {
		if paths[p] != kind {
			t.Errorf("deck: missing %s (%s) in %v", p, kind, paths)
		}
	}

	// Without install dir / steam user those templates are skipped, not half-filled.
	for _, s := range Suggest(g, Env{GOOS: "linux"}) {
		if s.Kind == KindInstall || s.Kind == KindProton || s.Kind == KindSteamCloud {
			t.Errorf("unexpected %+v", s)
		}
	}
}

func TestStoreUserIDBecomesWildcard(t *testing.T) {
	g := &Game{Files: []File{{Path: "<winAppData>/EldenRing/<storeUserId>"}, {Path: "<winLocalAppData>/Packages/X/wgs", MSStore: true}}}
	s := Suggest(g, Env{GOOS: "windows"})
	if len(s) != 2 || s[0].Path != "{appdata}/EldenRing" || !reflect.DeepEqual(s[0].Include, []string{"*", "*/**"}) {
		t.Fatalf("got %+v", s)
	}
	if !s[1].MSStore || s[0].MSStore {
		t.Fatalf("ms store flag wrong: %+v", s)
	}
}

func TestSplitGlob(t *testing.T) {
	type res struct {
		dir string
		inc []string
	}
	cases := map[string]res{
		"{appdata}/Game/Saves":                 {"{appdata}/Game/Saves", nil},
		"{appdata}/Game/*.sav":                 {"{appdata}/Game", []string{"*.sav", "*.sav/**"}},
		"{appdata}/Game/*/Saves":               {"{appdata}/Game", []string{"*/Saves", "*/Saves/**"}},
		"{appdata}/Game/savegame*":             {"{appdata}/Game", []string{"savegame*", "savegame*/**"}},
		"{documents}/Game/save.dat":            {"{documents}/Game", []string{"save.dat"}},
		"{home}/Library/unity.Studio.Game Two": {"{home}/Library/unity.Studio.Game Two", nil},
	}
	for in, want := range cases {
		d, inc := splitGlob(in)
		if d != want.dir || !reflect.DeepEqual(inc, want.inc) {
			t.Errorf("%s: got (%q,%q) want %+v", in, d, inc, want)
		}
	}
}
