package ui

import (
	"testing"
	"time"

	"fyne.io/fyne/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/akfaiz/carryover/internal/cloudsync/syncer"
	"github.com/akfaiz/carryover/internal/core"
	"github.com/akfaiz/carryover/services"
	"github.com/akfaiz/carryover/ui/kit"
)

// focusedText is the text of the focused button of the open prompt.
func focusedText(t *testing.T, h *harness) string {
	t.Helper()
	switch f := h.u.Window().Canvas().Focused().(type) {
	case *kit.NavButton:
		return f.Text
	case nil:
		require.Fail(t, "nothing is focused")
	default:
		require.Failf(t, "unexpected focus", "%T", f)
	}
	return ""
}

func pressKey(h *harness, name fyne.KeyName) {
	if f := h.u.Window().Canvas().Focused(); f != nil {
		f.TypedKey(&fyne.KeyEvent{Name: name})
	} else if fn := h.u.Window().Canvas().OnTypedKey(); fn != nil {
		fn(&fyne.KeyEvent{Name: name})
	}
}

// Every prompt can be driven with the keyboard or a gamepad. The focus starts
// on the button that overwrites nothing, so a stray Enter or A is harmless, and
// Escape or B gives the same safe answer.
func TestPromptsStartOnTheSafeButton(t *testing.T) {
	cases := []struct {
		name   string
		prompt services.Prompt
		safe   string
	}{
		{"conflict before play", conflictPrompt("c1", true, true), "Skip sync and play"},
		{"conflict after play", conflictPrompt("c2", false, true), "Decide later"},
		{"lock", services.Prompt{ID: "l", Kind: "lock", GameName: "Game A", Lock: &syncer.Lock{DeviceName: "PC", Since: time.Now()}}, "Cancel"},
		{"sync error before play", services.Prompt{ID: "e1", Kind: "syncError", GameName: "Game A", Error: "boom", BeforePlay: true}, "Cancel"},
		{"sync error after play", services.Prompt{ID: "e2", Kind: "syncError", GameName: "Game A", Error: "boom"}, "OK"},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			h := newHarness(t)
			h.u.Prompts().Add(tt.prompt)
			require.NotNil(t, h.u.Prompts().Current())
			assert.Equal(t, tt.safe, focusedText(t, h))

			// Escape answers it the safe way and closes the dialog.
			pressKey(h, fyne.KeyEscape)
			assert.Nil(t, h.u.Prompts().Current())
			assert.Empty(t, h.u.Prompts().Pending())
			assert.Empty(t, h.u.Window().Canvas().Overlays().List())
		})
	}
}

// The arrow keys move between the two ways to resolve a conflict, and Enter
// presses the one that has the focus.
func TestConflictPromptArrowsAndEnter(t *testing.T) {
	h := newHarness(t)
	h.u.Prompts().Add(conflictPrompt("p1", true, true))
	seen := map[string]bool{focusedText(t, h): true}
	for range 4 {
		pressKey(h, fyne.KeyRight)
		seen[focusedText(t, h)] = true
	}
	assert.True(t, seen["Keep this device's saves"] && seen["Keep cloud saves"] && seen["Skip sync and play"],
		"the arrows reach every button, got %v", seen)

	for focusedText(t, h) != "Keep cloud saves" {
		pressKey(h, fyne.KeyRight)
	}
	pressKey(h, fyne.KeyReturn)
	assert.Nil(t, h.u.Prompts().Current(), "Enter pressed the focused button")
}

// The emulator prompt starts on the highlighted choice, and the first press of
// an arrow key stops the countdown: the player is choosing.
func TestEmulatorPromptNavigation(t *testing.T) {
	h := newHarness(t)
	h.u.Prompts().Add(services.Prompt{
		ID: "p1", Kind: "emulator", GameName: "Game A", Default: "ryu", Seconds: 30,
		Choices: []services.ProfileChoice{{ID: core.NativeProfile, Name: "Without an emulator"}, {ID: "eden", Name: "Eden"}, {ID: "ryu", Name: "Ryujinx"}},
	})
	tiles := overlayWidgets[*kit.ChoiceTile](h)
	require.Len(t, tiles, 3)
	byName := map[string]*kit.ChoiceTile{}
	for _, c := range tiles {
		byName[c.Name] = c
	}
	require.Same(t, byName["Ryujinx"].Focus, h.u.Window().Canvas().Focused(), "the highlighted choice has the focus")
	stop := buttonWith(t, overlayButtons(h), "Stop countdown")
	require.True(t, stop.Visible(), "the countdown is running")

	pressKey(h, fyne.KeyLeft)
	assert.Same(t, byName["Eden"].Focus, h.u.Window().Canvas().Focused(), "moved to the previous choice")
	assert.False(t, stop.Visible(), "moving stops the countdown")

	pressKey(h, fyne.KeyEscape)
	assert.Nil(t, h.u.Prompts().Current(), "Escape cancels")
}

// A Steam shortcut's window says how to use the controller.
func TestWrapperPromptShowsTheControllerHint(t *testing.T) {
	h := newHarness(t)
	h.u.Prompts().Add(conflictPrompt("p1", true, true))
	assert.False(t, hasBadge(h, "D-pad or arrow keys to choose, A or Enter to confirm, B or Esc to go back."), "no hint in the launcher")
	h.u.Prompts().Remove("p1")

	h.u.Prompts().Wrapper = true // as in the session wrapper's window
	h.u.Prompts().Add(conflictPrompt("p2", true, true))
	assert.True(t, hasBadge(h, "D-pad or arrow keys to choose, A or Enter to confirm, B or Esc to go back."))
}
