package kit

import (
	"testing"

	"github.com/egoist/mygo/ui"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// btnPage draws one button of each kind: Save, Remove (danger), a busy one and an icon-only one.
type btnPage struct {
	nav  *Navigator
	runs map[string]int
	busy bool
}

func (p *btnPage) view(c *ui.Context) {
	p.nav.Begin(c)
	p.nav.Keys(c)
	ui.Row(c).Gap(8).Children(func() {
		Btn(c, p.nav, "save", BtnPrimary, IconPlay, "Save", func() { p.runs["save"]++ })
		Btn(c, p.nav, "remove", BtnDanger, IconTrash, "Remove", func() { p.runs["remove"]++ })
		Btn(c, p.nav, "busy", BtnSecondary, nil, "Working", func() { p.runs["busy"]++ }, BtnBusy(p.busy))
		IconBtn(c, p.nav, "more", BtnGhost, IconMore, "More", func() { p.runs["more"]++ })
	})
	p.nav.End(c)
}

func newBtnPage() (*btnPage, *ui.Tester) {
	p := &btnPage{nav: NewNavigator(NavigatorOptions{Back: func() {}}), runs: map[string]int{}}
	return p, ui.NewTester(p.view, 640, 200)
}

func TestBtnRunsActionOnceOnClick(t *testing.T) {
	p, tt := newBtnPage()
	tt.Frame()
	require.NoError(t, tt.Click("Save"))
	tt.Frame()
	assert.Equal(t, 1, p.runs["save"])
	require.NoError(t, tt.Click("More"))
	tt.Frame()
	assert.Equal(t, 1, p.runs["more"], "an icon button is found by its label")
}

func TestBtnRunsActionOnPadAccept(t *testing.T) {
	p, tt := newBtnPage()
	tt.Frame()
	press2 := func(a PadAction) {
		p.nav.Press(a)
		tt.Frame()
		tt.Frame()
	}
	press2(PadDown)
	assert.True(t, tt.Focused("Save"), "the first press picks the first button")
	press2(PadAccept)
	assert.Equal(t, 1, p.runs["save"], "A runs the button's action")
	press2(PadRight)
	press2(PadAccept)
	assert.Equal(t, 1, p.runs["remove"])
}

func TestBusyBtnIgnoresClicks(t *testing.T) {
	p, tt := newBtnPage()
	p.busy = true
	tt.Frame()
	require.NoError(t, tt.Click("Working"))
	tt.Frame()
	assert.Zero(t, p.runs["busy"])
}
