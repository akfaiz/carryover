package mobile

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/akfaiz/carryover/internal/library"
	"github.com/akfaiz/carryover/services"
)

func TestRomFileName(t *testing.T) {
	assert.Equal(t, "/storage/emulated/0/Games/Odyssey [0100000000010000][v0].nsp", romFileName("/storage/emulated/0/Games/Odyssey [0100000000010000][v0].nsp"), "a path is left to the lookup")
	assert.Equal(t, "plain.nsp", romFileName("  plain.nsp "))
	assert.Empty(t, romFileName(""))
	// A document of the file picker: the escaped ID ends in the path of the file.
	assert.Equal(t, "Odyssey [0100000000010000].nsp", romFileName(
		"content://com.android.externalstorage.documents/document/primary%3AGames%2FSwitch%2FOdyssey%20%5B0100000000010000%5D.nsp"))
	assert.Equal(t, "Odyssey [0100000000010000].nsp", romFileName(
		"content://com.android.externalstorage.documents/tree/primary%3AGames/document/primary%3AGames%2FOdyssey%20%5B0100000000010000%5D.nsp"))
	assert.Equal(t, "1234", romFileName("content://media/external/file/1234"), "no name to find: what is there")
}

// profile is a game with one emulator profile with the given slots, the paths this device has for them,
// and the paths another device has.
func profile(slots []library.SaveSlot, here, there map[string]string, share string) (services.GameInput, library.Game) {
	in := services.GameInput{
		ID: "g1", Name: "Game",
		Emulators: []services.GameEmulatorInput{{EmulatorID: "e1", Slots: slots, Paths: here, ShareWith: share}},
	}
	g := library.Game{ID: "g1", Name: "Game", Emulators: []library.GameEmulator{{EmulatorID: "e1", Slots: slots, ShareWith: share}}, Devices: map[string]library.DeviceConfig{
		"pc": {Emulators: map[string]library.EmulatorConfig{"e1": {Paths: there}}},
	}}
	return in, g
}

func TestUseSuggestionFillsTheFirstEmptySlotAndSetsItsFilter(t *testing.T) {
	in, g := profile([]library.SaveSlot{{Key: "saves", Label: "Saves"}}, map[string]string{}, nil, "")
	sg := services.EmuSuggestion{Game: "Luigi's Mansion", Path: "/data/GC/USA", Include: []string{"*.gci"}}
	require.NoError(t, useSuggestion(&in, g, "phone", "e1", sg))
	ge := in.Emulators[0]
	assert.Equal(t, "/data/GC/USA", ge.Paths["saves"])
	assert.Equal(t, []string{"*.gci"}, ge.Slots[0].Include)
	assert.Len(t, ge.Slots, 1, "no slot was added")
	assert.Nil(t, g.Emulators[0].Slots[0].Include, "the stored game is not touched")
}

func TestUseSuggestionAddsASlotWhenAllHaveAFolder(t *testing.T) {
	in, g := profile([]library.SaveSlot{{Key: "saves", Label: "Saves"}}, map[string]string{"saves": "/old"}, nil, "")
	require.NoError(t, useSuggestion(&in, g, "phone", "e1", services.EmuSuggestion{Path: "/more"}))
	ge := in.Emulators[0]
	require.Len(t, ge.Slots, 2)
	assert.Equal(t, "saves-2", ge.Slots[1].Key)
	assert.Equal(t, "Saves 2", ge.Slots[1].Label)
	assert.Equal(t, "/more", ge.Paths["saves-2"])
	assert.Equal(t, "/old", ge.Paths["saves"], "the first folder stays")

	require.NoError(t, useSuggestion(&in, g, "phone", "e1", services.EmuSuggestion{Path: "/third"}))
	assert.Equal(t, "saves-3", in.Emulators[0].Slots[2].Key)
}

