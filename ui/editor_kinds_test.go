package ui

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/akfaiz/carryover/services"
)

func sg(kind, path string, exists bool) services.EmuSuggestion {
	return services.EmuSuggestion{Kind: kind, Path: path, Exists: exists}
}

func TestUseKindFillsTheSlotOfThatName(t *testing.T) {
	// The untouched default slot of a new emulator entry becomes the account slot, then a device slot is added.
	m := &emuForm{slots: []*slotForm{{key: "saves", label: "Saves", isNew: true}}}
	m.useSuggestion(sg(saveKindAccount, "/a", true))
	m.useSuggestion(sg(saveKindDevice, "/d", true))
	if assert.Len(t, m.slots, 2) {
		assert.Equal(t, saveKindAccount, m.slots[0].key)
		assert.False(t, m.slots[0].isNew, "the key names the cloud folder and must not follow the label")
		assert.Equal(t, "/a", m.slots[0].path)
		assert.Equal(t, saveKindDevice, m.slots[1].key)
	}

	// A path of another device is replaced in the slot of the same kind, not added again.
	m.useSuggestion(sg(saveKindAccount, "/b", true))
	assert.Len(t, m.slots, 2)
	assert.Equal(t, "/b", m.slots[0].path)

	// A slot that already has a path (the old layout) is left alone.
	old := &emuForm{slots: []*slotForm{{key: "saves", label: "Saves", path: "/old", include: "*/x", isNew: true}}}
	old.useSuggestion(sg(saveKindAccount, "/a", true))
	assert.Len(t, old.slots, 2)
	assert.Equal(t, "/old", old.slots[0].path)
	assert.Equal(t, "*/x", old.slots[0].include)
}

func TestAutofillKinds(t *testing.T) {
	// A new device: the game's slots exist with no paths, so both get this device's paths even if the folders don't exist yet.
	m := &emuForm{slots: []*slotForm{{key: saveKindAccount}, {key: saveKindDevice}}}
	m.autofillKinds([]services.EmuSuggestion{sg(saveKindAccount, "/a", false), sg(saveKindDevice, "/d", false)})
	assert.Equal(t, "/a", m.slots[0].path)
	assert.Equal(t, "/d", m.slots[1].path)

	// A path already set is kept.
	m.autofillKinds([]services.EmuSuggestion{sg(saveKindAccount, "/other", true)})
	assert.Equal(t, "/a", m.slots[0].path)

	// Without the slots, only locations that exist on disk create them.
	n := &emuForm{slots: []*slotForm{{key: "saves", isNew: true}}}
	n.autofillKinds([]services.EmuSuggestion{sg(saveKindAccount, "/a", true), sg(saveKindDevice, "/d", false)})
	if assert.Len(t, n.slots, 1) {
		assert.Equal(t, saveKindAccount, n.slots[0].key)
	}

	// Several candidates of one kind (several profiles): the user chooses.
	s := &emuForm{slots: []*slotForm{{key: saveKindAccount}}}
	s.autofillKinds([]services.EmuSuggestion{sg(saveKindAccount, "/a", true), sg(saveKindAccount, "/b", true)})
	assert.Empty(t, s.slots[0].path)
}

// An emulator that shares another's cloud save has that emulator's save slots:
// its own paths go into the slot of the same kind, and no slot is added.
func TestSharedEmulatorOnlyFillsTheSharedSlots(t *testing.T) {
	m := &emuForm{share: "eden", slots: []*slotForm{{key: saveKindAccount}, {key: saveKindDevice}}}
	assert.True(t, m.useSuggestion(sg(saveKindAccount, "{ryujinx:X:account}", true)))
	assert.Equal(t, "{ryujinx:X:account}", m.slots[0].path)
	m.autofillKinds([]services.EmuSuggestion{sg(saveKindDevice, "{ryujinx:X:device}", false)})
	assert.Equal(t, "{ryujinx:X:device}", m.slots[1].path, "a device whose folder isn't there yet still gets its path")

	// The other emulator's slots aren't named by kind: nothing can be matched, and none is added.
	o := &emuForm{share: "eden", slots: []*slotForm{{key: "saves"}}}
	assert.False(t, o.useSuggestion(sg(saveKindAccount, "/a", true)))
	assert.False(t, o.useSuggestion(sg("", "/b", true)))
	o.autofillKinds([]services.EmuSuggestion{sg(saveKindAccount, "/a", true)})
	if assert.Len(t, o.slots, 1) {
		assert.Empty(t, o.slots[0].path)
	}
}
