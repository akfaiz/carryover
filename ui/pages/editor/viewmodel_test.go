package editor

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/akfaiz/carryover/internal/library"
	"github.com/akfaiz/carryover/services"
)

func steamGame(name, appID string) services.SteamLibraryGame {
	var g services.SteamLibraryGame
	g.Name, g.AppID = name, appID
	return g
}

// saveSug is a suggested save location with that many files found in it.
func saveSug(path string, files int, include ...string) services.SaveSuggestion {
	var s services.SaveSuggestion
	s.Path, s.Include, s.Files = path, include, files
	return s
}

func newVM(id string) *ViewModel {
	return &ViewModel{id: id, loading: true, kind: KindDirect,
		dev: services.DeviceInput{Paths: map[string]string{}, Env: map[string]string{}, LaunchMode: services.LaunchDirect}}
}

// newGame is a form for a new game, as loaded.
func newGame() *ViewModel {
	vm := newVM("")
	vm.Apply(Loaded{}, nil)
	return vm
}

func storedGame() *services.GameView {
	return &services.GameView{
		ID: "g1", Name: "Celeste", StoreAppID: "504230", CoverPath: "/c.png", CoverVersion: 3,
		Slots: []services.SaveSlot{{Key: "saves", Label: "", Include: []string{"*.sav"}, Exclude: []string{"*.log"}}},
		Device: library.DeviceConfig{
			Exe: "/games/celeste", Args: "-windowed", WorkDir: "/games", WaitProcess: "celeste.exe",
			Env: map[string]string{"B": "2", "A": "1"}, LaunchMode: "",
			Paths: map[string]string{"saves": "/home/me/saves"},
		},
		Art:          map[string]services.ArtFile{"hero": {Path: "/hero.png", Version: 9}},
		OtherDevices: []services.DevicePaths{{DeviceName: "Deck"}},
	}
}

func TestApplyANewGameStartsWithOneEmptySaveLocation(t *testing.T) {
	vm := newVM("")
	vm.Apply(Loaded{HasKey: true, Emulators: []services.EmulatorView{{ID: "e1"}}}, nil)

	assert.False(t, vm.loading)
	assert.True(t, vm.hasGridKey)
	assert.Len(t, vm.emuList, 1)
	require.Len(t, vm.slots, 1)
	assert.Equal(t, "saves", vm.slots[0].key)
	assert.True(t, vm.slots[0].isNew, "a new location's key follows its label")
	assert.False(t, vm.Dirty(), "nothing typed yet: leaving loses nothing")
}

func TestApplyAStoredGameFillsEveryField(t *testing.T) {
	vm := newVM("g1")
	vm.Apply(Loaded{Game: storedGame()}, nil)

	assert.Equal(t, "Celeste", vm.name)
	assert.Equal(t, "504230", vm.storeAppID)
	assert.Equal(t, "/c.png", vm.coverPath)
	assert.Equal(t, int64(3), vm.coverVersion)
	assert.Equal(t, "/games/celeste", vm.dev.Exe)
	assert.Equal(t, services.LaunchDirect, vm.dev.LaunchMode, "a stored game without a mode launches directly")
	assert.Equal(t, "A=1\nB=2", vm.envText, "sorted")
	assert.True(t, vm.advanced, "working directory, wait process and variables are set: Advanced starts open")
	assert.Len(t, vm.others, 1)

	require.Len(t, vm.slots, 1)
	s := vm.slots[0]
	assert.Equal(t, "saves", s.label, "an unlabelled location shows its key")
	assert.Equal(t, "*.sav", s.include)
	assert.Equal(t, "*.log", s.exclude)
	assert.Equal(t, "/home/me/saves", s.path)
	assert.False(t, s.isNew, "an existing key names its cloud folder and never changes")

	hero := vm.art[services.ArtHero]
	require.NotNil(t, hero)
	assert.Equal(t, "/hero.png", hero.path)
	assert.True(t, hero.has())
	assert.False(t, hero.changed())

	assert.Equal(t, KindDirect, vm.kind)
	assert.False(t, vm.Dirty(), "as loaded, nothing to lose")
}