// A PC set up the save states, then the in-game saves. A phone with only
// in-game saves must not sync them with the save states.
func TestASuggestionGoesToTheSlotOtherDevicesKeepThoseFilesIn(t *testing.T) {
	slots := []library.SaveSlot{
		{Key: "saves", Label: "Saves", Include: []string{"ULUS10067_*"}},
		{Key: "saves-2", Label: "Saves 2", Include: []string{"ULUS10067*"}},
	}
	pc := map[string]string{"saves": "C:/PSP/PPSSPP_STATE", "saves-2": "C:/PSP/SAVEDATA"}
	data := services.EmuSuggestion{Path: "/phone/PSP/SAVEDATA", Include: []string{"ULUS10067*", "ULES00264*"}, Exists: true}
	states := services.EmuSuggestion{Path: "/phone/PSP/PPSSPP_STATE", Include: []string{"ULUS10067_*"}}

	in, g := profile(slots, map[string]string{}, pc, "")
	assert.Equal(t, 1, autofill(&in, g, "phone", "e1", services.EmuSuggestResult{PerGame: true, Suggestions: []services.EmuSuggestion{data, states}}))
	ge := in.Emulators[0]
	assert.Equal(t, map[string]string{"saves-2": "/phone/PSP/SAVEDATA"}, ge.Paths)
	assert.Equal(t, slots, ge.Slots, "the filters the PC uses stay")

	require.NoError(t, useSuggestion(&in, g, "phone", "e1", states))
	assert.Equal(t, "/phone/PSP/PPSSPP_STATE", in.Emulators[0].Paths["saves"])
	assert.Equal(t, slots, in.Emulators[0].Slots)
}

func TestASuggestionThatMatchesNoSlotIsLeftToTheUser(t *testing.T) {
	slots := []library.SaveSlot{{Key: "saves", Label: "Saves"}}
	in, g := profile(slots, map[string]string{}, map[string]string{"saves": "/pc/MySaves"}, "")
	sg := services.EmuSuggestion{Path: "/phone/SAVEDATA", Include: []string{"ULUS10067*"}, Exists: true}

	assert.Zero(t, autofill(&in, g, "phone", "e1", services.EmuSuggestResult{PerGame: true, Suggestions: []services.EmuSuggestion{sg}}))
	require.ErrorContains(t, useSuggestion(&in, g, "phone", "e1", sg), "No empty save location matches")
	assert.Empty(t, in.Emulators[0].Paths)
	assert.Equal(t, slots, in.Emulators[0].Slots)
}

// A new slot would keep the PC from syncing until it has a folder for it.
func TestAutofillAddsNoSlotToAGameSetUpElsewhere(t *testing.T) {
	in, g := profile([]library.SaveSlot{{Key: "saves"}}, map[string]string{}, map[string]string{"saves": "/pc/SAVEDATA"}, "")
	res := services.EmuSuggestResult{PerGame: true, Suggestions: []services.EmuSuggestion{
		{Path: "/phone/SAVEDATA", Exists: true}, {Path: "/phone/PPSSPP_STATE", Exists: true},
	}}
	assert.Equal(t, 1, autofill(&in, g, "phone", "e1", res))
	assert.Len(t, in.Emulators[0].Slots, 1)

	require.NoError(t, useSuggestion(&in, g, "phone", "e1", res.Suggestions[1]), "asked for, it is added")
	assert.Len(t, in.Emulators[0].Slots, 2)
}

func TestUseSuggestionNeedsAFolderAndAnEmulator(t *testing.T) {
	in, g := profile([]library.SaveSlot{{Key: "saves"}}, map[string]string{}, nil, "")
	require.ErrorIs(t, useSuggestion(&in, g, "phone", "e1", services.EmuSuggestion{Path: "  "}), errNoFolder)
	require.ErrorContains(t, useSuggestion(&in, g, "phone", "nope", services.EmuSuggestion{Path: "/x"}), `no emulator "nope"`)
	assert.Empty(t, in.Emulators[0].Paths)
}

func TestAnEmulatorThatSharesSavesTakesNoNewLocations(t *testing.T) {
	in, g := profile([]library.SaveSlot{{Key: "saves"}}, map[string]string{"saves": "/old"}, nil, "e0")
	require.ErrorIs(t, useSuggestion(&in, g, "phone", "e1", services.EmuSuggestion{Path: "/x"}), errSharedSaves)
	require.ErrorIs(t, useSuggestion(&in, g, "phone", "e1", services.EmuSuggestion{Path: "/x", Kind: saveKindAccount}), errSharedSaves)
	assert.Len(t, in.Emulators[0].Slots, 1)
	// ...but a slot of that kind that it shares gets this device's folder.
	in, g = profile([]library.SaveSlot{{Key: saveKindAccount}}, map[string]string{}, nil, "e0")
	require.NoError(t, useSuggestion(&in, g, "phone", "e1", services.EmuSuggestion{Path: "/acct", Kind: saveKindAccount}))
	assert.Equal(t, "/acct", in.Emulators[0].Paths[saveKindAccount])
}

