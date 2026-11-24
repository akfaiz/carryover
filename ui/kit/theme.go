package kit

import (
	"image/color"

	"github.com/egoist/mygo/ui"
)

func col(c color.NRGBA) ui.Color { return ui.Color{R: c.R, G: c.G, B: c.B, A: c.A} }

// AppTheme is the active palette (SetPalette) as a MyGo theme. Apply it with Context.SetTheme when the
// palette changes.
func AppTheme() *ui.Theme {
	var t *ui.Theme
	accentText := col(ColBg)
	if LightOn {
		t = ui.LightTheme()
		accentText = ui.RGB(0xff, 0xff, 0xff)
	} else {
		t = ui.DarkTheme()
	}
	t.Background = col(ColBg)
	t.Surface = col(ColBg2)
	t.SurfaceHover = col(ColBg3)
	t.SurfacePressed = col(ColLine)
	t.Border = col(ColLine)
	t.Text = col(ColText)
	t.TextMuted = col(ColMuted)
	t.Accent = col(ColAccent)
	t.AccentText = accentText
	t.Danger = col(ColErr)
	t.Warning = col(ColWarn)
	t.Success = col(ColOK)
	t.Focus = col(ColAccent)
	t.Selection = ui.RGBA(ColAccent.R, ColAccent.G, ColAccent.B, 0.33)
	t.Radius = 6
	return t
}