func TestApplyReportsWhyLoadingFailed(t *testing.T) {
	vm := newVM("g1")
	vm.Apply(Loaded{}, errors.New("game not found"))
	assert.Equal(t, "game not found", vm.errText)
	assert.Equal(t, NoteError, vm.BarNote())
}

func TestDirtyFollowsWhatWouldBeLost(t *testing.T) {
	vm := newVM("g1")
	assert.False(t, vm.Dirty(), "still loading")
	vm.Apply(Loaded{Game: storedGame()}, nil)

	for name, change := range map[string]func(*ViewModel){
		"name":      func(v *ViewModel) { v.name = "Celeste 2" },
		"store id":  func(v *ViewModel) { v.storeAppID = "" },
		"exe":       func(v *ViewModel) { v.dev.Exe = "/other" },
		"env":       func(v *ViewModel) { v.envText += "\nC=3" },
		"path":      func(v *ViewModel) { v.slots[0].path = "/elsewhere" },
		"label":     func(v *ViewModel) { v.slots[0].label = "Autosaves" },
		"new slot":  func(v *ViewModel) { v.addSlot() },
		"kind":      func(v *ViewModel) { v.kind = KindBoth },
		"cover":     func(v *ViewModel) { v.coverSrc = "/new.png" },
		"art":       func(v *ViewModel) { v.artEdit(services.ArtLogo).src = "/logo.png" },
		"clear art": func(v *ViewModel) { v.artEdit(services.ArtHero).clear = true },
	} {
		v := newVM("g1")
		v.Apply(Loaded{Game: storedGame()}, nil)
		change(v)
		assert.True(t, v.Dirty(), name)
	}

	vm.name = "Celeste 2"
	vm.saving = true
	assert.False(t, vm.Dirty(), "saving: the form is on its way out")
	vm.saving, vm.saved = false, true
	assert.False(t, vm.Dirty(), "saved: leaving loses nothing")
}

func TestPrefillFromSteam(t *testing.T) {
	vm := newGame()
	vm.name = "typed before"
	vm.PrefillFromSteam(steamGame("Hades", "1145360"))

	assert.Equal(t, "Hades", vm.name)
	assert.Equal(t, "1145360", vm.storeAppID)
	assert.Equal(t, services.LaunchSteamWrap, vm.dev.LaunchMode, "Steam starts it; Carryover wraps the launch options")
	assert.False(t, vm.Dirty(), "the game picked in Steam is where the form starts")
}

func TestPrefillFromLutris(t *testing.T) {
	vm := newGame()
	vm.name = "typed before"
	var lg services.LutrisLibraryGame // the embedded Game's fields are promoted
	lg.ConfigID, lg.Name, lg.Exe = "hades-1", "Hades", "/home/deck/Games/hades/Hades.exe"
	vm.PrefillFromLutris(lg)

	assert.Equal(t, "Hades", vm.name)
	assert.Equal(t, "/home/deck/Games/hades/Hades.exe", vm.dev.Exe)
	assert.Equal(t, services.LaunchLutris, vm.dev.LaunchMode, "Lutris starts it, in its own Wine prefix")
	assert.False(t, vm.Dirty(), "the game picked in Lutris is where the form starts")
}

func TestPrefillFromBottles(t *testing.T) {
	vm := newGame()
	var bg services.BottlesLibraryGame
	bg.Bottle, bg.Name, bg.Exe = "Games", "Hades", "/home/deck/bottles/Games/drive_c/Hades.exe"
	vm.PrefillFromBottles(bg)

	assert.Equal(t, "Hades", vm.name)
	assert.Equal(t, "/home/deck/bottles/Games/drive_c/Hades.exe", vm.dev.Exe)
	assert.Equal(t, services.LaunchBottles, vm.dev.LaunchMode, "Bottles starts it, in its bottle")
	assert.False(t, vm.Dirty(), "the program picked in Bottles is where the form starts")
}

