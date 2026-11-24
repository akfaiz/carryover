package kit

import (
	"testing"

	"github.com/egoist/mygo/ui"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// page is a window with a sidebar, a header button and a 3-column grid, and an
// optional dialog with Cancel (the safe default) and Upload.
type page struct {
	nav        *Navigator
	side       [3]ui.Handle
	header     ui.Handle
	tiles      [6]ui.Handle
	dlg        [2]ui.Handle
	open       bool
	accepted   []string
	dialogBack int
}

func (p *page) view(c *ui.Context) {
	p.nav.Begin(c)
	p.nav.Keys(c)
	ui.Row(c).Fill().Gap(16).AlignItems(ui.Stretch).Children(func() {
		ui.Column(c).Width(120).Gap(4).Children(func() {
			for i, name := range []string{"Library", "Cloud", "Settings"} {
				p.nav.Item(&p.side[i], ui.Button(c.Key([2]any{"s", i}), name), func() { p.accepted = append(p.accepted, name) })
			}
		})
		ui.Column(c).Grow(1).Children(func() {
			el := p.nav.Item(&p.header, ui.Button(c.Key("hdr"), "Open dialog"), func() { p.open = true })
			if el.Clicked() {
				p.open = true
			}
			ui.Row(c).Wrap().Gap(8).Children(func() {
				for i := range p.tiles {
					name := []string{"Game A", "Game B", "Game C", "Game D", "Game E", "Game F"}[i]
					t := p.nav.Item(&p.tiles[i], ui.Button(c.Key([2]any{"t", i}), name).Width(150).Height(60), func() { p.accepted = append(p.accepted, name) })
					_ = t
				}
			})
		})
	})
	p.nav.Overlay("confirm", p.open, func() { p.open = false; p.dialogBack++ }, func() {
		ui.Modal(c, &p.open, func() {
			ui.Row(c).Gap(8).Padding(16).Children(func() {
				p.nav.Item(&p.dlg[0], ui.Button(c.Key("cancel"), "Cancel"), func() { p.open = false }, FocusSafe)
				p.nav.Item(&p.dlg[1], ui.PrimaryButton(c.Key("upload"), "Upload"), func() { p.open = false; p.accepted = append(p.accepted, "upload") })
			})
		})
	})
	p.nav.End(c)
}

func newPage(t *testing.T) (*page, *ui.Tester) {
	t.Helper()
	p := &page{nav: NewNavigator(NavigatorOptions{Back: func() {}})}
	return p, ui.NewTester(p.view, 640, 400)
}

func press(p *page, tt *ui.Tester, acts ...PadAction) {
	for _, a := range acts {
		p.nav.Press(a)
		tt.Frame()
		tt.Frame() // a focus request applies after the build that made it
	}
}

func TestFirstPressPicksTheTopLeft(t *testing.T) {
	p, tt := newPage(t)
	tt.Frame()
	press(p, tt, PadDown)
	assert.True(t, tt.Focused("Library"))
}

func TestDPadWalksSidebarHeaderAndGrid(t *testing.T) {
	p, tt := newPage(t)
	tt.Frame()
	press(p, tt, PadDown, PadDown)
	assert.True(t, tt.Focused("Cloud"), "down in the sidebar")
	press(p, tt, PadRight)
	require.True(t, tt.Focused("Game A"), "right leaves the sidebar for the tile level with it")
	press(p, tt, PadRight)
	assert.True(t, tt.Focused("Game B"), "right in the grid")
	press(p, tt, PadDown)
	assert.True(t, tt.Focused("Game E"), "down in the grid keeps the column")
	press(p, tt, PadAccept)
	assert.Equal(t, []string{"Game E"}, p.accepted, "A runs the focused control's action")
}

func TestOverlayScopeSafeDefaultAndBack(t *testing.T) {
	p, tt := newPage(t)
	tt.Frame()
	p.open = true
	tt.Frame()
	tt.Frame()
	tt.Frame()
	assert.True(t, tt.Focused("Cancel"), "a dialog starts on its safe control")
	press(p, tt, PadRight)
	assert.True(t, tt.Focused("Upload"))
	press(p, tt, PadLeft, PadLeft, PadLeft, PadUp)
	assert.True(t, tt.Focused("Cancel"), "the page behind is out of reach")
	press(p, tt, PadBack)
	assert.False(t, p.open, "B runs the dialog's back")
	assert.Equal(t, 1, p.dialogBack)
}

func TestPageHooks(t *testing.T) {
	var sec []int
	var back, search, sync, view int
	p, tt := newPage(t)
	p.nav.SetOptions(NavigatorOptions{
		Back: func() { back++ }, Search: func() { search++ }, Sync: func() { sync++ }, View: func() { view++ },
		Section: func(d int) { sec = append(sec, d) },
	})
	tt.Frame()
	press(p, tt, PadBack, PadSearch, PadSync, PadView, PadTabPrev, PadTabNext)
	assert.Equal(t, []int{1, 1, 1, 1}, []int{back, search, sync, view})
	assert.Equal(t, []int{-1, 1}, sec)
}

func TestHooksIgnoredUnderADialog(t *testing.T) {
	var search, view int
	p, tt := newPage(t)
	p.nav.SetOptions(NavigatorOptions{Search: func() { search++ }, View: func() { view++ }})
	p.open = true
	tt.Frame()
	tt.Frame()
	press(p, tt, PadSearch, PadView)
	assert.Zero(t, search, "X is for the page")
	assert.Equal(t, 1, view, "View works everywhere")
}

func TestArrowKeysMoveFocus(t *testing.T) {
	p, tt := newPage(t)
	tt.Frame()
	tt.Key(0, ui.KeyDown)
	tt.Frame()
	tt.Frame()
	assert.True(t, tt.Focused("Library"), "the first arrow press picks where to start")
	tt.Key(0, ui.KeyDown)
	tt.Frame()
	tt.Frame()
	assert.True(t, tt.Focused("Cloud"))
	_ = p
}

func TestProgrammaticFocusShowsTheRing(t *testing.T) {
	p, tt := newPage(t)
	tt.Frame()
	press(p, tt, PadDown, PadDown, PadRight)
	require.True(t, tt.Focused("Game A"))
	tt.Frame()
	assert.Same(t, &p.tiles[0], p.nav.ringed, "a pad-moved focus gets a ring MyGo would not draw")
}

func TestPadSourceReachesTheNavigator(t *testing.T) {
	p, tt := newPage(t)
	tt.Frame()
	seq := []PadState{{}, {Down: true}, {}}
	i := 0
	done := make(chan struct{})
	src := func() (PadState, bool) {
		s := seq[min(i, len(seq)-1)]
		if i++; i == len(seq)+2 {
			close(done)
		}
		return s, true
	}
	stop := p.nav.StartPad(src, func(f func()) { f() })
	defer stop()
	<-done
	tt.Frame()
	tt.Frame()
	assert.True(t, p.nav.PadPresent())
	assert.True(t, tt.Focused("Library"))
}

func TestFirstAcceptOnlyPicksTheStart(t *testing.T) {
	p, tt := newPage(t)
	tt.Frame()
	press(p, tt, PadAccept)
	assert.True(t, tt.Focused("Library"), "A with nothing focused focuses the start")
	assert.Empty(t, p.accepted, "and presses nothing")
	press(p, tt, PadAccept)
	assert.Equal(t, []string{"Library"}, p.accepted, "the second A presses")
}

// Both kinds of Confirm draw (a duplicated key panics) and answer.
func TestConfirmDialogs(t *testing.T) {
	for _, danger := range []bool{false, true} {
		var d Confirm
		var nav = NewNavigator(NavigatorOptions{})
		var got []bool
		tt := ui.NewTester(func(c *ui.Context) {
			nav.Begin(c)
			d.View(c, nav)
			nav.End(c)
		}, 600, 400)
		d.Show(ConfirmOptions{Title: "T", Body: "B", Cancel: "No", OK: "Yes", Danger: danger, Check: "More", Checked: "B+",
			OnConfirm: func(checked bool) { got = append(got, checked) }}, nil)
		tt.Frame()
		tt.Frame()
		require.NoError(t, tt.Click("More"))
		tt.Frame()
		assert.True(t, tt.HasText("B+"), "ticking the box changes the text")
		require.NoError(t, tt.Click("Yes"))
		tt.Frame()
		assert.Equal(t, []bool{true}, got, "danger=%v", danger)
		assert.False(t, d.Open())
	}
}
