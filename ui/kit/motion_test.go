package kit

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The Tester's clock stands still until Advance moves it, so these tests step
// through a motion; the rest of the suite sees where each one starts or ends.

// at is where the text s shows.
func at(t *testing.T, tt *ui.Tester, s string) ui.Rect {
	t.Helper()
	r, ok := tt.Find(s)
	require.True(t, ok, "no %q in %v", s, tt.Texts())
	return r
}

func TestToastSlidesInAndCountsDownWithoutBuildingTheView(t *testing.T) {
	builds, add := 0, false
	tt := ui.NewTester(func(c *ui.Context) {
		builds++
		if add {
			add = false
			c.AddToast(Toast("saved", ToastOK))
		}
		Toaster(c, 0, "Close")
	}, 640, 480)
	add = true // into a window already showing, as in the app
	tt.Frame()

	start := at(t, tt, "saved").X
	tt.Advance(time.Second)
	rest := at(t, tt, "saved").X
	assert.Greater(t, start, rest, "it comes in from the right")

	before := builds
	tt.Advance(2 * time.Second)
	assert.LessOrEqual(t, builds-before, 1, "the countdown only paints")

	tt.Advance(toastTime)
	assert.False(t, tt.HasText("saved"), "it goes once its time is over")
}

// reflowList is a card of rows with something above them that grows for a
// reason of its own.
type reflowList struct {
	reflow Reflow
	rows   []string
	above  float32
}

func (l *reflowList) view(c *ui.Context) {
	l.reflow.Watch(c, len(l.rows))
	l.reflow.Box(Card(c.Key("list"), "", func() {
		ui.Box(c).Height(l.above)
		for i, r := range l.rows {
			l.reflow.Row(ListRow(c, r, i == 0, r, nil, nil))
		}
	}))
}

func TestReflowSlidesRowsOnlyWhenTheRowsChange(t *testing.T) {
	l := &reflowList{rows: []string{"one", "two", "three"}}
	tt := ui.NewTester(l.view, 400, 600)
	first, third := at(t, tt, "one").Y, at(t, tt, "three").Y

	l.above = 40
	tt.Frame()
	assert.InDelta(t, third+40, at(t, tt, "three").Y, 1, "another change of layout is followed at once")

	l.above = 0
	l.rows = []string{"two", "three"}
	tt.Frame()
	assert.Greater(t, at(t, tt, "two").Y, first+1, "the row below the one removed is still on its way")
	tt.Advance(time.Second)
	assert.InDelta(t, first, at(t, tt, "two").Y, 1)
	assert.False(t, tt.HasText("one"))
}

func TestRevealTakesNoRoomWhenClosedAndGrowsOpen(t *testing.T) {
	open := false
	tt := ui.NewTester(func(c *ui.Context) {
		ui.Column(c).Children(func() {
			ui.Text(c, "above")
			Reveal(c, "r", open, func() {
				ui.Column(c).Height(100).Children(func() { ui.Text(c, "details") })
			})
			ui.Text(c, "below")
		})
	}, 400, 400)
	closed := at(t, tt, "below").Y
	assert.False(t, tt.HasText("details"))

	open = true
	tt.Frame()
	tt.Advance(motionBase / 2)
	half := at(t, tt, "below").Y
	assert.Greater(t, half, closed)
	assert.Less(t, half, closed+100)
	tt.Advance(time.Second)
	assert.InDelta(t, closed+100, at(t, tt, "below").Y, 1)

	open = false
	tt.Frame()
	tt.Advance(time.Second)
	assert.InDelta(t, closed, at(t, tt, "below").Y, 1)
	assert.False(t, tt.HasText("details"))
}

