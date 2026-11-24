package editor

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/akfaiz/carryover/services"
	"github.com/akfaiz/carryover/ui/env"
	"github.com/akfaiz/carryover/ui/internal/uitest"
	"github.com/akfaiz/carryover/ui/shell"
)

// The application's backend passes each call on to the services. Calls that
// would reach the network use a game that doesn't exist, so they fail first.
func TestServiceBackendReachesTheServices(t *testing.T) {
	h := uitest.New(t, shell.Config{
		Page:         func(env.Env, string) env.Page { return nil },
		AddGame:      func(env.Env) {},
		SyncProblems: func(env.Env, int, []services.SyncResult) {},
	})
	var b Backend = serviceBackend{h.Shell}

	assert.NotNil(t, b.Tokens(""))
	assert.False(t, b.HasSteamGridKey())
	assert.Empty(t, b.Emulators())
	_, err := b.Game("missing")
	assert.Error(t, err)
	assert.NotPanics(t, func() {
		_ = b.SteamLibrary()
		_ = b.LutrisLibrary()
		_ = b.HeroicLibrary()
		_ = b.GuessName("/games/Some Game/game.exe")
		_, _ = b.HeroicGame("/games/Some Game/game.exe")
		_, _ = b.LutrisGame("/games/Some Game/game.exe")
		_ = b.GuessRomName("missing", "/roms/zelda.nsp")
	})

	res, err := b.SuggestSaves(services.SuggestInput{Name: "Some Unknown Game"})
	require.NoError(t, err)
	assert.Empty(t, res.Suggestions)
	_, err = b.SuggestEmuSaves("missing", services.EmuSuggestInput{})
	assert.Error(t, err)

	g, err := b.SaveGame(services.GameInput{Name: "Game A"})
	require.NoError(t, err)
	assert.Equal(t, "Game A", g.Name)

	assert.Error(t, b.SetCover(g.ID, "cover.txt"), "only images")
	assert.Error(t, b.SetCoverFromURL("missing", "https://example.invalid/c.png"))
	assert.Error(t, b.SetArt(g.ID, services.ArtKind("nope"), "a.png"), "an unknown kind")
	assert.Error(t, b.SetArtFromURL("missing", services.ArtKind("hero"), "https://example.invalid/a.png"))
	assert.Error(t, b.ClearArt(g.ID, services.ArtKind("nope")))
	_, err = b.SearchGridGames("zelda")
	assert.Error(t, err, "no SteamGridDB key")
	_, err = b.GridArt(services.ArtKind("hero"), 1, "")
	assert.Error(t, err, "no SteamGridDB key")
}

func TestSaveKindAndSuggestionLabels(t *testing.T) {
	assert.Equal(t, "Device save", saveKindLabel(saveKindDevice))
	assert.Equal(t, "Account save", saveKindLabel(saveKindAccount))
	seen := map[string]bool{}
	for _, k := range []string{"native", "proton", "install", "steamcloud", "prefix"} {
		l := kindLabel(k)
		assert.NotEmpty(t, l, k)
		seen[l] = true
	}
	assert.Len(t, seen, 5, "each kind has its own label")
	assert.Empty(t, kindLabel("other"))
}

func TestNewSlotsGetUniqueKeys(t *testing.T) {
	vm := &ViewModel{}
	vm.addSlot()
	vm.addSlot()
	require.Len(t, vm.slots, 2)
	assert.Equal(t, "saves-1", vm.slots[0].key)
	assert.Equal(t, "saves-2", vm.slots[1].key)

	m := &emuForm{}
	m.addSlot()
	m.addSlot()
	require.Len(t, m.slots, 2)
	assert.NotEqual(t, m.slots[0].key, m.slots[1].key)

	assert.Equal(t, "saves", m.uniqueKey("saves", nil))
	m.slots[0].key = "saves"
	assert.Equal(t, "saves-3", m.uniqueKey("saves", nil), "saves-2 is taken too")
	assert.Equal(t, "saves", m.uniqueKey("saves", m.slots[0]), "a slot doesn't clash with itself")
}
