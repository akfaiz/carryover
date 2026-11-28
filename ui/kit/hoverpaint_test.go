package kit

import (
	"testing"

	"github.com/egoist/mygo/ui"
	"github.com/stretchr/testify/assert"
)

func TestFitTextIgnoresRoundingInTheBoxWidth(t *testing.T) {
	var whole, short string
	tt := ui.NewTester(func(c *ui.Context) {
		ui.Box(c).Width(100).Height(20).Draw(func(p *ui.Painter, _ ui.Rect) {
			span := ui.Span{Text: "Dark", Weight: 600}
			w, _ := p.MeasureText(0, span)
			whole = fitText(p, span, w-0.001)
			short = fitText(p, span, w-5)
		})
	}, 200, 100)
	tt.Image()

	assert.Equal(t, "Dark", whole)
	assert.NotEqual(t, "Dark", short)
}
