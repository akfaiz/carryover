package kit

import (
	"testing"

	"github.com/egoist/mygo/ui"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// rowsPage draws a disclosure over a list of two rows.
type rowsPage struct {
	nav    *Navigator
	open   bool
	picked []string
}

func newRowsPage() (*rowsPage, *ui.Tester) {
	p := &rowsPage{nav: NewNavigator(NavigatorOptions{Back: func() {}})}
	return p, ui.NewTester(p.view, 640, 480)
}

func (p *rowsPage) view(c *ui.Context) {
	p.nav.Begin(c)
	p.nav.Keys(c)
	ui.Column(c).Gap(8).Children(func() {
		Disclosure(c, p.nav, "more", "Advanced", &p.open, func() {
			ui.Text(c, "Hidden setting")
		})
		for _, name := range []string{"First", "Second"} {
			ListButton(c, p.nav, name, name, func() { p.picked = append(p.picked, name) }, func() {
				ui.Text(c, name)
			})
		}
	})
	p.nav.End(c)
}

func (p *rowsPage) press(tt *ui.Tester, acts ...PadAction) {
	for _, a := range acts {
		p.nav.Press(a)
		tt.Frame()
		tt.Frame()
		tt.Frame()
	}
}

func TestDisclosureOpensWithPad(t *testing.T) {
	p, tt := newRowsPage()
	tt.Frame()
	p.press(tt, PadDown) // the first press picks the first control: the disclosure
	p.press(tt, PadAccept)
	assert.True(t, p.open, "A opens it")
	assert.True(t, tt.HasText("Hidden setting"))
	p.press(tt, PadAccept)
	assert.False(t, p.open, "A again closes it")
}

func TestDisclosureOpensWithMouse(t *testing.T) {
	p, tt := newRowsPage()
	tt.Frame()
	require.NoError(t, tt.Click("Advanced"))
	tt.Frame()
	assert.True(t, p.open)
}

func TestListButtonPicksWithPadAndMouse(t *testing.T) {
	p, tt := newRowsPage()
	tt.Frame()
	p.press(tt, PadDown, PadDown, PadAccept)
	assert.Equal(t, []string{"First"}, p.picked, "the D-pad reaches the rows and A picks one")
	require.NoError(t, tt.Click("Second"))
	tt.Frame()
	assert.Equal(t, []string{"First", "Second"}, p.picked, "a click picks once")
}
