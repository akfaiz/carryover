package kit

import (
	"testing"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestActionForKey(t *testing.T) {
	for k, want := range map[fyne.KeyName]PadAction{
		fyne.KeyLeft: PadPrev, fyne.KeyUp: PadPrev,
		fyne.KeyRight: PadNext, fyne.KeyDown: PadNext,
		fyne.KeyReturn: PadAccept, fyne.KeyEnter: PadAccept,
		fyne.KeyEscape: PadBack, fyne.KeyBackspace: PadBack,
		fyne.KeyA: PadNone, fyne.KeySpace: PadNone, fyne.KeyTab: PadNone,
	} {
		assert.Equal(t, want, actionForKey(k), k)
	}
}

// A press fires once; a held direction repeats after a delay; releasing and
// pressing again fires again. A and B never repeat.
func TestPadTracker(t *testing.T) {
	var tr padTracker
	now := time.Unix(1000, 0)
	at := now.Add

	assert.Empty(t, tr.update(PadState{}, at(0)))
	assert.Equal(t, []PadAction{PadNext}, tr.update(PadState{Down: true}, at(0)), "the press")
	assert.Empty(t, tr.update(PadState{Down: true}, at(100*time.Millisecond)), "held, before the repeat delay")
	assert.Equal(t, []PadAction{PadNext}, tr.update(PadState{Down: true}, at(500*time.Millisecond)), "repeat")
	assert.Empty(t, tr.update(PadState{Down: true}, at(550*time.Millisecond)), "repeats are spaced")
	assert.Equal(t, []PadAction{PadNext}, tr.update(PadState{Down: true}, at(650*time.Millisecond)))
	assert.Empty(t, tr.update(PadState{}, at(700*time.Millisecond)), "released")
	assert.Equal(t, []PadAction{PadPrev}, tr.update(PadState{Left: true}, at(710*time.Millisecond)), "left is previous")
	tr.update(PadState{}, at(720*time.Millisecond))

	assert.Equal(t, []PadAction{PadAccept}, tr.update(PadState{Accept: true}, at(800*time.Millisecond)))
	assert.Empty(t, tr.update(PadState{Accept: true}, at(5*time.Second)), "A doesn't repeat")
	tr.update(PadState{}, at(6*time.Second))
	assert.Equal(t, []PadAction{PadBack}, tr.update(PadState{Back: true}, at(7*time.Second)))
	assert.Equal(t, []PadAction{PadPrev, PadAccept}, tr.update(PadState{Up: true, Accept: true}, at(8*time.Second)), "several at once")
}

// A button still held from the press that opened the dialog (A starts the
// shortcut) must not answer it: it counts only after being released.
func TestPadTrackerIgnoresButtonsHeldAtStart(t *testing.T) {
	var tr padTracker
	now := time.Unix(1000, 0)
	tr.prime(PadState{Accept: true, Down: true}, now)
	for i := 1; i < 40; i++ {
		assert.Empty(t, tr.update(PadState{Accept: true, Down: true}, now.Add(time.Duration(i)*100*time.Millisecond)),
			"still held after %d00 ms", i)
	}
	tr.update(PadState{}, now.Add(5*time.Second))
	assert.Equal(t, []PadAction{PadAccept}, tr.update(PadState{Accept: true}, now.Add(6*time.Second)), "pressed again")
}

// navFixture is a dialog of three buttons driven by a Nav.
type navFixture struct {
	w       fyne.Window
	nav     *Nav
	btns    []*NavButton
	tapped  []string
	backs   int
	outside int // presses of the window's own key handler
}

func newNavFixture(t *testing.T) *navFixture {
	t.Helper()
	test.NewApp()
	f := &navFixture{w: test.NewWindow(widget.NewLabel("page"))}
	f.w.Canvas().SetOnTypedKey(func(*fyne.KeyEvent) { f.outside++ })
	f.nav = NewNav(f.w, func() { f.backs++ })
	var objs []fyne.CanvasObject
	for _, name := range []string{"A", "B", "C"} {
		b := f.nav.Button(name, widget.MediumImportance, func() { f.tapped = append(f.tapped, name) })
		f.btns = append(f.btns, b)
		objs = append(objs, b)
	}
	dialog.NewCustomWithoutButtons("Title", container.NewHBox(objs...), f.w).Show()
	return f
}

func (f *navFixture) focused() fyne.Focusable { return f.w.Canvas().Focused() }

func key(name fyne.KeyName) *fyne.KeyEvent { return &fyne.KeyEvent{Name: name} }

func TestNavMovesFocusAndPresses(t *testing.T) {
	f := newNavFixture(t)
	f.nav.Start(f.btns[1])
	require.Same(t, f.btns[1], f.focused(), "starts on the safe button")

	f.btns[1].TypedKey(key(fyne.KeyRight))
	require.Same(t, f.btns[2], f.focused())
	f.btns[2].TypedKey(key(fyne.KeyDown))
	require.Same(t, f.btns[0], f.focused(), "wraps around")
	f.btns[0].TypedKey(key(fyne.KeyUp))
	require.Same(t, f.btns[2], f.focused())
	f.btns[2].TypedKey(key(fyne.KeyLeft))
	require.Same(t, f.btns[1], f.focused())

	f.btns[1].TypedKey(key(fyne.KeyReturn))
	f.btns[1].TypedKey(key(fyne.KeySpace)) // Fyne's own key still works
	assert.Equal(t, []string{"B", "B"}, f.tapped)

	f.btns[1].TypedKey(key(fyne.KeyEscape))
	assert.Equal(t, 1, f.backs)
	assert.Zero(t, f.outside)
}

func TestNavSkipsDisabledButtons(t *testing.T) {
	f := newNavFixture(t)
	f.btns[1].Disable()
	f.nav.Start(f.btns[0])
	f.btns[0].TypedKey(key(fyne.KeyRight))
	require.Same(t, f.btns[2], f.focused(), "the disabled button is skipped")
}

// After a click on empty space nothing is focused; keys then reach the window
// instead of a button, and still work.
func TestNavHandlesKeysWithoutFocus(t *testing.T) {
	f := newNavFixture(t)
	f.nav.Start(f.btns[0])
	f.w.Canvas().Unfocus()
	require.Nil(t, f.focused())

	f.w.Canvas().OnTypedKey()(key(fyne.KeyEscape))
	assert.Equal(t, 1, f.backs)
	f.w.Canvas().OnTypedKey()(key(fyne.KeyRight))
	require.NotNil(t, f.focused(), "an arrow key picks a button")
	f.w.Canvas().OnTypedKey()(key(fyne.KeyReturn))
	assert.Len(t, f.tapped, 1, "Enter presses it")
}

func TestNavStopRestoresTheWindowsKeys(t *testing.T) {
	f := newNavFixture(t)
	f.nav.Start(f.btns[0])
	f.w.Canvas().OnTypedKey()(key(fyne.KeyEscape))
	require.Zero(t, f.outside)
	f.nav.Stop()
	f.w.Canvas().OnTypedKey()(key(fyne.KeyEscape))
	assert.Equal(t, 1, f.outside, "the window's own handler is back")
	assert.Equal(t, 1, f.backs)
	f.nav.Stop() // twice is harmless
}

func TestNavOnInput(t *testing.T) {
	f := newNavFixture(t)
	inputs := 0
	f.nav.OnInput(func() { inputs++ })
	f.nav.Start(f.btns[0])
	f.btns[0].TypedKey(key(fyne.KeyRight))
	f.btns[1].TypedKey(key(fyne.KeyA)) // not a navigation key
	assert.Equal(t, 1, inputs)
	f.btns[1].TypedKey(key(fyne.KeyEscape))
	assert.Equal(t, 2, inputs, "going back is input too")
}

// The gamepad drives the same actions through poll.
func TestNavGamepad(t *testing.T) {
	f := newNavFixture(t)
	var state PadState
	f.nav.pad = func() (PadState, bool) { return state, true }
	f.nav.Start(f.btns[0]) // starts the poller; the test polls by hand too
	defer f.nav.Stop()
	now := time.Unix(1000, 0)
	step := func(s PadState) {
		state = s
		now = now.Add(time.Second)
		f.nav.poll(now)
	}
	step(PadState{})
	step(PadState{Right: true})
	require.Same(t, f.btns[1], f.focused())
	step(PadState{})
	step(PadState{Accept: true})
	assert.Equal(t, []string{"B"}, f.tapped)
	step(PadState{})
	step(PadState{Back: true})
	assert.Equal(t, 1, f.backs)
}

func TestNavChoiceTile(t *testing.T) {
	test.NewApp()
	w := test.NewWindow(widget.NewLabel("page"))
	nav := NewNav(w, func() {})
	chosen := ""
	a := nav.ChoiceTile("A", nil, false, func() { chosen = "A" })
	b := nav.ChoiceTile("B", nil, true, func() { chosen = "B" })
	dialog.NewCustomWithoutButtons("t", container.NewHBox(a, b), w).Show()
	nav.Start(b.Focus)
	require.Same(t, b.Focus, w.Canvas().Focused(), "the tile takes the focus")
	b.Focus.TypedKey(key(fyne.KeyLeft))
	require.Same(t, a.Focus, w.Canvas().Focused())
	a.Focus.TypedKey(key(fyne.KeyEnter))
	assert.Equal(t, "A", chosen)
	test.Tap(b.Btn)
	assert.Equal(t, "B", chosen, "tapping the tile's button still answers")
}
