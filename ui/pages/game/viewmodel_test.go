package game

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/akfaiz/carryover/internal/library"
	"github.com/akfaiz/carryover/services"
)

var ownSlots = []library.SaveSlot{{Key: "saves"}}

func plainGame() services.GameView {
	return services.GameView{ID: "g1", Name: "Celeste", Ready: true, Slots: ownSlots}
}

// emulated is a game with the given emulator profiles; own says whether it also
// has saves of its own.
func emulated(own bool, emus ...services.ProfileView) services.GameView {
	g := services.GameView{ID: "g1", Name: "Zelda", Ready: true, Emulators: emus}
	if own {
		g.Slots = ownSlots
	}
	return g
}

func emu(id, name string, ready bool) services.ProfileView {
	return services.ProfileView{EmulatorID: id, Name: name, Ready: ready, Slots: []library.SaveSlot{{Key: "account"}}}
}

func TestProfilesOfAPlainGameIsJustItsOwnSaves(t *testing.T) {
	vm := newViewModel("g1")
	profs := vm.profiles(plainGame())

	require.Len(t, profs, 1)
	assert.Equal(t, "", profs[0].id)
	assert.False(t, profs[0].noEmu, "nothing to tell it apart from")
	assert.True(t, profs[0].ready)
}

func TestProfilesOfAGameWithEmulators(t *testing.T) {
	vm := newViewModel("g1")
	profs := vm.profiles(emulated(true, emu("eden", "Eden", true), emu("ryu", "Ryujinx", false)))

	require.Len(t, profs, 3)
	assert.Equal(t, "", profs[0].id)
	assert.True(t, profs[0].noEmu, "its own saves are named against the emulators")
	assert.Equal(t, "eden", profs[1].id)
	assert.Equal(t, "Eden", profs[1].tag)
	assert.True(t, profs[1].ready)
	assert.False(t, profs[2].ready, "ryujinx isn't set up on this device")
}

func TestAGameOnlyPlayedThroughEmulatorsHasNoOwnProfile(t *testing.T) {
	profs := newViewModel("g1").profiles(emulated(false, emu("eden", "Eden", true)))
	require.Len(t, profs, 1)
	assert.Equal(t, "eden", profs[0].id)
}

func TestProfileViewShowsThatEmulatorsSavesAndStatus(t *testing.T) {
	g := emulated(true, services.ProfileView{
		EmulatorID: "eden", Ready: false, SetupHint: "set the path", SafetyDir: "/safety/eden",
		SteamAppID: "123", Slots: []library.SaveSlot{{Key: "account"}},
		ResolvedPaths: map[string]string{"account": "/p"},
		Status:        services.GameStatus{Conflict: true},
	})
	g.Device.LaunchMode = "steamwrap"

	v := profileView(g, "eden")
	assert.Equal(t, []library.SaveSlot{{Key: "account"}}, v.Slots)
	assert.Equal(t, "/p", v.ResolvedPaths["account"])
	assert.False(t, v.Ready)
	assert.Equal(t, "set the path", v.SetupHint)
	assert.True(t, v.Status.Conflict)
	assert.Equal(t, "/safety/eden", v.SafetyDir)
	assert.Equal(t, library.LaunchDirect, v.Device.LaunchMode, "an emulator is started by Carryover")
	assert.Equal(t, "123", v.Device.SteamAppID)

	assert.Equal(t, g, profileView(g, ""), "the game's own saves are the game as it is")
}

func TestSelectedProfile(t *testing.T) {
	vm := newViewModel("g1")
	fine := emu("eden", "Eden", true)
	broken := emu("ryu", "Ryujinx", true)
	broken.Status = services.GameStatus{Conflict: true}
	profs := vm.profiles(emulated(true, fine, broken))

	assert.Equal(t, "ryu", vm.selectedProfile(profs).id, "the first that needs attention")

	vm.selected, vm.chosen = "eden", true
	assert.Equal(t, "eden", vm.selectedProfile(profs).id, "what the user picked wins")

	vm.selected = "gone"
	assert.Equal(t, "ryu", vm.selectedProfile(profs).id, "a pick that no longer exists is forgotten")

	calm := vm.profiles(emulated(true, fine))
	vm.chosen = false
	assert.Equal(t, "", vm.selectedProfile(calm).id, "nothing needs attention: the first")
}

func TestActiveProfileAndKey(t *testing.T) {
	vm := newViewModel("g1")
	assert.Equal(t, "g1", vm.key(), "the game's own saves")

	vm.setActive("eden")
	assert.Equal(t, "eden", vm.activeProfile())
	assert.Equal(t, "g1.eden", vm.key(), "an emulator's saves have their own sync key")
}

func TestBusyForNamesTheActionAndTheProfile(t *testing.T) {
	vm := newViewModel("g1")
	assert.False(t, vm.busyFor("sync", ""))

	vm.busy = "sync"
	vm.setActive("eden")
	assert.True(t, vm.busyFor("sync", "eden"))
	assert.False(t, vm.busyFor("sync", ""), "another profile isn't busy")
	assert.False(t, vm.busyFor("backup", "eden"), "another action isn't running")
}