func TestAKindGoesIntoTheSlotOfThatName(t *testing.T) {
	in, g := profile([]library.SaveSlot{{Key: saveKindAccount, Label: "Account save"}, {Key: saveKindDevice}}, map[string]string{}, map[string]string{saveKindAccount: "/pc"}, "")
	require.NoError(t, useSuggestion(&in, g, "phone", "e1", services.EmuSuggestion{Path: "/phone/acct", Kind: saveKindAccount}))
	assert.Equal(t, "/phone/acct", in.Emulators[0].Paths[saveKindAccount])
	assert.Len(t, in.Emulators[0].Slots, 2)
}

func TestAnUnusedDefaultSlotBecomesTheKind(t *testing.T) {
	// Nobody has a folder for "saves": it can take the kind's key.
	in, g := profile([]library.SaveSlot{{Key: "saves", Label: "Saves"}}, map[string]string{}, nil, "")
	require.NoError(t, useSuggestion(&in, g, "phone", "e1", services.EmuSuggestion{Path: "/acct", Kind: saveKindAccount}))
	ge := in.Emulators[0]
	assert.Equal(t, []library.SaveSlot{{Key: saveKindAccount, Label: "Account save"}}, ge.Slots)
	assert.Equal(t, map[string]string{saveKindAccount: "/acct"}, ge.Paths)
}

func TestADefaultSlotAnotherDeviceUsesIsKept(t *testing.T) {
	// The PC syncs "saves": renaming it would split the cloud folder. The kind is a slot of its own.
	in, g := profile([]library.SaveSlot{{Key: "saves", Label: "Saves"}}, map[string]string{}, map[string]string{"saves": "/pc"}, "")
	require.NoError(t, useSuggestion(&in, g, "phone", "e1", services.EmuSuggestion{Path: "/acct", Kind: saveKindAccount}))
	ge := in.Emulators[0]
	require.Len(t, ge.Slots, 2)
	assert.Equal(t, "saves", ge.Slots[0].Key)
	assert.Equal(t, saveKindAccount, ge.Slots[1].Key)
	assert.Equal(t, "/acct", ge.Paths[saveKindAccount])
	assert.Empty(t, ge.Paths["saves"])
}

func TestHideADeviceSaveThatIsNotThere(t *testing.T) {
	_, g := profile([]library.SaveSlot{{Key: saveKindAccount}}, nil, nil, "")
	dev := services.EmuSuggestion{Kind: saveKindDevice}
	assert.True(t, hideSuggestion(g, "e1", dev), "no slot, no folder")
	dev.Exists = true
	assert.False(t, hideSuggestion(g, "e1", dev), "there is one here")
	assert.False(t, hideSuggestion(g, "e1", services.EmuSuggestion{Kind: saveKindAccount}), "an account save is always offered")
	assert.False(t, hideSuggestion(g, "e1", services.EmuSuggestion{}), "so is a plain folder")
	_, g = profile([]library.SaveSlot{{Key: saveKindDevice}}, nil, nil, "")
	assert.False(t, hideSuggestion(g, "e1", services.EmuSuggestion{Kind: saveKindDevice}), "the game has a device save slot already")
}

func TestAutofillTakesWhatIsOnThisDevice(t *testing.T) {
	in, g := profile([]library.SaveSlot{{Key: "saves", Label: "Saves"}}, map[string]string{}, nil, "")
	res := services.EmuSuggestResult{PerGame: true, Suggestions: []services.EmuSuggestion{
		{Game: "A", Path: "/not/here"},
		{Game: "B", Path: "/here", Exists: true, Include: []string{"*.sav"}},
		{Game: "C", Path: "/here/too", Exists: true},
	}}
	assert.Equal(t, 2, autofill(&in, g, "phone", "e1", res), "what exists, each into a slot")
	ge := in.Emulators[0]
	require.Len(t, ge.Slots, 2)
	assert.Equal(t, "/here", ge.Paths["saves"])
	assert.Equal(t, []string{"*.sav"}, ge.Slots[0].Include)
	assert.Equal(t, "/here/too", ge.Paths["saves-2"])
}

func TestAutofillLeavesAProfileThatHasAFolder(t *testing.T) {
	in, g := profile([]library.SaveSlot{{Key: "saves"}, {Key: "other"}}, map[string]string{"saves": "/mine"}, nil, "")
	res := services.EmuSuggestResult{PerGame: true, Suggestions: []services.EmuSuggestion{{Path: "/here", Exists: true}}}
	assert.Zero(t, autofill(&in, g, "phone", "e1", res))
	assert.Empty(t, in.Emulators[0].Paths["other"])
}