func TestPrefillFromHeroic(t *testing.T) {
	vm := newGame()
	var hg services.HeroicLibraryGame
	hg.Runner, hg.AppName, hg.Name, hg.Exe = "gog", "1234", "Hades", "/home/deck/Games/Heroic/Hades/Hades.exe"
	vm.PrefillFromHeroic(hg)

	assert.Equal(t, "Hades", vm.name)
	assert.Equal(t, "/home/deck/Games/Heroic/Hades/Hades.exe", vm.dev.Exe)
	assert.Equal(t, services.LaunchHeroic, vm.dev.LaunchMode, "Heroic starts it, with its own settings")
	assert.False(t, vm.Dirty(), "the game picked in Heroic is where the form starts")
}

func TestAddSlotNumbersTheNewLocationPastTheTakenOnes(t *testing.T) {
	vm := newGame() // one slot: "saves"
	vm.addSlot()
	vm.addSlot()
	require.Len(t, vm.slots, 3)
	assert.Equal(t, "saves-2", vm.slots[1].key)
	assert.Equal(t, "Saves 2", vm.slots[1].label)
	assert.Equal(t, "saves-3", vm.slots[2].key)

	vm.slots = vm.slots[:2]
	vm.slots[1].key = "saves-3" // a gap that would collide with the next number
	vm.addSlot()
	assert.Equal(t, "saves-4", vm.slots[2].key, "never a key that exists")
}

func TestUniqueKey(t *testing.T) {
	vm := newGame()
	vm.slots = []*slotForm{{key: "saves"}, {key: "saves-2"}}
	assert.Equal(t, "other", vm.uniqueKey("other", nil))
	assert.Equal(t, "saves-3", vm.uniqueKey("saves", nil))
	assert.Equal(t, "saves", vm.uniqueKey("saves", vm.slots[0]), "a slot doesn't clash with itself")
}

func TestInputForAPCGame(t *testing.T) {
	vm := newVM("g1")
	vm.Apply(Loaded{Game: storedGame()}, nil)
	vm.emus = []*emuForm{{id: "eden"}} // left over from before the user chose "PC or Steam game"

	in, err := vm.Input()
	require.NoError(t, err)
	assert.Equal(t, "g1", in.ID)
	assert.Equal(t, "Celeste", in.Name)
	assert.Equal(t, "504230", in.StoreAppID)
	require.Len(t, in.Slots, 1)
	assert.Equal(t, []string{"*.sav"}, in.Slots[0].Include)
	assert.Equal(t, "/home/me/saves", in.Device.Paths["saves"])
	assert.Equal(t, map[string]string{"A": "1", "B": "2"}, in.Device.Env)
	assert.Equal(t, "/games/celeste", in.Device.Exe)
	assert.Empty(t, in.Emulators, "a PC game drops its emulators when saved")
}

func TestInputForAnEmulatedGameHasNoLaunchOrSavesOfItsOwn(t *testing.T) {
	vm := newGame()
	vm.name = "Zelda"
	vm.kind = KindEmulator
	vm.dev.Exe = "/leftover"
	vm.slots[0].path = "/leftover"
	vm.emus = []*emuForm{{id: "eden", rom: "/roms/zelda.nsp", slots: []*slotForm{{key: "account", path: "/a"}}}}

	in, err := vm.Input()
	require.NoError(t, err)
	assert.Empty(t, in.Slots)
	assert.Empty(t, in.Device.Exe)
	assert.Equal(t, services.LaunchDirect, in.Device.LaunchMode)
	require.Len(t, in.Emulators, 1)
	assert.Equal(t, "eden", in.Emulators[0].EmulatorID)
	assert.Equal(t, "/roms/zelda.nsp", in.Emulators[0].Rom)
	assert.Equal(t, "/a", in.Emulators[0].Paths["account"])
}

