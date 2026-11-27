package kit

import (
	"fmt"
	"testing"

	"github.com/egoist/mygo/ui"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// selectPage draws a select of three options and a button below it.
type selectPage struct {
	nav     *Navigator
	ch      *Choice
	value   string
	changed []string
	other   int
}

func newSelectPage() (*selectPage, *ui.Tester) {
	ch := &Choice{}
	ch.Add("a", "Alpha")
	ch.Add("b", "Beta")
	ch.Add("c", "Gamma")
	p := &selectPage{nav: NewNavigator(NavigatorOptions{Back: func() {}}), ch: ch, value: "b"}
	tt := ui.NewTester(p.view, 640, 480)
	return p, tt
}

func (p *selectPage) view(c *ui.Context) {
	p.nav.Begin(c)
	p.nav.Keys(c)
	ui.Column(c).Gap(8).Children(func() {
		SelectOf(c, p.nav, "pick", "Pick one", p.ch, p.value, func(v string) { p.value = v; p.changed = append(p.changed, v) })
		Btn(c, p.nav, "other", BtnSecondary, nil, "Other", func() { p.other++ })
	})
	p.nav.End(c)
}

func (p *selectPage) press(tt *ui.Tester, acts ...PadAction) {
	for _, a := range acts {
		p.nav.Press(a)
		tt.Frame()
		tt.Frame()
		tt.Frame()
	}
}

func TestSelectOpensWithPadAndPicks(t *testing.T) {
	p, tt := newSelectPage()
	tt.Frame()
	p.press(tt, PadDown) // the first press picks the first control: the field
	assert.True(t, tt.Focused("Beta"), "the field shows the current choice")
	p.press(tt, PadAccept)
	assert.True(t, tt.HasText("Gamma"), "A opens the list")
	assert.True(t, tt.Focused("Beta"), "the list starts on the current choice")
	p.press(tt, PadDown, PadAccept)
	assert.Equal(t, []string{"c"}, p.changed, "D-pad then A picks the next option once")
	assert.False(t, tt.HasText("Alpha"), "picking closes the list")
}

func TestSelectBackClosesWithoutChange(t *testing.T) {
	p, tt := newSelectPage()
	tt.Frame()
	p.press(tt, PadDown, PadAccept)
	require.True(t, tt.HasText("Alpha"))
	p.press(tt, PadUp, PadBack)
	assert.False(t, tt.HasText("Alpha"), "B closes the list")
	assert.Empty(t, p.changed)
}

func TestSelectListHidesThePageFromThePad(t *testing.T) {
	p, tt := newSelectPage()
	tt.Frame()
	p.press(tt, PadDown, PadAccept)
	p.press(tt, PadDown, PadDown, PadDown, PadDown, PadAccept)
	assert.Zero(t, p.other, "the button behind the open list is out of reach")
}

func TestSelectPickByMouse(t *testing.T) {
	p, tt := newSelectPage()
	tt.Frame()
	require.NoError(t, tt.Click("Beta"))
	tt.Frame()
	require.True(t, tt.HasText("Alpha"))
	require.NoError(t, tt.Click("Alpha"))
	tt.Frame()
	assert.Equal(t, []string{"a"}, p.changed)
	assert.False(t, tt.HasText("Gamma"))
}

func TestSelectPickingTheCurrentChoiceChangesNothing(t *testing.T) {
	p, tt := newSelectPage()
	tt.Frame()
	p.press(tt, PadDown, PadAccept, PadAccept)
	assert.Empty(t, p.changed)
}

func TestSelectOpenedByMouseFocusesNothing(t *testing.T) {
	_, tt := newSelectPage()
	tt.Frame()
	require.NoError(t, tt.Click("Beta"))
	tt.Frame()
	tt.Frame()
	require.True(t, tt.HasText("Alpha"))
	for _, option := range []string{"Alpha", "Gamma"} {
		assert.False(t, tt.Focused(option))
	}
}

// A long list the mouse opens still shows the current choice.
func TestSelectOpenedByMouseScrollsToTheCurrentChoice(t *testing.T) {
	p, tt := newSelectPage()
	p.ch = &Choice{}
	for i := range 40 {
		p.ch.Add(fmt.Sprint(i), fmt.Sprint("Option ", i))
	}
	p.value = "35"
	tt.Frame()
	require.NoError(t, tt.Click("Option 35"))
	for range 4 {
		tt.Frame()
	}
	require.NoError(t, tt.Click("Option 34"), "the rows around the current one are in view")
	tt.Frame()
	assert.Equal(t, []string{"34"}, p.changed)
}
