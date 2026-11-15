package kit

import (
	"fmt"
	"testing"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A grid of tiles, each with a button under it, as in the library.
func TestPickInDirection(t *testing.T) {
	var rects []navRect
	for row := range 2 {
		for col := range 3 {
			x, y := float32(col*180), float32(row*340)
			rects = append(rects, navRect{x, y, 170, 290}, navRect{x, y + 295, 170, 36}) // tile, then its button
		}
	}
	tile := func(row, col int) int { return (row*3 + col) * 2 }
	button := func(row, col int) int { return tile(row, col) + 1 }

	assert.Equal(t, tile(0, 1), pickInDirection(rects[tile(0, 0)], PadRight, rects))
	assert.Equal(t, button(0, 0), pickInDirection(rects[tile(0, 0)], PadDown, rects), "down from a tile is its button")
	assert.Equal(t, tile(1, 0), pickInDirection(rects[button(0, 0)], PadDown, rects), "then the tile below")
	assert.Equal(t, button(0, 2), pickInDirection(rects[button(0, 1)], PadRight, rects), "along the row of buttons")
	assert.Equal(t, button(0, 1), pickInDirection(rects[tile(1, 1)], PadUp, rects))
	assert.Equal(t, -1, pickInDirection(rects[tile(0, 0)], PadLeft, rects), "nothing to the left")
	assert.Equal(t, -1, pickInDirection(rects[tile(0, 0)], PadUp, rects), "nothing above")

	// Controls of slightly different heights in one row are not above or below
	// each other.
	row := []navRect{{0, 0, 200, 36}, {210, 2, 120, 32}}
	assert.Equal(t, -1, pickInDirection(row[0], PadDown, row))
	assert.Equal(t, 1, pickInDirection(row[0], PadRight, row))
}

// navWindow is a window of a Navigator over a 3×2 grid of buttons.
type navWindow struct {
	w       fyne.Window
	n       *Navigator
	btns    []*widget.Button
	tapped  []string
	backs   int
	section []int
}

func newNavWindow(t *testing.T) *navWindow {
	t.Helper()
	test.NewApp()
	f := &navWindow{}
	grid := container.NewGridWithColumns(3)
	for i := range 6 {
		name := fmt.Sprint(i)
		b := widget.NewButton(name, func() { f.tapped = append(f.tapped, name) })
		f.btns = append(f.btns, b)
		grid.Add(b)
	}
	f.w = test.NewWindow(container.NewVBox(grid))
	f.w.Resize(fyne.NewSize(600, 400))
	f.n = NewNavigator(f.w, NavigatorOptions{
		Back:    func() { f.backs++ },
		Section: func(dir int) { f.section = append(f.section, dir) },
	})
	return f
}

func (f *navWindow) focused() fyne.Focusable { return f.w.Canvas().Focused() }

func TestNavigatorMovesAcrossTheGrid(t *testing.T) {
	f := newNavWindow(t)
	f.n.Apply(PadRight)
	require.Same(t, f.btns[0], f.focused(), "the first press only picks where to start")
	f.n.Apply(PadRight)
	require.Same(t, f.btns[1], f.focused())
	f.n.Apply(PadDown)
	require.Same(t, f.btns[4], f.focused())
	f.n.Apply(PadDown)
	require.Same(t, f.btns[4], f.focused(), "nothing below: stays")
	f.n.Apply(PadLeft)
	require.Same(t, f.btns[3], f.focused())

	f.n.Apply(PadAccept)
	assert.Equal(t, []string{"3"}, f.tapped, "A presses the focused button")

	f.btns[0].Disable()
	f.n.Apply(PadUp)
	assert.Same(t, f.btns[1], f.focused(), "the disabled button above is skipped, for the nearest one up there")

	f.n.Apply(PadBack)
	f.n.Apply(PadTabPrev)
	f.n.Apply(PadTabNext)
	assert.Equal(t, 1, f.backs)
	assert.Equal(t, []int{-1, 1}, f.section, "LB and RB switch sections")
}

func TestNavigatorDefaultFocus(t *testing.T) {
	f := newNavWindow(t)
	f.n.opts.DefaultFocus = func() fyne.Focusable { return f.btns[4] }
	f.n.Apply(PadAccept)
	assert.Same(t, f.btns[4], f.focused(), "A with nothing focused only picks the page's start")
	assert.Empty(t, f.tapped)
}

// The arrow keys and Enter that reach the window work like the D-pad and A.
func TestNavigatorHandleKey(t *testing.T) {
	f := newNavWindow(t)
	assert.True(t, f.n.HandleKey(&fyne.KeyEvent{Name: fyne.KeyDown}))
	assert.True(t, f.n.HandleKey(&fyne.KeyEvent{Name: fyne.KeyDown}))
	require.Same(t, f.btns[3], f.focused())
	assert.True(t, f.n.HandleKey(&fyne.KeyEvent{Name: fyne.KeyReturn}))
	assert.Equal(t, []string{"3"}, f.tapped)
	assert.False(t, f.n.HandleKey(&fyne.KeyEvent{Name: fyne.KeyA}))
}

// In a dialog the focus stays inside it, goes round at the end, and B closes
// it the way the dialog said.
func TestNavigatorDialog(t *testing.T) {
	f := newNavWindow(t)
	f.n.present = true
	closed := 0
	var d Hider
	cancel := widget.NewButton("Cancel", func() { closed++; d.Hide() })
	del := DangerButton("Delete", func() {})
	d = ShowCustom(f.w, "Delete?", HBox(cancel, del), 300)
	OnBack(f.w, func() { closed++; d.Hide() })
	FocusSafe(f.w, cancel)
	require.Same(t, cancel, f.focused(), "the safe button has the focus")

	f.n.Apply(PadRight)
	require.Same(t, del, f.focused())
	f.n.Apply(PadRight)
	require.Same(t, cancel, f.focused(), "goes round inside the dialog")
	f.n.Apply(PadDown)
	assert.Same(t, del, f.focused(), "nothing below: on to the next one, never the page behind")

	f.n.Apply(PadTabNext)
	assert.Empty(t, f.section, "LB/RB don't switch the page behind a dialog")
	f.n.Apply(PadBack)
	assert.Equal(t, 1, closed)
	assert.Empty(t, f.w.Canvas().Overlays().List())
	assert.Zero(t, f.backs, "B closed the dialog, not the page")
}

// A dialog that didn't say what B does is closed.
func TestNavigatorClosesOtherDialogs(t *testing.T) {
	f := newNavWindow(t)
	pop := widget.NewModalPopUp(widget.NewButton("x", nil), f.w.Canvas())
	pop.Show()
	f.n.Apply(PadBack)
	assert.False(t, pop.Visible())
	assert.Zero(t, f.backs)
}

func TestNavigatorTappable(t *testing.T) {
	test.NewApp()
	n := 0
	tap := NewTappable(widget.NewLabel("Game"), func() { n++ })
	w := test.NewWindow(container.NewVBox(tap, widget.NewButton("Play", nil)))
	w.Resize(fyne.NewSize(300, 300))
	nav := NewNavigator(w, NavigatorOptions{})
	nav.Apply(PadDown)
	require.Same(t, tap, w.Canvas().Focused(), "a Tappable takes the focus")
	nav.Apply(PadAccept)
	assert.Equal(t, 1, n)
	tap.TypedKey(&fyne.KeyEvent{Name: fyne.KeyReturn})
	assert.Equal(t, 2, n, "Enter presses it")
	tap.TypedKey(&fyne.KeyEvent{Name: fyne.KeyDown})
	assert.IsType(t, &widget.Button{}, w.Canvas().Focused(), "an arrow key moves on")
}

// The page scrolls to keep the focused control in view.
func TestNavigatorScrollsToTheFocus(t *testing.T) {
	test.NewApp()
	list := VBox()
	var btns []*widget.Button
	for i := range 30 {
		b := widget.NewButton(fmt.Sprint(i), nil)
		btns = append(btns, b)
		list.Add(b)
	}
	scroll := container.NewVScroll(list)
	w := test.NewWindow(scroll)
	w.Resize(fyne.NewSize(300, 300))
	nav := NewNavigator(w, NavigatorOptions{})
	nav.Apply(PadDown)
	for range 20 {
		nav.Apply(PadDown)
	}
	require.Same(t, btns[20], w.Canvas().Focused())
	assert.Greater(t, scroll.Offset.Y, float32(0), "scrolled down")
	top := btns[20].Position().Y + list.Position().Y // where it is in the view
	assert.GreaterOrEqual(t, top, float32(0))
	assert.LessOrEqual(t, top+btns[20].Size().Height, scroll.Size().Height, "the focused button shows")

	nav.scrollBy(-1000)
	assert.Zero(t, scroll.Offset.Y, "the right stick scrolls")
}

// A on a text field asks for a keyboard (in Game Mode: Steam's); D-pad moves
// out of it.
func TestNavigatorTextField(t *testing.T) {
	test.NewApp()
	entry := widget.NewEntry()
	btn := widget.NewButton("Go", nil)
	w := test.NewWindow(container.NewVBox(entry, btn))
	w.Resize(fyne.NewSize(300, 200))
	var typed fyne.Focusable
	nav := NewNavigator(w, NavigatorOptions{TextField: func(f fyne.Focusable) { typed = f }})
	nav.Apply(PadDown)
	require.Same(t, entry, w.Canvas().Focused())
	nav.Apply(PadAccept)
	assert.Same(t, entry, typed)
	typed = nil
	nav.HandleKey(&fyne.KeyEvent{Name: fyne.KeyReturn})
	assert.Nil(t, typed, "Enter from a keyboard is the entry's own")
	nav.Apply(PadDown)
	assert.Same(t, btn, w.Canvas().Focused())
}

// A page that redraws itself replaces the focused button; the focus goes to
// the new one in its place.
func TestNavigatorKeepsFocusAcrossRedraws(t *testing.T) {
	f := newNavWindow(t)
	f.n.Apply(PadDown)
	f.n.Apply(PadRight)
	require.Same(t, f.btns[1], f.focused())

	parent := f.w.Content().(*fyne.Container).Objects[0].(*fyne.Container)
	fresh := widget.NewButton("1 again", nil)
	parent.Objects[1] = fresh
	parent.Refresh()
	f.n.keepFocus()
	assert.Same(t, fresh, f.focused(), "the redrawn button has the focus")

	parent.Objects[1] = widget.NewButton("1 once more", nil)
	parent.Refresh()
	f.n.Apply(PadRight)
	assert.Same(t, f.btns[2], f.focused(), "a press after a redraw carries on from the same place")
}

// An open Select's menu takes the D-pad itself; A picks, B closes it.
func TestNavigatorSelect(t *testing.T) {
	test.NewApp()
	sel := widget.NewSelect([]string{"One", "Two", "Three"}, nil)
	sel.SetSelected("One")
	w := test.NewWindow(container.NewVBox(sel))
	w.Resize(fyne.NewSize(300, 300))
	nav := NewNavigator(w, NavigatorOptions{})
	nav.Apply(PadDown)
	require.Same(t, sel, w.Canvas().Focused())
	nav.Apply(PadAccept)
	require.NotNil(t, w.Canvas().Overlays().Top(), "A opens the menu")
	nav.Apply(PadDown)
	nav.Apply(PadDown)
	nav.Apply(PadAccept)
	assert.Equal(t, "Two", sel.Selected)
	assert.Nil(t, w.Canvas().Overlays().Top(), "picking closes the menu")

	nav.Apply(PadAccept)
	require.NotNil(t, w.Canvas().Overlays().Top())
	nav.Apply(PadBack)
	assert.Nil(t, w.Canvas().Overlays().Top(), "B closes the menu")
	assert.Equal(t, "Two", sel.Selected)
}

// A prompt's Nav in a window with a Navigator: the Navigator reads the
// gamepad, the Nav says what B does and hears every press.
func TestNavigatorWithNav(t *testing.T) {
	f := newNavFixture(t)
	f.w.Resize(fyne.NewSize(400, 300))
	nv := NewNavigator(f.w, NavigatorOptions{Back: func() { t.Fatal("the page's back in a dialog") }})
	var state PadState
	nv.pad = func() (PadState, bool) { return state, true }
	inputs := 0
	f.nav.OnInput(func() { inputs++ })
	f.nav.Start(f.btns[0])
	require.Nil(t, f.nav.stopPoll, "the Nav doesn't poll the gamepad itself")
	now := time.Unix(1000, 0)
	step := func(s PadState) {
		state = s
		now = now.Add(time.Second)
		nv.poll(now)
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
	assert.Equal(t, 3, inputs)

	f.nav.Stop()
	assert.Empty(t, nv.scopes)
}

// A presses form controls the way the space bar does: it ticks a check and
// picks a radio item.
func TestNavigatorFormControls(t *testing.T) {
	test.NewApp()
	check := widget.NewCheck("Sync on exit", nil)
	radio := widget.NewRadioGroup([]string{"Light", "Dark"}, nil)
	w := test.NewWindow(container.NewVBox(check, radio))
	w.Resize(fyne.NewSize(300, 300))
	nav := NewNavigator(w, NavigatorOptions{})
	nav.Apply(PadDown)
	nav.Apply(PadAccept)
	assert.True(t, check.Checked)
	nav.Apply(PadDown)
	nav.Apply(PadDown)
	nav.Apply(PadAccept)
	assert.Equal(t, "Dark", radio.Selected)
}

// LB and RB switch the tabs of the dialog in front; the page's sections stay.
func TestNavigatorSwitchesTabs(t *testing.T) {
	f := newNavWindow(t)
	tabs := container.NewAppTabs(
		container.NewTabItem("Grids", widget.NewButton("a", nil)),
		container.NewTabItem("Heroes", widget.NewButton("b", nil)),
		container.NewTabItem("Logos", widget.NewButton("c", nil)),
	)
	d := ShowCustom(f.w, "Artwork", FixedSize(fyne.NewSize(300, 200), tabs), 400)
	f.n.Apply(PadTabNext)
	assert.Equal(t, 1, tabs.SelectedIndex())
	f.n.Apply(PadTabPrev)
	f.n.Apply(PadTabPrev)
	assert.Equal(t, 2, tabs.SelectedIndex(), "goes round")
	assert.Empty(t, f.section)
	d.Hide()
}

// Up and down stay inside a NavGroup (the sidebar) until its end, even when a
// control beside it is closer.
func TestNavigatorGroup(t *testing.T) {
	test.NewApp()
	top, bottom := widget.NewButton("Library", nil), widget.NewButton("Trash", nil)
	side := NewNavGroup(container.NewVBox(top, layout.NewSpacer(), bottom))
	beside := widget.NewButton("Page", nil)
	page := container.NewVBox(layout.NewSpacer(), beside, layout.NewSpacer())
	w := test.NewWindow(container.NewBorder(nil, nil, FixedWidth(120, side), nil, FixedSize(fyne.NewSize(200, 400), page)))
	w.Resize(fyne.NewSize(400, 400))
	nav := NewNavigator(w, NavigatorOptions{})
	nav.Focus(bottom)
	nav.Apply(PadUp)
	assert.Same(t, top, w.Canvas().Focused(), "stays in the sidebar")
	nav.Apply(PadRight)
	assert.Same(t, beside, w.Canvas().Focused(), "right leaves it")
	nav.Apply(PadLeft)
	assert.Contains(t, []fyne.Focusable{top, bottom}, w.Canvas().Focused(), "left comes back")
}