func TestStateIsPerProfileAndKept(t *testing.T) {
	vm := newViewModel("g1")
	a := vm.state("eden")
	a.backupsErr = "offline"
	assert.Same(t, a, vm.state("eden"))
	assert.NotSame(t, a, vm.state(""))
	assert.Empty(t, vm.state("").backupsErr)
}

func TestCanSyncAndSyncTargets(t *testing.T) {
	vm := newViewModel("g1")
	profs := vm.profiles(emulated(true, emu("eden", "Eden", true), emu("ryu", "Ryujinx", false)))

	assert.True(t, canSync(profs))
	assert.Equal(t, []string{"", "eden"}, syncTargets(profs), "the game's own saves and the emulators that are set up")

	none := vm.profiles(services.GameView{Slots: ownSlots, Ready: false})
	assert.False(t, canSync(none))
	assert.Empty(t, syncTargets(none))
}

func TestClassifySync(t *testing.T) {
	for name, tc := range map[string]struct {
		actions  []string
		want     syncKind
		conflict int
	}{
		"nothing to do":       {[]string{"none", "none"}, syncUpToDate, -1},
		"no profiles":         {nil, syncUpToDate, -1},
		"uploaded":            {[]string{"push", "none"}, syncPushed, -1},
		"downloaded":          {[]string{"none", "pull"}, syncPulled, -1},
		"any other is a pull": {[]string{"restore"}, syncPulled, -1},
		"both":                {[]string{"push", "pull"}, syncBoth, -1},
		"a conflict wins":     {[]string{"push", "conflict", "pull"}, syncConflict, 1},
		"the first conflict":  {[]string{"conflict", "conflict"}, syncConflict, 0},
	} {
		kind, conflict := classifySync(tc.actions)
		assert.Equal(t, tc.want, kind, name)
		assert.Equal(t, tc.conflict, conflict, name)
	}
}

func TestInitFromTheGamesLaunchMode(t *testing.T) {
	vm := newViewModel("g1")
	assert.Equal(t, "direct", vm.steamMode)

	g := plainGame()
	g.Device.LaunchMode, g.StoreAppID = "steamwrap", "504230"
	vm.initFrom(g)
	assert.Equal(t, "steamwrap", vm.steamMode)
	assert.True(t, vm.wrapOpen, "a Steam game with an app id opens its launch options")

	vm = newViewModel("g1")
	g.StoreAppID = ""
	vm.initFrom(g)
	assert.False(t, vm.wrapOpen, "without an app id there is nothing to show")

	vm = newViewModel("g1")
	g.Device.LaunchMode = "heroic"
	vm.initFrom(g)
	assert.Equal(t, "heroic", vm.steamMode)

	vm = newViewModel("g1")
	vm.initFrom(plainGame())
	assert.Equal(t, "direct", vm.steamMode)
}

func TestSteamChoiceForAPlainGameIsTheGame(t *testing.T) {
	vm := newViewModel("g1")
	c := vm.steamChoice(plainGame())
	assert.Equal(t, "g1", c.key)
	assert.Equal(t, "game", c.target)
	assert.False(t, c.asks)
	assert.False(t, c.hideMode, "a plain game chooses how Steam launches it")
}

func TestSteamChoiceForAGameWithEmulators(t *testing.T) {
	vm := newViewModel("g1")
	g := emulated(true, emu("eden", "Eden", true))

	c := vm.steamChoice(g)
	assert.Equal(t, "game", c.target, "one shortcut for the whole game by default")
	assert.True(t, c.asks, "it asks which emulator when it starts")
	assert.Equal(t, "g1", c.key)
	assert.False(t, c.hideMode, "the game has saves of its own: the mode is a choice")

	vm.steamTarget = "eden"
	c = vm.steamChoice(g)
	assert.Equal(t, "eden", c.target)
	assert.False(t, c.asks)
	assert.Equal(t, "g1.eden", c.key, "the emulator's own shortcut")
	assert.Equal(t, library.LaunchDirect, c.form.Device.LaunchMode)
	assert.True(t, c.hideMode, "an emulator is always started by Carryover")

	vm.steamTarget = "removed"
	c = vm.steamChoice(g)
	assert.Equal(t, "game", c.target, "an emulator the game no longer has falls back to the game")
	assert.Equal(t, "game", vm.steamTarget, "and stays there")
}

func TestSteamChoiceHidesTheModeForAGameOnlyPlayedThroughEmulators(t *testing.T) {
	vm := newViewModel("g1")
	c := vm.steamChoice(emulated(false, emu("eden", "Eden", true)))
	assert.True(t, c.asks)
	assert.True(t, c.hideMode, "no own launch: the shortcut can only ask for an emulator")
}