func TestInputForBothKeepsTheGamesOwnSetupAndItsEmulators(t *testing.T) {
	vm := newGame()
	vm.name = "Game"
	vm.kind = KindBoth
	vm.slots[0].path = "/own"
	vm.emus = []*emuForm{{id: "eden"}}

	in, err := vm.Input()
	require.NoError(t, err)
	assert.Len(t, in.Slots, 1)
	assert.Equal(t, "/own", in.Device.Paths["saves"])
	assert.Len(t, in.Emulators, 1)
}

func TestInputNeedsAnEmulatorForAnEmulatedGame(t *testing.T) {
	for _, k := range []PlayKind{KindEmulator, KindBoth} {
		vm := newGame()
		vm.name = "Game"
		vm.kind = k
		_, err := vm.Input()
		require.ErrorIs(t, err, ErrNoEmulator, string(k))
	}
	vm := newGame()
	_, err := vm.Input()
	assert.NoError(t, err, "a PC game needs none")
}

func TestBarNote(t *testing.T) {
	vm := newGame()
	assert.Equal(t, NoteName, vm.BarNote(), "nothing typed")
	assert.False(t, vm.CanSave())

	vm.name = "  "
	assert.Equal(t, NoteName, vm.BarNote(), "a blank name is no name")
	assert.False(t, vm.CanSave())

	vm.name = "Game"
	assert.True(t, vm.CanSave())
	assert.Equal(t, NoteSaveLocation, vm.BarNote(), "no path on this device yet; still savable")

	vm.slots[0].path = "/saves"
	assert.Equal(t, NoteNone, vm.BarNote())

	vm.kind = KindEmulator
	assert.Equal(t, NoteEmulator, vm.BarNote())
	vm.emus = []*emuForm{{id: "eden"}}
	assert.Equal(t, NoteNone, vm.BarNote(), "an emulated game needs no path of its own")

	vm.errText = "boom"
	assert.Equal(t, NoteError, vm.BarNote(), "an error comes first")

	vm.errText = ""
	vm.saving = true
	assert.False(t, vm.CanSave(), "not while saving")
}

func TestApplySuggestionsAutofillsOnlyAnUnambiguousFind(t *testing.T) {
	one := services.SuggestResult{Matched: "Celeste", Suggestions: []services.SaveSuggestion{
		saveSug("/found", 3, "*.sav"),
		saveSug("/empty", 0),
	}}
	vm := newGame()
	assert.True(t, vm.ApplySuggestions(one, true))
	assert.Equal(t, "/found", vm.slots[0].path)
	assert.Equal(t, "*.sav", vm.slots[0].include)
	assert.NotNil(t, vm.suggest, "the list is shown either way")

	two := services.SuggestResult{Suggestions: []services.SaveSuggestion{saveSug("/a", 1), saveSug("/b", 2)}}
	vm = newGame()
	assert.False(t, vm.ApplySuggestions(two, true), "two candidates: the user chooses")
	assert.Empty(t, vm.slots[0].path)

	vm = newGame()
	assert.False(t, vm.ApplySuggestions(one, false), "asked for by the user: only listed")
	assert.Empty(t, vm.slots[0].path)

	vm = newGame()
	vm.slots[0].path = "/mine"
	assert.False(t, vm.ApplySuggestions(one, true), "never over a path the user set")
	assert.Equal(t, "/mine", vm.slots[0].path)
}

func TestUseSuggestionGoesToTheTargetThenTheFirstEmptySlotThenANewOne(t *testing.T) {
	sug := saveSug("/s", 0, "*.dat")
	vm := newGame()
	vm.slots = []*slotForm{{key: "a", path: "/taken"}, {key: "b"}}

	vm.useSuggestion(sug, vm.slots[0])
	assert.Equal(t, "/s", vm.slots[0].path, "the target, even if it had a path")

	vm.slots[0].path = "/taken"
	vm.useSuggestion(sug, nil)
	assert.Equal(t, "/s", vm.slots[1].path, "the first empty one")
	assert.Equal(t, "*.dat", vm.slots[1].include)

	vm.useSuggestion(saveSug("/x/y/Autosaves", 0), nil)
	require.Len(t, vm.slots, 3, "none empty: a new location")
	assert.Equal(t, "Autosaves", vm.slots[2].label)
	assert.Equal(t, "autosaves", vm.slots[2].key)
}

