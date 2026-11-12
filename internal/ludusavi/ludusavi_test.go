package ludusavi

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIndexLoadsAndFinds(t *testing.T) {
	require.GreaterOrEqual(t, Count(), 10000, "index should hold the full manifest")
	g, err := Find("367520", "")
	require.NoError(t, err, "by steam id")
	require.NotNil(t, g)
	require.Equal(t, "Hollow Knight", g.Name)
	g, _ = Find("", "HOLLOW KNIGHT™")
	require.NotNil(t, g, "by name")
	require.Equal(t, "Hollow Knight", g.Name)
	g, _ = Find("", "definitely not a real game xyz")
	require.Nil(t, g, "unexpected match")
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
	require.Equal(t, want, win)

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
		assert.Equal(t, kind, paths[p])
	}

	// Without install dir / steam user those templates are skipped, not half-filled.
	for _, s := range Suggest(g, Env{GOOS: "linux"}) {
		assert.NotContains(t, []string{KindInstall, KindProton, KindSteamCloud}, s.Kind, "unexpected %+v", s)
	}
}

func TestStoreUserIDBecomesWildcard(t *testing.T) {
	g := &Game{Files: []File{{Path: "<winAppData>/EldenRing/<storeUserId>"}, {Path: "<winLocalAppData>/Packages/X/wgs", MSStore: true}}}
	s := Suggest(g, Env{GOOS: "windows"})
	require.Len(t, s, 2)
	require.Equal(t, "{appdata}/EldenRing", s[0].Path)
	require.Equal(t, []string{"*", "*/**"}, s[0].Include)
	require.True(t, s[1].MSStore, "ms store flag")
	require.False(t, s[0].MSStore, "ms store flag")
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
		assert.Equal(t, want.dir, d, in)
		assert.Equal(t, want.inc, inc, in)
	}
}

// Windows save paths map into a Wine prefix's user folder, named after the
// account; Linux-only and install-folder paths are skipped.
func TestSuggestPrefix(t *testing.T) {
	g := &Game{Files: []File{
		{Path: "<winAppData>/Game/<osUserName>", OS: OSWindows},
		{Path: "<winDocuments>/My Games/Game"},
		{Path: "<xdgData>/Game", OS: OSLinux},
		{Path: "<base>/saves"},
	}}
	got := SuggestPrefix(g, "{home}/Games/Heroic/Prefixes/default/Game/drive_c/users/deck", "deck")
	want := []Suggestion{
		{Path: "{home}/Games/Heroic/Prefixes/default/Game/drive_c/users/deck/AppData/Roaming/Game/deck", Kind: KindPrefix,
			Sources: []string{"<winAppData>/Game/<osUserName>"}},
		{Path: "{home}/Games/Heroic/Prefixes/default/Game/drive_c/users/deck/Documents/My Games/Game", Kind: KindPrefix,
			Sources: []string{"<winDocuments>/My Games/Game"}},
	}
	require.Equal(t, want, got)
}

func TestSameTitle(t *testing.T) {
	assert.True(t, SameTitle("The Witcher® 3: Wild Hunt", "the witcher 3 wild hunt"))
	assert.False(t, SameTitle("Hades", "Hades II"))
	assert.False(t, SameTitle("", ""))
}
