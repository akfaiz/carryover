package prompts_test

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
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

func (elsewhere) View(c *ui.Context) { ui.Text(c, "elsewhere") }

// newHarness shows a shell with a stand-in page: the prompts are the shell's.
func newHarness(t *testing.T) *uitest.Harness {
	t.Helper()
	return uitest.New(t, shell.Config{
		Page:         func(env.Env, string) env.Page { return elsewhere{} },
		AddGame:      func(env.Env) {},
		SyncProblems: func(env.Env, int, []services.SyncResult) {},
	})
}

func add(h *uitest.Harness, p services.Prompt) {
	h.Shell.Prompts().Add(p)
	h.Settle()
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
	add(h, conflictPrompt("p1", true, true))
	require.NotNil(t, h.Shell.Prompts().Current())
	for _, want := range []string{"Keep this device's saves", "Keep cloud saves", "Skip sync and play", "Newer"} {
		assert.Contains(t, h.Texts(), want)
	}

	// Answering closes the dialog.
	h.Click("Keep cloud saves")
	assert.Nil(t, h.Shell.Prompts().Current())
	assert.Empty(t, h.Shell.Prompts().Pending())
	assert.False(t, h.Shell.Dialogs().Open())
}

func TestPromptConflictAfterPlayOffersDecideLater(t *testing.T) {
	h := newHarness(t)
	add(h, conflictPrompt("p1", false, false))
	assert.Contains(t, h.Texts(), "Decide later")
	assert.NotContains(t, h.Texts(), "Skip sync and play")
	h.Click("Decide later")
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
			add(h, tt.prompt)
			for _, b := range tt.buttons {
				assert.Contains(t, h.Texts(), b)
			}
			h.Click(tt.buttons[len(tt.buttons)-1])
			assert.Nil(t, h.Shell.Prompts().Current(), "answering closes it")
		})
	}
}

// Prompts are shown one at a time, in order, and the same one isn't queued twice.
func TestPromptQueue(t *testing.T) {
	h := newHarness(t)
	first := services.Prompt{ID: "a", Kind: "syncError", GameName: "A", Error: "one"}
	second := services.Prompt{ID: "b", Kind: "syncError", GameName: "B", Error: "two"}
	add(h, first)
	add(h, second)
	add(h, first) // already queued
	require.Len(t, h.Shell.Prompts().Pending(), 2)
	assert.Equal(t, "a", h.Shell.Prompts().Pending()[0].ID)

	h.Click("OK")
	require.NotNil(t, h.Shell.Prompts().Current(), "the next prompt appears")
	require.Len(t, h.Shell.Prompts().Pending(), 1)
	assert.Equal(t, "b", h.Shell.Prompts().Pending()[0].ID)

	h.Click("OK")
	assert.Nil(t, h.Shell.Prompts().Current())
	assert.Empty(t, h.Shell.Prompts().Pending())
}

// A prompt withdrawn because its sync was cancelled disappears, whether it is
// the one on screen or still waiting.
func TestPromptRemove(t *testing.T) {
	h := newHarness(t)
	for _, id := range []string{"a", "b", "c"} {
		add(h, services.Prompt{ID: id, Kind: "syncError", GameName: id, Error: "x"})
	}
	h.Shell.Prompts().Remove("c") // waiting
	require.Len(t, h.Shell.Prompts().Pending(), 2)
	h.Shell.Prompts().Remove("a") // on screen: the next one takes over
	require.Len(t, h.Shell.Prompts().Pending(), 1)
	assert.Equal(t, "b", h.Shell.Prompts().Pending()[0].ID)
	require.NotNil(t, h.Shell.Prompts().Current())
	h.Shell.Prompts().Remove("nope") // unknown: ignored
	require.Len(t, h.Shell.Prompts().Pending(), 1)
	h.Shell.Prompts().Remove("b")
	h.Settle()
	assert.Nil(t, h.Shell.Prompts().Current())
	assert.False(t, h.Shell.Dialogs().Open())
}

// The gamepad: a prompt starts on its safe button, B gives the safe answer, and
// the page behind is out of reach.
func TestPromptPadStartsSafeAndBackAnswersSafe(t *testing.T) {
	h := newHarness(t)
	add(h, services.Prompt{ID: "l", Kind: "lock", GameName: "G", Lock: &syncer.Lock{DeviceName: "PC", Since: time.Now()}})
	h.Settle()
	assert.True(t, h.Tester.Focused("Cancel"), "starts on the safe button, not Play anyway")

	nav := h.Shell.Navigator()
	nav.Press(kit.PadRight)
	nav.Press(kit.PadLeft)
	nav.Press(kit.PadDown)
	nav.Press(kit.PadUp)
	h.Settle()
	assert.True(t, h.Tester.Focused("Cancel") || h.Tester.Focused("Play anyway"), "focus stays in the prompt")

	nav.Press(kit.PadBack)
	h.Settle()
	assert.Nil(t, h.Shell.Prompts().Current(), "B answers with the safe answer")
}

func emulatorPrompt(seconds int) services.Prompt {
	return services.Prompt{
		ID: "emu", Kind: "emulator", GameName: "Game A", Default: "e2", Seconds: seconds,
		Choices: []services.ProfileChoice{
			{ID: services.NativeProfile}, {ID: "e1", Name: "Eden"}, {ID: "e2", Name: "Citron"},
		},
	}
}

func TestPromptEmulatorChoices(t *testing.T) {
	h := newHarness(t)
	add(h, emulatorPrompt(0))
	for _, want := range []string{"Without an emulator", "Eden", "Citron", "Cancel"} {
		assert.Contains(t, h.Texts(), want)
	}
	assert.NotContains(t, h.Texts(), "Stop countdown", "no countdown without seconds")
	assert.True(t, h.Tester.Focused("Citron"), "starts on the default")
	h.Click("Eden")
	assert.Nil(t, h.Shell.Prompts().Current())
}

// Any press of the pad stops the countdown: whoever is choosing is choosing.
func TestPromptEmulatorCountdownStopsOnInput(t *testing.T) {
	h := newHarness(t)
	add(h, emulatorPrompt(30))
	assert.Contains(t, h.Texts(), "Stop countdown")
	assert.Contains(t, h.Texts(), "Starting with Citron in 30 seconds")

	h.Shell.Navigator().Press(kit.PadLeft)
	h.Settle()
	assert.NotContains(t, h.Texts(), "Stop countdown")
	assert.NotContains(t, h.Texts(), "Starting with Citron in 30 seconds")
	require.NotNil(t, h.Shell.Prompts().Current(), "still open: a choice is needed")

	h.Click("Cancel")
	assert.Nil(t, h.Shell.Prompts().Current())
}

func TestPromptEmulatorStopCountdownButton(t *testing.T) {
	h := newHarness(t)
	add(h, emulatorPrompt(30))
	h.Click("Stop countdown")
	assert.NotContains(t, h.Texts(), "Stop countdown")
	require.NotNil(t, h.Shell.Prompts().Current())
}

// The wrapper window's prompts say how to answer with a controller.
func TestPromptWrapperHint(t *testing.T) {
	h := newHarness(t)
	h.Shell.Prompts().Wrapper = true
	add(h, services.Prompt{ID: "x", Kind: "syncError", GameName: "A", Error: "e"})
	assert.Contains(t, h.Texts(), "D-pad or arrow keys to choose, A or Enter to confirm, B or Esc to go back.")
}
