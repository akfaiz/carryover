package prompts_test

import (
	"testing"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/widget"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/akfaiz/carryover/internal/cloudsync/syncer"
	"github.com/akfaiz/carryover/services"
	"github.com/akfaiz/carryover/ui/env"
	"github.com/akfaiz/carryover/ui/internal/uitest"
	"github.com/akfaiz/carryover/ui/kit"
	"github.com/akfaiz/carryover/ui/shell"
)

// elsewhere stands in for the page behind the prompts.
type elsewhere struct{}

func (elsewhere) View() fyne.CanvasObject { return widget.NewLabel("elsewhere") }

// newHarness shows a shell with a stand-in page: the prompts are the shell's,
// and need only its window.
func newHarness(t *testing.T) *uitest.Harness {
	t.Helper()
	return uitest.New(t, shell.Config{
		Page:         func(env.Env, string) env.Page { return elsewhere{} },
		AddGame:      func(env.Env) {},
		SyncProblems: func(env.Env, int, []services.SyncResult) {},
	})
}

func conflictPrompt(id string, beforePlay bool, localNewer bool) services.Prompt {
	now := time.Now()
	local := syncer.Summary{Files: 2, Bytes: 2048, Newest: now, DeviceName: "Deck"}
	remote := syncer.Summary{Files: 3, Bytes: 4096, Newest: now.Add(-time.Hour), DeviceName: "PC"}
	if !localNewer {
		local.Newest, remote.Newest = remote.Newest, local.Newest
	}
	return services.Prompt{
		ID: id, Kind: "conflict", GameID: "g1", GameName: "Game A", BeforePlay: beforePlay,
		Conflict: &services.ConflictInfo{Local: local, Remote: remote},
	}
}

func TestPromptConflictChoices(t *testing.T) {
	h := newHarness(t)
	h.Shell.Prompts().Add(conflictPrompt("p1", true, true))
	require.NotNil(t, h.Shell.Prompts().Current())
	assert.ElementsMatch(t, []string{"Keep this device's saves", "Keep cloud saves", "Skip sync and play"}, uitest.ButtonTexts(h))
	assert.True(t, uitest.HasBadge(h, "Newer"), "the newer side is marked")

	// Answering closes the dialog.
	uitest.TapOverlayButton(t, h, "Keep cloud saves")
	assert.Nil(t, h.Shell.Prompts().Current())
	assert.Empty(t, h.Shell.Prompts().Pending())
	assert.Empty(t, h.Shell.Window().Canvas().Overlays().List())
}

func TestPromptConflictAfterPlayOffersDecideLater(t *testing.T) {
	h := newHarness(t)
	h.Shell.Prompts().Add(conflictPrompt("p1", false, false))
	assert.Contains(t, uitest.ButtonTexts(h), "Decide later")
	assert.NotContains(t, uitest.ButtonTexts(h), "Skip sync and play")
	uitest.TapOverlayButton(t, h, "Decide later")
	assert.Nil(t, h.Shell.Prompts().Current())
}

func TestPromptKinds(t *testing.T) {
	cases := []struct {
		name    string
		prompt  services.Prompt
		buttons []string
	}{
		{"lock", services.Prompt{ID: "l", Kind: "lock", GameName: "Game A", Lock: &syncer.Lock{DeviceName: "PC", Since: time.Now()}}, []string{"Cancel", "Play anyway"}},
		{"sync error before play", services.Prompt{ID: "e1", Kind: "syncError", GameName: "Game A", Error: "boom", BeforePlay: true}, []string{"Cancel", "Play offline"}},
		{"sync error after play", services.Prompt{ID: "e2", Kind: "syncError", GameName: "Game A", Error: "boom"}, []string{"OK"}},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			h := newHarness(t)
			h.Shell.Prompts().Add(tt.prompt)
			assert.ElementsMatch(t, tt.buttons, uitest.ButtonTexts(h))
			uitest.TapOverlayButton(t, h, tt.buttons[len(tt.buttons)-1])
			assert.Nil(t, h.Shell.Prompts().Current(), "answering closes it")
		})
	}
}

// Prompts are shown one at a time, in order, and the same one isn't queued twice.
func TestPromptQueue(t *testing.T) {
	h := newHarness(t)
	first := services.Prompt{ID: "a", Kind: "syncError", GameName: "A", Error: "one"}
	second := services.Prompt{ID: "b", Kind: "syncError", GameName: "B", Error: "two"}
	h.Shell.Prompts().Add(first)
	h.Shell.Prompts().Add(second)
	h.Shell.Prompts().Add(first) // already queued
	require.Len(t, h.Shell.Prompts().Pending(), 2)
	assert.Equal(t, "a", h.Shell.Prompts().Pending()[0].ID)

	uitest.TapOverlayButton(t, h, "OK")
	require.NotNil(t, h.Shell.Prompts().Current(), "the next prompt appears")
	require.Len(t, h.Shell.Prompts().Pending(), 1)
	assert.Equal(t, "b", h.Shell.Prompts().Pending()[0].ID)

	uitest.TapOverlayButton(t, h, "OK")
	assert.Nil(t, h.Shell.Prompts().Current())
	assert.Empty(t, h.Shell.Prompts().Pending())
}