func TestAutofillOnlyFillsWhatBelongsToOneGame(t *testing.T) {
	in, g := profile([]library.SaveSlot{{Key: "saves"}}, map[string]string{}, nil, "")
	res := services.EmuSuggestResult{PerGame: false, Suggestions: []services.EmuSuggestion{{Path: "/everything", Exists: true}}}
	assert.Zero(t, autofill(&in, g, "phone", "e1", res), "a folder with every game's saves is only offered")
}

func TestAutofillOfKinds(t *testing.T) {
	acct := services.EmuSuggestion{Kind: saveKindAccount, Path: "/p1/acct", Exists: true}
	// One candidate that exists: used, and an unused default slot becomes it.
	in, g := profile([]library.SaveSlot{{Key: "saves"}}, map[string]string{}, nil, "")
	assert.Equal(t, 1, autofill(&in, g, "phone", "e1", services.EmuSuggestResult{PerGame: true, Suggestions: []services.EmuSuggestion{acct}}))
	assert.Equal(t, "/p1/acct", in.Emulators[0].Paths[saveKindAccount])

	// Two profiles: the user chooses.
	in, g = profile([]library.SaveSlot{{Key: "saves"}}, map[string]string{}, nil, "")
	two := services.EmuSuggestResult{PerGame: true, Suggestions: []services.EmuSuggestion{acct, {Kind: saveKindAccount, Path: "/p2/acct", Exists: true}}}
	assert.Zero(t, autofill(&in, g, "phone", "e1", two))

	// The game has the slot (from the PC): this device's folder goes in even before the emulator made it.
	in, g = profile([]library.SaveSlot{{Key: saveKindAccount}}, map[string]string{}, map[string]string{saveKindAccount: "/pc"}, "")
	notYet := services.EmuSuggestion{Kind: saveKindAccount, Path: "/p1/acct"}
	assert.Equal(t, 1, autofill(&in, g, "phone", "e1", services.EmuSuggestResult{PerGame: true, Suggestions: []services.EmuSuggestion{notYet}}))
	assert.Equal(t, "/p1/acct", in.Emulators[0].Paths[saveKindAccount])

	// Without the slot, a location that is not there yet is not made up.
	in, g = profile([]library.SaveSlot{{Key: "saves"}}, map[string]string{}, nil, "")
	assert.Zero(t, autofill(&in, g, "phone", "e1", services.EmuSuggestResult{PerGame: true, Suggestions: []services.EmuSuggestion{notYet}}))
}

// A Switch game file with no title ID in its name is looked up by the title ID
// of the save folders another device uses for the game; a name that has one wins.
func TestSwitchTitleIDFromAnotherDevice(t *testing.T) {
	const (
		title = "0100F2C0115B6000"
		user  = "8D2A4E6B11C34F5D9E0A1B2C3D4E5F60"
	)
	g := library.Game{Devices: map[string]library.DeviceConfig{
		"pc": {Emulators: map[string]library.EmulatorConfig{"eden": {Paths: map[string]string{
			"account": `{appdata}\eden\nand\user\save\0000000000000000\` + user + `\` + strings.ToLower(title),
		}}}},
		"phone": {Emulators: map[string]library.EmulatorConfig{"lemon": {Paths: map[string]string{
			"account": ":saf:x/root/nand/user/save/0000000000000000/" + user + "/0100AAAAAAAAA000",
		}}}},
	}}
	assert.Equal(t, title, otherDevicesTitleID(g, "phone"), "this device's own paths don't count")
	assert.Empty(t, otherDevicesTitleID(library.Game{}, "phone"))
	assert.Empty(t, otherDevicesTitleID(library.Game{Devices: map[string]library.DeviceConfig{
		"pc": {Emulators: map[string]library.EmulatorConfig{"x": {Paths: map[string]string{"saves": "/data/GC/USA"}}}},
	}}, "phone"), "not a Switch save folder")

	assert.Equal(t, "Zelda.nsp ["+title+"]", withTitleID("Zelda.nsp", title))
	assert.Equal(t, "["+title+"]", withTitleID("", title))
	assert.Equal(t, "Zelda [0100000000010000].nsp", withTitleID("Zelda [0100000000010000].nsp", title))
	assert.Equal(t, "Zelda.nsp", withTitleID("Zelda.nsp", ""))
}
