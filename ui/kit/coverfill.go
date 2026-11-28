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
				coverInitial(c, src.Name, TextHero)
			}
			return
		}
		coverImage(c, src, bm).Absolute().Top(0).Bottom(0).Left(0).Right(0)
	})
}

// coverInitial stands in for a cover: the first letter of the name, size tall.
// It is keyed apart from the picture, which then fades in as it arrives.
func coverInitial(c *ui.Context, name string, size float32) {
	ui.Column(c.Key("cover-initial")).Absolute().Top(0).Bottom(0).Left(0).Right(0).Center().
		Transition(placeholderMotion).Children(func() {
		ui.Text(c, initial(name)).FontSize(size).Bold().TextColor(c.Theme().TextMuted)
	})
}

// coverImage is a cover's picture. It fades in when it loads after its box has
// shown (one already in the cache shows with it, at once), and crosses over
// when another takes its place.
func coverImage(c *ui.Context, src CoverSource, bm *ui.Bitmap) ui.Element {
	fit := ui.Cover
	if src.Contain {
		fit = ui.Contain
	}
	return ui.Image(c.Key("cover-image"), bm).Fit(fit).Transition(imageMotion).ContentTransition(ui.ContentTransition{})
}
