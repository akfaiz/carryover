package kit

import (
	"image/color"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func texts(root fyne.CanvasObject) []string {
	var out []string
	walk(root, func(o fyne.CanvasObject) {
		switch x := o.(type) {
		case *widget.Label:
			out = append(out, x.Text)
		case *canvas.Text:
			out = append(out, x.Text)
		}
	})
	return out
}

func TestLabelsAndHeadings(t *testing.T) {
	test.NewApp()
	assert.Equal(t, theme.SizeNameSubHeadingText, Heading("h").SizeName)
	assert.True(t, Heading("h").TextStyle.Bold)
	assert.Equal(t, theme.SizeNameHeadingText, Title("t").SizeName)
	m := Muted("m")
	assert.Equal(t, widget.LowImportance, m.Importance)
	assert.Equal(t, fyne.TextWrapWord, m.Wrapping)
	c := Colored("c", widget.DangerImportance)
	assert.Equal(t, widget.DangerImportance, c.Importance)
	assert.True(t, Mono("x").TextStyle.Monospace)
	assert.Equal(t, fyne.TextWrapBreak, Mono("x").Wrapping)
	assert.Equal(t, fyne.TextWrapWord, WrapTitle("t").Wrapping)
	assert.Equal(t, theme.SizeNameCaptionText, Small("s").SizeName)
	assert.Equal(t, fyne.TextWrap(0), Small("s").Wrapping, "never wraps")
}

func TestCardAndHeads(t *testing.T) {
	test.NewApp()
	c := Card("Title", widget.NewLabel("body"))
	assert.Equal(t, []string{"Title", "body"}, texts(c))
	assert.Equal(t, []string{"body"}, texts(Card("", widget.NewLabel("body"))), "no title, no heading")

	h := CardHead("Saves", widget.NewButton("Go", nil))
	assert.Contains(t, texts(h), "Saves")
	assert.Len(t, all[*widget.Button](h), 1)

	f := Field("Label", widget.NewEntry())
	assert.Contains(t, texts(f), "Label")
	assert.Len(t, all[*widget.Entry](f), 1)
}

func TestBadgeKindsUseTheirColours(t *testing.T) {
	test.NewApp()
	for kind, want := range map[int]color.NRGBA{BadgeNone: ColMuted, BadgeOK: ColOK, BadgeWarn: ColWarn, BadgeErr: ColErr, BadgeInfo: ColAccent} {
		txt := all[*canvas.Text](Badge("text", kind))
		require.Len(t, txt, 1)
		assert.Equal(t, "text", txt[0].Text)
		assert.Equal(t, want, txt[0].Color, "kind %d", kind)
	}
}

func TestButtonsAndEnabling(t *testing.T) {
	test.NewApp()
	n := 0
	p := Primary("go", func() { n++ })
	assert.Equal(t, widget.HighImportance, p.Importance)
	test.Tap(p)
	assert.Equal(t, 1, n)
	assert.Equal(t, widget.LowImportance, LowButton("l", nil).Importance)
	assert.Equal(t, widget.DangerImportance, DangerButton("d", nil).Importance)

	SetEnabled(p, false)
	assert.True(t, p.Disabled())
	SetEnabled(p, true)
	assert.False(t, p.Disabled())

	o := box(1, 1)
	SetVisible(o, false)
	assert.False(t, o.Visible())
	SetVisible(o, true)
	assert.True(t, o.Visible())
}

// A check's initial state is set before its callback is attached, so building
// one never calls it.
func TestNewCheckDoesNotCallBackWhileBuilding(t *testing.T) {
	test.NewApp()
	calls := 0
	c := NewCheck("opt", true, func(bool) { calls++ })
	assert.True(t, c.Checked)
	assert.Zero(t, calls)
	test.Tap(c)
	assert.Equal(t, 1, calls)
	assert.False(t, c.Checked)
}

func TestChoiceInASelect(t *testing.T) {
	test.NewApp()
	c := &Choice{}
	c.Add("", "None")
	c.Add("x", "Ex")
	sel := SelectOf(c, "x", nil)
	assert.Equal(t, "Ex", sel.Selected)
	assert.Equal(t, []string{"None", "Ex"}, sel.Options)
	assert.NotPanics(t, func() { sel.SetSelected("None") }, "no callback is fine")
}

func TestChipsMarkTheSelectedOne(t *testing.T) {
	test.NewApp()
	var got string
	row := Chips([]ChipOption{
		{Value: "all", Label: "All"},
		{Value: "warn", Label: "Warn", Icon: theme.WarningIcon()},
	}, "warn", func(v string) { got = v })
	bs := all[*widget.Button](row)
	require.Len(t, bs, 2)
	assert.Equal(t, widget.LowImportance, bs[0].Importance)
	assert.Equal(t, widget.HighImportance, bs[1].Importance)
	assert.NotNil(t, bs[1].Icon)
	test.Tap(bs[0])
	assert.Equal(t, "all", got)
}

func TestProgressBarDeterminateAndNot(t *testing.T) {
	test.NewApp()
	p := NewProgress()
	assert.NotNil(t, test.WidgetRenderer(p))

	p.Set("Uploading", 512, 1024)
	assert.Equal(t, 0.5, p.bar.Value)
	assert.True(t, p.bar.Visible())
	assert.False(t, p.inf.Visible())
	assert.Equal(t, "Uploading · 512 B / 1.0 KB", p.info.Text)

	p.Set("", 2048, 1024)
	assert.Equal(t, 1.0, p.bar.Value, "never past the end")
	assert.Equal(t, "2.0 KB / 1.0 KB", p.info.Text)

	p.Set("Working", 0, 0)
	assert.False(t, p.bar.Visible())
	assert.True(t, p.inf.Visible(), "no total: indeterminate")
	assert.Equal(t, "Working", p.info.Text)
	p.Set("", 0, 0)
	assert.False(t, p.info.Visible(), "no text, no label")
}

func TestBusyLine(t *testing.T) {
	test.NewApp()
	b := BusyLine("Loading backups…")
	assert.Equal(t, []string{"Loading backups…"}, texts(b))
	assert.Len(t, all[*widget.ProgressBarInfinite](b), 1)
}

func TestTappableTapsAndHighlightsOnHover(t *testing.T) {
	test.NewApp()
	n := 0
	tp := NewTappable(Small("x"), func() { n++ })
	var _ desktop.Hoverable = tp
	assert.Equal(t, desktop.PointerCursor, tp.Cursor())
	test.Tap(tp)
	assert.Equal(t, 1, n)

	tp.MouseIn(&desktop.MouseEvent{})
	assert.Equal(t, ColHover, tp.hover.FillColor)
	tp.MouseMoved(&desktop.MouseEvent{})
	assert.Equal(t, ColHover, tp.hover.FillColor)
	tp.MouseOut()
	assert.NotEqual(t, ColHover, tp.hover.FillColor)

	assert.NotPanics(t, func() { NewTappable(Small("x"), nil).Tapped(nil) }, "no handler is fine")
}

func TestRows(t *testing.T) {
	test.NewApp()
	tapped := 0
	lr := ListRow("A long game name", "2024", func() { tapped++ })
	assert.ElementsMatch(t, []string{"A long game name", "2024"}, texts(lr))
	assert.Len(t, all[*Tappable](lr), 1)
	test.Tap(all[*Tappable](lr)[0])
	assert.Equal(t, 1, tapped)

	dr := DetailRow("Name", "C:/very/long/path", Badge("ok", BadgeOK), func() { tapped++ })
	assert.Contains(t, texts(dr), "C:/very/long/path")
	assert.Contains(t, texts(dr), "ok")
	assert.NotContains(t, texts(DetailRow("n", "d", nil, nil)), "ok", "no badge")

	mr := MediaRow(box(10, 10), "Game", Badge("new", BadgeInfo), func() { tapped++ })
	assert.Contains(t, texts(mr), "Game")
	assert.Contains(t, texts(mr), "new")
	assert.Contains(t, texts(MediaRow(box(10, 10), "Game", nil, nil)), "Game", "nil trailing object is fine")
}

func TestChoiceTileHighlightsAndTaps(t *testing.T) {
	test.NewApp()
	n := 0
	plain := NewChoiceTile("Eden", theme.ComputerIcon(), false, func() { n++ })
	hi := NewChoiceTile("Ryujinx", theme.ComputerIcon(), true, func() { n++ })
	assert.Equal(t, "Eden", plain.Name)
	assert.False(t, plain.Highlighted)
	assert.True(t, hi.Highlighted)
	assert.Len(t, all[*canvas.Rectangle](hi), len(all[*canvas.Rectangle](plain))+1, "the ring")
	assert.Contains(t, texts(hi), "Ryujinx")
	test.Tap(hi.Btn)
	test.Tap(plain.Btn)
	assert.Equal(t, 2, n)
}
