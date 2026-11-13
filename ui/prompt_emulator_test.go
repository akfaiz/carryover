package ui

import (
	"testing"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/akfaiz/carryover/internal/core"
	"github.com/akfaiz/carryover/internal/ui/kit"
	"github.com/akfaiz/carryover/services"
)

// emulatorPrompt is a prompt with the game's own launch config and two emulators,
// Ryujinx being the default, shown in a queue like the real one.
func emulatorPrompt(h *harness, seconds int) (*promptQueue, services.Prompt, <-chan string, fyne.CanvasObject) {
	p := services.Prompt{
		ID: "p1", Kind: "emulator", GameName: "Game A", Default: "ryu", Seconds: seconds,
		Choices: []services.ProfileChoice{{ID: core.NativeProfile, Name: "Without an emulator"}, {ID: "eden", Name: "Eden"}, {ID: "ryu", Name: "Ryujinx"}},
	}
	q := &promptQueue{u: h.u, queue: []services.Prompt{p}}
	answers := make(chan string, 4)
	body, _ := q.emulatorChoice(p, func(a string) { answers <- a }, kit.NewNav(h.u.win, func() {}))
	return q, p, answers, body
}

// tile is the choice tile named name.
func tile(t *testing.T, body fyne.CanvasObject, name string) *kit.ChoiceTile {
	t.Helper()
	for _, c := range widgetsOf[*kit.ChoiceTile](body) {
		if c.Name == name {
			return c
		}
	}
	require.Failf(t, "tile not found", "%q", name)
	return nil
}

// buttonsOf lists the buttons in root: plain ones and those of a kit.Nav.
func buttonsOf(root fyne.CanvasObject) []*widget.Button {
	out := widgetsOf[*widget.Button](root)
	for _, nb := range widgetsOf[*kit.NavButton](root) {
		out = append(out, &nb.Button)
	}
	return out
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
	_, _, answers, body := emulatorPrompt(h, 30)
	assert.True(t, tile(t, body, "Ryujinx").Highlighted, "the default is highlighted")
	assert.False(t, tile(t, body, "Eden").Highlighted)
	assert.Len(t, widgetsOf[*kit.ChoiceTile](body), 3, "one square tile per way to play")

	test.Tap(tile(t, body, "Eden").Btn)
	assert.Equal(t, "eden", <-answers)
	h.settleFor(1500 * time.Millisecond)
	assert.Empty(t, answers, "the countdown stops once a kit.Choice is made")
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
	_, _, answers, body := emulatorPrompt(h, 2)
	test.Tap(button(t, buttonsOf(body), "Stop countdown"))
	h.settleFor(3500 * time.Millisecond)
	assert.Empty(t, answers, "a stopped countdown never chooses")
	test.Tap(button(t, buttonsOf(body), "Cancel"))
	assert.Equal(t, "cancel", <-answers)
}

func TestEmulatorPromptWithdrawnStopsTheCountdown(t *testing.T) {
	h := newHarness(t)
	q, _, answers, _ := emulatorPrompt(h, 2)
	q.queue = nil // the session that asked ended
	h.settleFor(3500 * time.Millisecond)
	assert.Empty(t, answers)
}
