package kit

import (
	"testing"

	"fyne.io/fyne/v2/test"
	"github.com/stretchr/testify/assert"
)

func TestFormatBytes(t *testing.T) {
	for in, want := range map[int64]string{0: "0 B", -5: "0 B", 512: "512 B", 1536: "1.5 KB", 5 << 20: "5.0 MB", 3 << 30: "3.0 GB"} {
		assert.Equal(t, want, FormatBytes(in))
	}
	assert.Equal(t, "G", initial("game"))
}

func TestChoiceMapsLabelsToValues(t *testing.T) {
	c := &Choice{}
	c.Add("a", "Alpha")
	c.Add("b", "Beta")
	assert.Equal(t, "Beta", c.Label("b"))
	assert.Equal(t, "b", c.Value("Beta"))
	assert.Empty(t, c.Label("zzz"))
	assert.Empty(t, c.Value("zzz"))

	var got string
	sel := SelectOf(c, "a", func(v string) { got = v })
	assert.Equal(t, "Alpha", sel.Selected)
	sel.SetSelected("Beta")
	assert.Equal(t, "b", got)
}

func TestTappableTaps(t *testing.T) {
	test.NewApp()
	n := 0
	tp := NewTappable(Small("x"), func() { n++ })
	test.Tap(tp)
	assert.Equal(t, 1, n)
}

func TestToasterKeepsFourAndErrorsStay(t *testing.T) {
	test.NewApp()
	var queued []func()
	prev := OnUI
	OnUI = func(f func()) { queued = append(queued, f) }
	t.Cleanup(func() { OnUI = prev })
	tt := NewToaster()
	for range 6 {
		tt.Show("hello", ToastInfo)
	}
	assert.Len(t, tt.Box.Objects, 4, "the oldest are pushed out")
	tt.Show("boom", ToastErr)
	assert.Len(t, tt.Box.Objects, 4)
	assert.Len(t, tt.BGs, 4)
}

func TestPaletteFollowsTheSetting(t *testing.T) {
	t.Cleanup(func() { SetPalette(ThemeDark) })
	SystemDark.Store(true)
	assert.Equal(t, ThemeDark, PaletteFor(ThemeSystem))
	SystemDark.Store(false)
	assert.Equal(t, ThemeLight, PaletteFor(ThemeSystem))
	assert.Equal(t, ThemeDark, PaletteFor(ThemeDark))
	SetPalette(ThemeLight)
	assert.True(t, LightOn)
	light := ColBg
	SetPalette(ThemeDark)
	assert.False(t, LightOn)
	assert.NotEqual(t, light, ColBg)
}
