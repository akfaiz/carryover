package kit

import "github.com/egoist/mygo/ui"

// coverFillSide is the longest side, in pixels, a CoverFill's image is decoded
// at: one size for every width the cover gets, so resizing a window never asks
// the image cache for another copy (which would show the placeholder until it
// loaded).
const coverFillSide = 640

// CoverFill is CoverView for a cover that fills the width of the place it is
// drawn in, with its height from ratio (height over width): in a grid whose
// tiles grow with the window.
func CoverFill(c *ui.Context, src CoverSource, ratio float32) ui.Element {
	t := c.Theme()
	var bm *ui.Bitmap
	switch {
	case src.Path != "":
		bm = Images.File(src.Path, src.Version, coverFillSide)
	case src.URL != "":
		bm = Images.URL(src.URL, coverFillSide)
	}
	box := ui.Box(c).FillWidth().AspectRatio(1/ratio).Radius(8).Background(col(ColBg3)).Border(1, t.Border).Clip()
	return box.Children(func() {
		if bm == nil {
			if src.Name != "" {
				ui.Column(c).Absolute().Top(0).Bottom(0).Left(0).Right(0).Center().Children(func() {
					ui.Text(c, initial(src.Name)).FontSize(TextHero).Bold().TextColor(t.TextMuted)
				})
			}
			return
		}
		fit := ui.Cover
		if src.Contain {
			fit = ui.Contain
		}
		ui.Image(c, bm).Absolute().Top(0).Bottom(0).Left(0).Right(0).Fit(fit)
	})
}