// A suggestion fills a location another device uses only if it holds the same
// files there, and never changes the filters that device syncs with.
func TestSuggestionsKeepToTheLocationsOtherDevicesUse(t *testing.T) {
	others := []services.DevicePaths{{DeviceName: "PC", Paths: map[string]string{"saves": "C:/PSP/PPSSPP_STATE", "saves-2": "C:/PSP/SAVEDATA"}}}
	m := &emuForm{id: "ppsspp", others: others, slots: []*slotForm{
		{key: "saves", include: "ULUS10067_*"}, {key: "saves-2", include: "ULUS10067*"},
	}}
	vm := newGame()
	vm.ApplyEmuSuggestions(m, services.EmuSuggestResult{PerGame: true, Suggestions: []services.EmuSuggestion{
		{Path: "/deck/PSP/SAVEDATA", Include: []string{"ULUS10067*", "ULES00264*"}, Exists: true},
		{Path: "/deck/PSP/screenshots", Exists: true},
	}}, true)
	require.Len(t, m.slots, 2, "none is added by itself")
	assert.Empty(t, m.slots[0].path)
	assert.Equal(t, "/deck/PSP/SAVEDATA", m.slots[1].path)
	assert.Equal(t, "ULUS10067*", m.slots[1].include)

	assert.False(t, m.useSuggestion(services.EmuSuggestion{Path: "/deck/PSP/screenshots"}), "the save states' location is still empty")
	require.Len(t, m.slots, 2)
	assert.True(t, m.useSuggestion(services.EmuSuggestion{Path: "/deck/PSP/PPSSPP_STATE", Include: []string{"ULUS10067_*"}}))
	assert.Equal(t, "/deck/PSP/PPSSPP_STATE", m.slots[0].path)
}

func TestAPCGameSuggestionKeepsToTheLocationsOtherDevicesUse(t *testing.T) {
	vm := newGame()
	vm.slots = []*slotForm{{key: "saves", include: "*.sav", exclude: "*.log"}}
	vm.others = []services.DevicePaths{{DeviceName: "PC", Paths: map[string]string{"saves": "{appdata}/Game/Saves"}}}

	other := saveSug("/deck/Game/Config", 3, "*.ini")
	assert.False(t, vm.ApplySuggestions(services.SuggestResult{Suggestions: []services.SaveSuggestion{other}}, true))
	assert.False(t, vm.useSuggestion(other, nil))
	require.Len(t, vm.slots, 1)
	assert.Empty(t, vm.slots[0].path)

	assert.True(t, vm.ApplySuggestions(services.SuggestResult{Suggestions: []services.SaveSuggestion{saveSug("/deck/Game/saves", 3, "*.sav", "*.bak")}}, true))
	assert.Equal(t, "/deck/Game/saves", vm.slots[0].path)
	assert.Equal(t, "*.sav", vm.slots[0].include, "the PC's filters stay")
	assert.Equal(t, "*.log", vm.slots[0].exclude)
}

