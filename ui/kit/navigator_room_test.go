package kit

import (
	"testing"

	"github.com/egoist/mygo/ui"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// longPage is a scrolling column of buttons below a header, as a page is.
type longPage struct {
	nav    *Navigator
	scroll ui.ScrollState
	title  ui.Handle // a control above the scrolling area, as a page's filters are
	rows   [30]ui.Handle
	ctx    *ui.Context
}

func (p *longPage) view(c *ui.Context) {
	p.ctx = c
	p.nav.Begin(c)
	p.nav.SetScroll(&p.scroll)
	ui.Column(c).Fill().Children(func() {
		p.nav.Item(&p.title, ui.Button(c.Key("title"), "Title").Margin(20), nil)
		ui.Scroll(c).Grow(1).TrackScroll(&p.scroll).Children(func() {
			ui.Column(c).Gap(10).Padding(20).Children(func() {
				for i := range p.rows {
					p.nav.Item(&p.rows[i], ui.Button(c.Key([2]any{"r", i}), "Row").Height(40), nil)
				}
			})
		})
	})
	p.nav.End(c)
}

// A control the D-pad moves to is not left on the edge of the scrolling area,
// where its ring would be cut off: there is room around it, going down and going
// back up.
func TestPadMoveLeavesRoomAroundTheFocusedControl(t *testing.T) {
	p := &longPage{nav: NewNavigator(NavigatorOptions{Back: func() {}})}
	tt := ui.NewTester(p.view, 400, 300)
	tt.Frame()
	tt.Frame()

	room := func(i int) (above, below float32) {
		box, view := p.rows[i].Bounds(p.ctx), p.scroll.Bounds(p.ctx)
		return box.Y - view.Y, (view.Y + view.H) - (box.Y + box.H)
	}
	step := func(a PadAction) {
		p.nav.Press(a)
		for range 6 {
			tt.Frame()
		}
	}
	step(PadDown) // the first press picks where to start: the title
	step(PadDown) // the first row
	require.True(t, p.rows[0].Focused(p.ctx))
	down := 0
	for i := 1; i < len(p.rows); i++ {
		step(PadDown)
		require.True(t, p.rows[i].Focused(p.ctx), "row %d has the focus", i)
		above, below := room(i)
		assert.GreaterOrEqual(t, above, focusRoom-0.5, "row %d: room above", i)
		assert.GreaterOrEqual(t, below, focusRoom-0.5, "row %d: room below", i)
		down = i
	}
	require.Equal(t, len(p.rows)-1, down)
	for i := len(p.rows) - 2; i >= 0; i-- {
		step(PadUp)
		require.True(t, p.rows[i].Focused(p.ctx), "row %d has the focus", i)
		above, below := room(i)
		assert.GreaterOrEqual(t, above, focusRoom-0.5, "row %d going up: room above", i)
		assert.GreaterOrEqual(t, below, focusRoom-0.5, "row %d going up: room below", i)
	}
}

// A control outside the scrolling area (the filters above a grid) gets no room
// made around it: it is not in the area, so scrolling it would only throw the
// area off, however far the control is from it.
func TestPadMoveToAControlAboveTheAreaDoesNotScrollIt(t *testing.T) {
	p := &longPage{nav: NewNavigator(NavigatorOptions{Back: func() {}})}
	tt := ui.NewTester(p.view, 400, 300)
	tt.Frame()
	tt.Frame()
	p.scroll.Y = 200
	tt.Frame()
	tt.Frame()
	require.InDelta(t, 200, p.scroll.Y, 0.5)

	// What a move up from the first control of the area to the title does.
	p.nav.revealing, p.nav.frames = &p.title, 4
	for range 6 {
		tt.Frame()
	}
	assert.InDelta(t, 200, p.scroll.Y, 0.5, "the area stays where it was")
}

// A control the D-pad moves to in a dialog is not in the page: the page is behind
// the dialog, and its scroll is not for the control, though the dialog is over it.
func TestPadMoveInADialogLeavesThePageAlone(t *testing.T) {
	p, tt := newPage(t)
	var scroll ui.ScrollState
	p.nav.SetScroll(&scroll)
	tt.Frame()
	p.open = true
	tt.Frame()
	tt.Frame()
	press(p, tt, PadRight) // Cancel to Upload, both in the dialog
	require.True(t, tt.Focused("Upload"), "the pad moved in the dialog")
	assert.Nil(t, p.nav.revealing, "no room is made in the page for it")
}
