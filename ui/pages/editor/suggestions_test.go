package editor

import (
	"testing"

	"fyne.io/fyne/v2/test"
	"github.com/stretchr/testify/assert"

	"github.com/akfaiz/carryover/internal/saves/ludusavi"
	"github.com/akfaiz/carryover/services"
)

func TestKindLabel(t *testing.T) {
	test.NewApp()
	for _, k := range []string{"native", "proton", "install", "steamcloud", "prefix"} {
		assert.NotEmpty(t, kindLabel(k), k)
	}
	assert.Empty(t, kindLabel("other"))
}

func TestSuggestionList(t *testing.T) {
	test.NewApp()
	var used, added, hidden int
	use := func(services.SaveSuggestion) { used++ }
	add := func(services.SaveSuggestion) { added++ }
	hide := func() { hidden++ }

	// nothing known
	assert.NotNil(t, newSuggestionList("G", &services.SuggestResult{}, use, add, hide))
	// matched, but nothing applies here
	assert.NotNil(t, newSuggestionList("G", &services.SuggestResult{Matched: "G"}, use, add, hide))

	mk := func(kind string, files int, exists, ms bool, prefix string) services.SaveSuggestion {
		return services.SaveSuggestion{
			Suggestion: ludusavi.Suggestion{Path: "/p", Kind: kind, Prefix: prefix, MSStore: ms, Include: []string{"a.sav", "x/**"}},
			Files:      files, Bytes: 10, Exists: exists,
		}
	}
	res := &services.SuggestResult{Matched: "G", Suggestions: []services.SaveSuggestion{
		mk("native", 2, true, false, ""),
		mk("proton", 0, true, true, "Heroic: G"),
		mk("steamcloud", 0, false, false, ""),
	}}
	assert.NotNil(t, newSuggestionList("G", res, use, add, hide))
}

func TestArtSpecFor(t *testing.T) {
	test.NewApp()
	for _, s := range artSpecs() {
		assert.Equal(t, s.kind, artSpecFor(s.kind).kind)
	}
	assert.Equal(t, services.ArtCover, artSpecFor(services.ArtCover).kind)
}

func TestSlotKeys(t *testing.T) {
	a := &slotForm{key: "saves"}
	b := &slotForm{key: "saves-2"}
	m := &emuForm{slots: []*slotForm{a, b}}
	assert.Equal(t, "saves-3", m.uniqueKey("saves", nil))
	assert.Equal(t, "saves", m.uniqueKey("saves", a))
	m.addSlot()
	assert.Len(t, m.slots, 3)
	assert.True(t, m.slots[2].isNew)
}
