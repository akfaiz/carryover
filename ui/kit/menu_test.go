package kit

import (
	"testing"

	"github.com/egoist/mygo/ui"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// menuPage draws a menu button at the end of a row and a button below it.
type menuPage struct {
	nav   *Navigator
	ran   []string
	other int
}

func newMenuPage() (*menuPage, *ui.Tester) {
	p := &menuPage{}
	p.nav = NewNavigator(NavigatorOptions{Back: func() {}, DefaultFocus: func() *ui.Handle { return BtnHandle(p.nav, "more") }})
	return p, ui.NewTester(p.view, 640, 480)
}

func (p *menuPage) view(c *ui.Context) {
	p.nav.Begin(c)
	p.nav.Keys(c)
	run := func(name string) func() { return func() { p.ran = append(p.ran, name) } }
	ui.Column(c).Gap(8).Children(func() {
		ui.Row(c).Justify(ui.End).Children(func() {
			MenuButton(c, p.nav, "more", BtnSecondary, IconMore, "More", []MenuItem{
				{Key: "open", Label: "Open folder", Icon: IconFolderOpen, Action: run("open")},
				{Key: "stuck", Label: "Can't do", Disabled: true, Action: run("stuck")},
				{Key: "delete", Label: "Delete it", Icon: IconTrash, Danger: true, Action: run("delete")},
			})
		})
		Btn(c, p.nav, "other", BtnSecondary, nil, "Other", func() { p.other++ })
	})
	p.nav.End(c)
}

func (p *menuPage) press(tt *ui.Tester, acts ...PadAction) {
	for _, a := range acts {
		p.nav.Press(a)
		tt.Frame()
		tt.Frame()
		tt.Frame()
	}
}

func TestMenuPickByMouse(t *testing.T) {
	p, tt := newMenuPage()
	tt.Frame()
	assert.False(t, tt.HasText("Delete it"), "closed at first")
	require.NoError(t, tt.Click("More"))
	tt.Frame()
	require.True(t, tt.HasText("Delete it"))
	require.NoError(t, tt.Click("Delete it"))
	tt.Frame()
	assert.Equal(t, []string{"delete"}, p.ran)
	assert.False(t, tt.HasText("Open folder"), "picking closes the menu")
}

func TestMenuButtonClosesItsOpenMenu(t *testing.T) {
	p, tt := newMenuPage()
	tt.Frame()
	require.NoError(t, tt.Click("More"))
	tt.Frame()
	require.True(t, tt.HasText("Open folder"))
	require.NoError(t, tt.Click("More"))
	tt.Frame()
	assert.False(t, tt.HasText("Open folder"))
	assert.Empty(t, p.ran)
}

func TestMenuPadRunsAnItemAndSkipsDisabledOnes(t *testing.T) {
	p, tt := newMenuPage()
	tt.Frame()
	p.press(tt, PadDown) // the first press picks the page's default: the button
	require.True(t, tt.Focused("More"))
	p.press(tt, PadAccept)
	require.True(t, tt.Focused("Open folder"), "the menu starts on its first item")
	p.press(tt, PadDown)
	assert.True(t, tt.Focused("Delete it"), "a disabled item is passed over")
	p.press(tt, PadAccept)
	assert.Equal(t, []string{"delete"}, p.ran)
	assert.False(t, tt.HasText("Open folder"))
	assert.True(t, tt.Focused("More"), "the focus goes back to the button")
}

func TestMenuBackClosesAndKeepsThePageOutOfReach(t *testing.T) {
	p, tt := newMenuPage()
	tt.Frame()
	p.press(tt, PadDown, PadAccept, PadDown, PadDown, PadDown)
	assert.False(t, tt.Focused("Other"), "the page behind the menu is out of reach")
	p.press(tt, PadBack)
	assert.False(t, tt.HasText("Open folder"), "B closes the menu")
	assert.True(t, tt.Focused("More"), "the focus goes back to the button")
	assert.Empty(t, p.ran)
	assert.Zero(t, p.other)
}

func TestMenuOpenedByMouseFocusesNothing(t *testing.T) {
	p, tt := newMenuPage()
	tt.Frame()
	require.NoError(t, tt.Click("More"))
	tt.Frame()
	tt.Frame()
	require.True(t, tt.HasText("Open folder"))
	assert.False(t, tt.Focused("Open folder"), "only the pointer lights an item")
	p.press(tt, PadDown)
	assert.True(t, tt.Focused("Open folder"), "the first D-pad or arrow press starts on the first item")
}

func TestMenuOpenedByKeyStartsOnTheFirstItem(t *testing.T) {
	_, tt := newMenuPage()
	tt.Frame()
	tt.Key(0, ui.KeyTab)
	tt.Frame()
	require.True(t, tt.Focused("More"))
	tt.Key(0, ui.KeyEnter)
	tt.Frame()
	tt.Frame()
	tt.Frame()
	assert.True(t, tt.Focused("Open folder"))
}