// A prompt withdrawn because its sync was cancelled disappears, whether it is
// the one on screen or still waiting.
func TestPromptRemove(t *testing.T) {
	h := newHarness(t)
	h.Shell.Prompts().Add(services.Prompt{ID: "a", Kind: "syncError", GameName: "A", Error: "one"})
	h.Shell.Prompts().Add(services.Prompt{ID: "b", Kind: "syncError", GameName: "B", Error: "two"})
	h.Shell.Prompts().Add(services.Prompt{ID: "c", Kind: "syncError", GameName: "C", Error: "three"})

	h.Shell.Prompts().Remove("c") // waiting
	require.Len(t, h.Shell.Prompts().Pending(), 2)
	h.Shell.Prompts().Remove("a") // on screen: the next one takes over
	require.Len(t, h.Shell.Prompts().Pending(), 1)
	assert.Equal(t, "b", h.Shell.Prompts().Pending()[0].ID)
	require.NotNil(t, h.Shell.Prompts().Current())
	h.Shell.Prompts().Remove("nope") // unknown: ignored
	require.Len(t, h.Shell.Prompts().Pending(), 1)
	h.Shell.Prompts().Remove("b")
	assert.Nil(t, h.Shell.Prompts().Current())
	assert.Empty(t, h.Shell.Window().Canvas().Overlays().List())
}

// focusedText is the text of the focused button of the open prompt.
func focusedText(t *testing.T, h *uitest.Harness) string {
	t.Helper()
	switch f := h.Shell.Window().Canvas().Focused().(type) {
	case *kit.NavButton:
		return f.Text
	case nil:
		require.Fail(t, "nothing is focused")
	default:
		require.Failf(t, "unexpected focus", "%T", f)
	}
	return ""
}

func pressKey(h *uitest.Harness, name fyne.KeyName) {
	if f := h.Shell.Window().Canvas().Focused(); f != nil {
		f.TypedKey(&fyne.KeyEvent{Name: name})
	} else if fn := h.Shell.Window().Canvas().OnTypedKey(); fn != nil {
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
			h.Shell.Prompts().Add(tt.prompt)
			require.NotNil(t, h.Shell.Prompts().Current())
			assert.Equal(t, tt.safe, focusedText(t, h))

			// Escape answers it the safe way and closes the dialog.
			pressKey(h, fyne.KeyEscape)
			assert.Nil(t, h.Shell.Prompts().Current())
			assert.Empty(t, h.Shell.Prompts().Pending())
			assert.Empty(t, h.Shell.Window().Canvas().Overlays().List())
		})
	}
}

// The arrow keys move between the two ways to resolve a conflict, and Enter
// presses the one that has the focus.
func TestConflictPromptArrowsAndEnter(t *testing.T) {
	h := newHarness(t)
	h.Shell.Prompts().Add(conflictPrompt("p1", true, true))
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
	assert.Nil(t, h.Shell.Prompts().Current(), "Enter pressed the focused button")
}

// The emulator prompt starts on the highlighted choice, and the first press of
// an arrow key stops the countdown: the player is choosing.
func TestEmulatorPromptNavigation(t *testing.T) {
	h := newHarness(t)
	h.Shell.Prompts().Add(services.Prompt{
		ID: "p1", Kind: "emulator", GameName: "Game A", Default: "ryu", Seconds: 30,
		Choices: []services.ProfileChoice{{ID: services.NativeProfile, Name: "Without an emulator"}, {ID: "eden", Name: "Eden"}, {ID: "ryu", Name: "Ryujinx"}},
	})
	tiles := uitest.OverlayWidgets[*kit.ChoiceTile](h)
	require.Len(t, tiles, 3)
	byName := map[string]*kit.ChoiceTile{}
	for _, c := range tiles {
		byName[c.Name] = c
	}
	require.Same(t, byName["Ryujinx"].Focus, h.Shell.Window().Canvas().Focused(), "the highlighted choice has the focus")
	stop := uitest.ButtonWith(t, uitest.OverlayButtons(h), "Stop countdown")
	require.True(t, stop.Visible(), "the countdown is running")

	pressKey(h, fyne.KeyLeft)
	assert.Same(t, byName["Eden"].Focus, h.Shell.Window().Canvas().Focused(), "moved to the previous choice")
	assert.False(t, stop.Visible(), "moving stops the countdown")

	pressKey(h, fyne.KeyEscape)
	assert.Nil(t, h.Shell.Prompts().Current(), "Escape cancels")
}

// A Steam shortcut's window says how to use the controller.
func TestWrapperPromptShowsTheControllerHint(t *testing.T) {
	h := newHarness(t)
	h.Shell.Prompts().Add(conflictPrompt("p1", true, true))
	assert.False(t, uitest.HasBadge(h, "D-pad or arrow keys to choose, A or Enter to confirm, B or Esc to go back."), "no hint in the launcher")
	h.Shell.Prompts().Remove("p1")

	h.Shell.Prompts().Wrapper = true // as in the session wrapper's window
	h.Shell.Prompts().Add(conflictPrompt("p2", true, true))
	assert.True(t, uitest.HasBadge(h, "D-pad or arrow keys to choose, A or Enter to confirm, B or Esc to go back."))
}
