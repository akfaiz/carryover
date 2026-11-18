package prompts

import (
	"testing"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/akfaiz/carryover/internal/core"
	"github.com/akfaiz/carryover/services"
	"github.com/akfaiz/carryover/ui/env"
	"github.com/akfaiz/carryover/ui/kit"
)

// stubEnv is the part of env.Env a prompt uses: the window. Anything else would
// panic on the nil embedded interface, which is the point.
type stubEnv struct {
	env.Env
	win fyne.Window
}

func (s stubEnv) Window() fyne.Window { return s.win }

// harness runs the countdown's UI work on the test's own goroutine, like the
// real app's single UI thread.
type harness struct {
	t     *testing.T
	win   fyne.Window
	queue chan func()
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	h := &harness{t: t, win: test.NewTempWindow(t, widget.NewLabel("")), queue: make(chan func(), 256)}
	old := kit.OnUI
	kit.OnUI = func(f func()) { h.queue <- f }
	t.Cleanup(func() { kit.OnUI = old })
	return h
}

func (h *harness) settle() {
	for {
		select {
		case f := <-h.queue:
			f()
		case <-time.After(50 * time.Millisecond):
			return
		}
	}
}

// widgetsOf lists the widgets of type T under root.
func widgetsOf[T fyne.CanvasObject](root fyne.CanvasObject) []T {
	var out []T
	var walk func(o fyne.CanvasObject)
	walk = func(o fyne.CanvasObject) {
		if x, ok := o.(T); ok {
			out = append(out, x)
		}
		switch x := o.(type) {
		case *fyne.Container:
			for _, c := range x.Objects {
				walk(c)
			}
		case fyne.Widget:
			for _, c := range test.WidgetRenderer(x).Objects() {
				walk(c)
			}
		}
	}
	walk(root)
	return out
}

// emulatorPrompt is a prompt with the game's own launch config and two emulators,
// Ryujinx being the default, shown in a queue like the real one.
func emulatorPrompt(h *harness, seconds int) (*Queue, services.Prompt, <-chan string, fyne.CanvasObject) {
	p := services.Prompt{
		ID: "p1", Kind: "emulator", GameName: "Game A", Default: "ryu", Seconds: seconds,
		Choices: []services.ProfileChoice{{ID: core.NativeProfile, Name: "Without an emulator"}, {ID: "eden", Name: "Eden"}, {ID: "ryu", Name: "Ryujinx"}},
	}
	q := &Queue{e: stubEnv{win: h.win}, queue: []services.Prompt{p}}
	answers := make(chan string, 4)
	body, _ := q.emulatorChoice(p, func(a string) { answers <- a }, kit.NewNav(h.win, func() {}))
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
