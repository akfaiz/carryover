package services

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFolderNames(t *testing.T) {
	for path, want := range map[string][2]string{
		`C:\Users\me\Documents\PPSSPP\PSP\SAVEDATA`: {"SAVEDATA", "PSP"},
		"{documents}/PPSSPP/PSP/PPSSPP_STATE/":      {"PPSSPP_STATE", "PSP"},
		"{proton:shortcut}/AppData/Roaming/Game":    {"Game", "Roaming"},
		"/storage/emulated/0/PSP/SAVEDATA":          {"SAVEDATA", "PSP"},
		":saf:docs/primary%3APSP/SAVEDATA":          {"SAVEDATA", "PSP"},
		":saf:docs/primary%3APSP%2FSAVEDATA":        {"SAVEDATA", "PSP"},
		":saf:docs/primary%3ASAVEDATA":              {"SAVEDATA", "primary"},
		"{documents}":                               {"{documents}", ""},
		"":                                          {"", ""},
	} {
		name, parent := folderNames(path)
		assert.Equal(t, want, [2]string{name, parent}, path)
	}
}

// RetroArch keeps a folder per core under both saves and states.
func TestPlaceSuggestionLetsTheParentFolderDecide(t *testing.T) {
	slots := []SlotState{{Elsewhere: []string{"/pc/saves/mGBA"}}, {Elsewhere: []string{"/pc/states/mGBA"}}}
	i, _ := PlaceSuggestion(slots, "/phone/states/mGBA", nil)
	assert.Equal(t, 1, i)
	i, _ = PlaceSuggestion(slots, "/phone/saves/mGBA", nil)
	assert.Equal(t, 0, i)
	// The files still come first.
	slots[0].Include = []string{"Game.srm"}
	i, _ = PlaceSuggestion(slots, "/phone/states/mGBA", []string{"Game.srm"})
	assert.Equal(t, 0, i)
}

// A PC set up two PSP locations; the other device fills in its own folders.
func TestPlaceSuggestionMatchesTheLocationOtherDevicesUse(t *testing.T) {
	states := SlotState{Include: []string{"ULUS10067_*"}, Elsewhere: []string{"{documents}/PSP/PPSSPP_STATE"}}
	data := SlotState{Include: []string{"ULUS10067*"}, Elsewhere: []string{`C:\PSP\SAVEDATA`}}
	both := []SlotState{states, data}
	place := func(slots []SlotState, path string, include ...string) (int, bool) {
		return PlaceSuggestion(slots, path, include)
	}

	i, _ := place(both, ":saf:x/primary%3APSP/SAVEDATA", "ULUS10067*", "ULES00264*")
	assert.Equal(t, 1, i, "the in-game saves, though the states come first")
	i, _ = place(both, "/phone/PSP/ppsspp_state", "ULUS10067_*")
	assert.Equal(t, 0, i, "whatever the case of the name")

	// The PC never added the save states: they don't take the in-game saves' location, even with filters
	// that name the same files, and can't be added while that one is empty.
	i, mayAdd := place([]SlotState{data}, "/phone/PSP/PPSSPP_STATE", "ULUS10067*")
	assert.Equal(t, -1, i)
	assert.False(t, mayAdd)

	filled := data
	filled.Path = "/phone/PSP/SAVEDATA"
	i, mayAdd = place([]SlotState{filled}, "/phone/PSP/PPSSPP_STATE", "ULUS10067_*")
	assert.Equal(t, -1, i)
	assert.True(t, mayAdd, "every location used elsewhere has a folder here")

	// The same folder name with other files is not the same location.
	i, _ = place([]SlotState{data}, "/phone/SAVEDATA", "ULJM05141*")
	assert.Equal(t, -1, i)
	// A whole folder elsewhere, or a suggestion of one, rules nothing out.
	i, _ = place([]SlotState{{Elsewhere: []string{"/pc/SAVEDATA"}}}, "/phone/SAVEDATA", "ULUS10067*")
	assert.Equal(t, 0, i)

	// Two locations in one folder: the one whose files they are.
	two := []SlotState{{Elsewhere: []string{"/pc/saves"}}, {Include: []string{"b.srm"}, Elsewhere: []string{"/pc/saves"}}}
	i, _ = place(two, "/phone/saves", "b.srm")
	assert.Equal(t, 1, i)
}

func TestPlaceSuggestionInALocationNobodyUses(t *testing.T) {
	i, mayAdd := PlaceSuggestion([]SlotState{{Path: "/mine"}, {}}, "/x", []string{"*.sav"})
	assert.Equal(t, 1, i, "the first empty one")
	assert.True(t, mayAdd)

	i, mayAdd = PlaceSuggestion([]SlotState{{Include: []string{"*.gci"}}}, "/x", []string{"*.sav"})
	assert.Equal(t, -1, i, "its filters are for other files")
	assert.True(t, mayAdd)

	i, _ = PlaceSuggestion([]SlotState{{}, {Elsewhere: []string{"/pc/x"}}}, "/phone/x", nil)
	assert.Equal(t, 1, i, "a location used elsewhere comes first")

	assert.True(t, SlotState{Elsewhere: []string{" "}}.TakesFilters())
	assert.False(t, SlotState{Include: []string{"a"}}.TakesFilters())
	assert.False(t, SlotState{Elsewhere: []string{"/pc"}}.TakesFilters())
}