// What is on its way out is painted from a view built earlier: no Draw may ask
// an element of it anything (see Hovered). The Tester panics if one does.
func TestEverythingWithAHoverLookCanLeave(t *testing.T) {
	var dialogs Dialogs
	var choice Choice
	choice.Add("a", "Alpha")
	nav := NewNavigator(NavigatorOptions{})
	shown, open := true, false
	tt := ui.NewTester(func(c *ui.Context) {
		nav.Begin(c)
		if shown {
			ui.Column(c.Key("leaving")).Transition(ui.ElementTransition{Exit: &ui.Motion{}}).Children(func() {
				Btn(c, nav, "plain", BtnSecondary, IconX, "Button", nil)
				IconBtn(c, nav, "icon", BtnGhost, IconX, "Icon", nil)
				NavItem(c, nav, nav.Handle("nav"), 1, IconX, "Entry", true, 0, func() {})
				NavItem(c, nav, nav.Handle("busy"), 2, nil, "Busy", false, 0, func() {})
				Chips(c, nav, "chips", []ChipOption{{Value: "x", Label: "Chip", Icon: IconX}}, "", func(string) {})
				ListButton(c, nav, "list", "Item", func() {}, func() { ui.Text(c, "Item") })
				Disclosure(c, nav, "more", "Advanced", &open, func() {})
				SelectOf(c, nav, "select", "Pick", &choice, "a", nil)
			})
		}
		dialogs.View(c, nav)
		nav.End(c)
	}, 640, 600)

	shown = false
	tt.Frame()
	tt.Advance(motionBase / 3)
	tt.Frame()
	tt.Advance(time.Second)
	assert.False(t, tt.HasText("Button"))

	// A dialog with buttons, then a list dropped from a select.
	d := dialogs.Show(&Dialog{Body: func(c *ui.Context, nav *Navigator) {
		Btn(c, nav, "ok", BtnPrimary, nil, "OK", nil)
		SelectOf(c, nav, "in-dialog", "Pick", &choice, "a", nil)
	}})
	tt.Frame()
	tt.Advance(time.Second)
	require.NoError(t, tt.Click("Pick"))
	tt.Advance(time.Second)
	tt.Key(0, ui.KeyEscape) // the list goes
	tt.Advance(motionFast / 3)
	tt.Frame()
	tt.Advance(time.Second)
	d.Close()
	tt.Frame()
	tt.Advance(motionFast / 3)
	tt.Frame()
	tt.Advance(time.Second)
	assert.False(t, tt.HasText("OK"))
}

// Dialogs and lists are keyed: by place alone, one closing under another would
// hand it its state, and the Tester panics on two elements with one key.
func TestStackedDialogsCloseInAnyOrder(t *testing.T) {
	var dialogs Dialogs
	nav := NewNavigator(NavigatorOptions{})
	body := func(text string) *Dialog {
		return &Dialog{Body: func(c *ui.Context, _ *Navigator) { ui.Text(c, text) }}
	}
	tt := ui.NewTester(func(c *ui.Context) {
		nav.Begin(c)
		dialogs.View(c, nav)
		nav.End(c)
	}, 800, 600)

	lower := dialogs.Show(body("lower"))
	tt.Frame()
	start := at(t, tt, "lower")
	tt.Advance(time.Second)
	tt.Frame() // a pose moves in frames that only paint: build to read where it got
	assert.NotEqual(t, start, at(t, tt, "lower"), "the panel grows into place")

	upper := dialogs.Show(body("upper"))
	tt.Frame()
	tt.Advance(time.Second)
	tt.Frame()
	rest := at(t, tt, "upper")

	lower.Close()
	tt.Frame()
	assert.Equal(t, rest, at(t, tt, "upper"), "the one left stays where it is")
	tt.Advance(time.Second)
	assert.False(t, tt.HasText("lower"))

	upper.Close()
	tt.Frame()
	tt.Advance(time.Second)
	assert.False(t, tt.HasText("upper"))
}

func TestProgressTween(t *testing.T) {
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	tw := progressTween{}
	tw.head(0.5, start, false)

	v, moving := tw.at(start)
	assert.Zero(t, v)
	assert.True(t, moving)
	v, _ = tw.at(start.Add(progressStep / 2))
	assert.Greater(t, v, float32(0))
	assert.Less(t, v, float32(0.5))
	v, moving = tw.at(start.Add(progressStep))
	assert.InDelta(t, 0.5, v, 1e-6)
	assert.False(t, moving)

	// Another transfer starting is not a bar running backward.
	tw.head(0.1, start.Add(time.Second), false)
	v, moving = tw.at(start.Add(time.Second))
	assert.InDelta(t, 0.1, v, 1e-6)
	assert.False(t, moving)

	tw.head(0.9, start.Add(2*time.Second), true)
	v, moving = tw.at(start.Add(2 * time.Second))
	assert.InDelta(t, 0.9, v, 1e-6)
	assert.False(t, moving)
}
