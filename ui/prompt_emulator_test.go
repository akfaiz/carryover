package ui

import (
	"testing"
	"time"

	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/akfaiz/carryover/internal/core"
	"github.com/akfaiz/carryover/services"
)

// emulatorPrompt is a prompt with the game's own launch config and two emulators,
// Ryujinx being the default, shown in a queue like the real one.
func emulatorPrompt(h *harness, seconds int) (*promptQueue, services.Prompt, <-chan string, func() []*widget.Button) {
	p := services.Prompt{
		ID: "p1", Kind: "emulator", GameName: "Game A", Default: "ryu", Seconds: seconds,
		Choices: []services.ProfileChoice{{ID: core.NativeProfile, Name: "Without an emulator"}, {ID: "eden", Name: "Eden"}, {ID: "ryu", Name: "Ryujinx"}},
	}
	q := &promptQueue{u: h.u, queue: []services.Prompt{p}}
	answers := make(chan string, 4)
	body := q.emulatorChoice(p, func(a string) { answers <- a })
	return q, p, answers, func() []*widget.Button { return widgetsOf[*widget.Button](body) }
}

func button(t *testing.T, buttons []*widget.Button, text string) *widget.Button {
	t.Helper()
	for _, b := range buttons {
		if b.Text == text {
			return b
		}
	}
	require.Failf(t, "button not found", "%q among %d buttons", text, len(buttons))
	return nil
}

// settleFor lets the countdown run for about d.
func (h *harness) settleFor(d time.Duration) {
	h.t.Helper()
	for end := time.Now().Add(d); time.Now().Before(end); {
		h.settle()
	}
}

func TestEmulatorPromptButtons(t *testing.T) {
	h := newHarness(t)
	_, _, answers, buttons := emulatorPrompt(h, 30)
	bs := buttons()
	assert.Equal(t, widget.HighImportance, button(t, bs, "Ryujinx").Importance, "the default is highlighted")
	assert.NotEqual(t, widget.HighImportance, button(t, bs, "Eden").Importance)

	test.Tap(button(t, bs, "Eden"))
	assert.Equal(t, "eden", <-answers)
	h.settleFor(1500 * time.Millisecond)
	assert.Empty(t, answers, "the countdown stops once a choice is made")
}

func TestEmulatorPromptStartsTheDefaultWhenTheCountdownEnds(t *testing.T) {
	h := newHarness(t)
	_, _, answers, _ := emulatorPrompt(h, 2)
	h.settleFor(3500 * time.Millisecond)
	select {
	case a := <-answers:
		assert.Equal(t, "ryu", a)
	default:
		assert.Fail(t, "nothing was chosen when the countdown ended")
	}
	assert.Empty(t, answers, "chosen once")
}

func TestEmulatorPromptCanBeStoppedAndCancelled(t *testing.T) {
	h := newHarness(t)
	_, _, answers, buttons := emulatorPrompt(h, 2)
	test.Tap(button(t, buttons(), "Stop countdown"))
	h.settleFor(3500 * time.Millisecond)
	assert.Empty(t, answers, "a stopped countdown never chooses")
	test.Tap(button(t, buttons(), "Cancel"))
	assert.Equal(t, "cancel", <-answers)
}

func TestEmulatorPromptWithdrawnStopsTheCountdown(t *testing.T) {
	h := newHarness(t)
	q, _, answers, _ := emulatorPrompt(h, 2)
	q.queue = nil // the session that asked ended
	h.settleFor(3500 * time.Millisecond)
	assert.Empty(t, answers)
}