func TestSharingAnEmulatorsSaves(t *testing.T) {
	eden := &emuForm{id: "eden", slots: []*slotForm{{key: "account", path: "/eden"}}}
	ryu := &emuForm{id: "ryu", slots: []*slotForm{{key: "saves", path: "/ryu-old", isNew: true}}}
	vm := newGame()
	vm.emus = []*emuForm{eden, ryu}

	assert.Equal(t, []*emuForm{ryu}, vm.shareTargets(eden), "any emulator with its own saves")

	vm.setShare(ryu, eden)
	assert.Equal(t, "eden", ryu.share)
	require.Len(t, ryu.slots, 1)
	assert.Equal(t, "account", ryu.slots[0].key, "the locations are the other's")
	assert.Empty(t, ryu.slots[0].path, "only its own paths are its own; none yet")

	assert.Equal(t, []*emuForm{eden}, vm.shareTargets(ryu), "the choice it made stays selectable")
	assert.Empty(t, vm.shareTargets(eden), "eden is shared by ryu: one level only, so it can't share in turn")

	vm.setShare(ryu, nil)
	assert.Empty(t, ryu.share)
	assert.False(t, ryu.slots[0].isNew)
}

func TestEmulatorInputs(t *testing.T) {
	vm := newGame()
	vm.emus = []*emuForm{{id: "eden", rom: "/r", share: "ryu", slots: []*slotForm{
		{key: "account", label: "Account", include: "a, b", exclude: "c", path: "/p"},
	}}}
	in := vm.emulatorInputs()
	require.Len(t, in, 1)
	assert.Equal(t, "eden", in[0].EmulatorID)
	assert.Equal(t, "ryu", in[0].ShareWith)
	assert.Equal(t, "/p", in[0].Paths["account"])
	assert.Equal(t, []string{"a", "b"}, in[0].Slots[0].Include, "split on commas and lines")
}

func TestShareRomOnlyFillsEmptyOnes(t *testing.T) {
	a, b, c := &emuForm{id: "a", rom: "/zelda.nsp"}, &emuForm{id: "b"}, &emuForm{id: "c", rom: "/own.nsp"}
	vm := newGame()
	vm.emus = []*emuForm{a, b, c}
	vm.shareRom(a)
	assert.Equal(t, "/zelda.nsp", b.rom)
	assert.Equal(t, "/own.nsp", c.rom, "a game file the user set stays")
	assert.Equal(t, "/zelda.nsp", vm.firstRom())

	assert.Empty(t, newGame().firstRom())
}

func TestApplyEmuSuggestionsFillsPerGameLocationsThatExist(t *testing.T) {
	m := &emuForm{id: "ryu", slots: []*slotForm{{key: "saves", isNew: true}}}
	vm := newGame()
	res := services.EmuSuggestResult{PerGame: true, Suggestions: []services.EmuSuggestion{
		{Path: "/exists", Exists: true}, {Path: "/missing"},
	}}
	vm.ApplyEmuSuggestions(m, res, true)
	assert.Equal(t, "/exists", m.slots[0].path)
	assert.NotNil(t, m.sug)

	m = &emuForm{id: "ryu", slots: []*slotForm{{key: "saves"}}}
	vm.ApplyEmuSuggestions(m, services.EmuSuggestResult{PerGame: false, Suggestions: res.Suggestions}, true)
	assert.Empty(t, m.slots[0].path, "locations that hold every game's saves are only offered")

	m = &emuForm{id: "ryu", slots: []*slotForm{{key: "saves"}}}
	vm.ApplyEmuSuggestions(m, res, false)
	assert.Empty(t, m.slots[0].path, "not autofilled when the user asked")
	assert.NotNil(t, m.sug, "but listed")
}

func TestArtworkJobsListOnlyChanges(t *testing.T) {
	vm := newVM("g1")
	vm.Apply(Loaded{Game: storedGame()}, nil)
	assert.Empty(t, vm.artworkJobs(), "nothing changed")

	vm.artEdit(services.ArtLogo).src = "/logo.png"
	vm.artEdit(services.ArtHero).clear = true
	jobs := vm.artworkJobs()
	assert.Len(t, jobs, 2)
	assert.Equal(t, "/logo.png", jobs[services.ArtLogo].src)
	hero := jobs[services.ArtHero]
	assert.True(t, hero.clear)
	assert.False(t, hero.has(), "cleared: no image after saving")
}
