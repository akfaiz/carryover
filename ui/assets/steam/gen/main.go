// Command gen draws the Steam library artwork for Carryover itself (for when
// it is added to Steam as a non-Steam game) into ui/assets/steam:
//
//	cover.png  600x900   portrait capsule (<appid>p.png)
//	wide.png   920x430   wide capsule     (<appid>.png)
//	hero.png   1920x620  library header   (<appid>_hero.png)
//	logo.png   1280x400  logo over hero   (<appid>_logo.png)
//
// The icon is ui/assets/appicon.png. Each image is built as SVG, with the
// wordmark turned into paths from Fyne's bold font, and rasterized with the
// same SVG renderer Fyne uses. Run from the repository root:
//
//	go run ./ui/assets/steam/gen
package main

import (
	"bytes"
	"fmt"
	"image"
	"image/png"
	"log"
	"math"
	"os"
	"path/filepath"
	"strings"

	"fyne.io/fyne/v2/theme"
	"github.com/fyne-io/oksvg"
	"github.com/srwiley/rasterx"
	"golang.org/x/image/font"
	"golang.org/x/image/font/sfnt"
	"golang.org/x/image/math/fixed"
)

const outDir = "ui/assets/steam"

const (
	textColor  = "#f2f4fa"
	mutedColor = "#9aa4bf"
	blue       = "#5b8cff"
	green      = "#3ccf91"
	tagline    = "Your saves, on every device"
)

var bold, regular *sfnt.Font

func main() {
	var err error
	if bold, err = sfnt.Parse(theme.DefaultTextBoldFont().Content()); err != nil {
		log.Fatal(err)
	}
	if regular, err = sfnt.Parse(theme.DefaultTextFont().Content()); err != nil {
		log.Fatal(err)
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		log.Fatal(err)
	}
	for _, a := range []struct {
		name string
		w, h int
		svg  func(w, h float64) string
	}{
		{"cover", 600, 900, cover},
		{"wide", 920, 430, wide},
		{"hero", 1920, 620, hero},
		{"logo", 1280, 400, logo},
	} {
		svg := a.svg(float64(a.w), float64(a.h))
		if err := writePNG(filepath.Join(outDir, a.name+".png"), svg, a.w, a.h); err != nil {
			log.Fatalf("%s: %v", a.name, err)
		}
	}
}

func writePNG(path, svg string, w, h int) error {
	icon, err := oksvg.ReadIconStream(strings.NewReader(svg), oksvg.StrictErrorMode)
	if err != nil {
		return err
	}
	icon.SetTarget(0, 0, float64(w), float64(h))
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	scanner := rasterx.NewScannerGV(w, h, img, img.Bounds())
	icon.Draw(rasterx.NewDasher(w, h, scanner), 1)

	var buf bytes.Buffer
	enc := png.Encoder{CompressionLevel: png.BestCompression}
	if err := enc.Encode(&buf, img); err != nil {
		return err
	}
	return os.WriteFile(path, buf.Bytes(), 0o644)
}

// doc wraps the body in an SVG document with the shared gradients.
func doc(w, h float64, body string) string {
	return fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 %[1]g %[2]g" width="%[1]g" height="%[2]g">
<defs>
  <linearGradient id="bg" x1="0" y1="0" x2="0.35" y2="1">
    <stop offset="0" stop-color="#232838"/>
    <stop offset="1" stop-color="#0e0f14"/>
  </linearGradient>
  <linearGradient id="hop" x1="0" y1="0" x2="1" y2="0">
    <stop offset="0" stop-color="%[3]s"/>
    <stop offset="1" stop-color="%[4]s"/>
  </linearGradient>
  <linearGradient id="play" x1="0" y1="0" x2="1" y2="1">
    <stop offset="0" stop-color="#7aa3ff"/>
    <stop offset="1" stop-color="#3d6de0"/>
  </linearGradient>
  <radialGradient id="glowBlue" cx="0.5" cy="0.5" r="0.5">
    <stop offset="0" stop-color="%[3]s" stop-opacity="0.38"/>
    <stop offset="1" stop-color="%[3]s" stop-opacity="0"/>
  </radialGradient>
  <radialGradient id="glowGreen" cx="0.5" cy="0.5" r="0.5">
    <stop offset="0" stop-color="%[4]s" stop-opacity="0.26"/>
    <stop offset="1" stop-color="%[4]s" stop-opacity="0"/>
  </radialGradient>
  <linearGradient id="shade" x1="0" y1="0" x2="0" y2="1">
    <stop offset="0" stop-color="#0e0f14" stop-opacity="0"/>
    <stop offset="1" stop-color="#0e0f14" stop-opacity="0.85"/>
  </linearGradient>
</defs>
%[5]s
</svg>`, w, h, blue, green, body)
}

func background(w, h float64) string {
	return fmt.Sprintf(`<rect x="0" y="0" width="%g" height="%g" fill="url(#bg)"/>`, w, h)
}

// glow is a soft round light of radius r centred at (cx, cy).
func glow(id string, cx, cy, r float64) string {
	return fmt.Sprintf(`<circle cx="%g" cy="%g" r="%g" fill="url(#%s)"/>`, cx, cy, r, id)
}

// dots is a faint dot grid that fades out towards the bottom.
func dots(w, h, step float64) string {
	var b strings.Builder
	for y := step / 2; y < h; y += step {
		op := 0.10 * (1 - y/h)
		if op < 0.012 {
			continue
		}
		for x := step / 2; x < w; x += step {
			fmt.Fprintf(&b, `<circle cx="%.1f" cy="%.1f" r="1.6" fill="#ffffff" fill-opacity="%.3f"/>`, x, y, op)
		}
	}
	return b.String()
}

// Bounds of the symbol in the app icon's 1024 grid, strokes included.
const markX, markY, markW, markH = 176.0, 282.0, 672.0, 460.0

// mark draws the app icon's symbol (the hop over the play button) without
// its tile, with its top-left corner at (x, y) and height h.
func mark(x, y, h float64, ground bool) string {
	s := h / markH
	g := ""
	if ground {
		g = `<path d="M 186 732 H 810" stroke="#ffffff" stroke-opacity="0.16" stroke-width="20" stroke-linecap="round"/>`
	}
	return fmt.Sprintf(`<g transform="translate(%g %g) scale(%g %g)">
  %s
  <path d="M 238 622 V 572 A 260 260 0 0 1 758 572 V 592" fill="none" stroke="url(#hop)" stroke-width="60" stroke-linecap="round" stroke-linejoin="round"/>
  <polygon points="682,586 834,586 758,698" fill="%s" stroke="%s" stroke-width="28" stroke-linejoin="round"/>
  <polygon points="454,460 454,620 590,540" fill="url(#play)" stroke="url(#play)" stroke-width="40" stroke-linejoin="round"/>
</g>`, x-markX*s, y-markY*s, s, s, g, green, green)
}

func markWidth(h float64) float64 { return markW * h / markH }

// text is a run of text turned into an SVG path.
type text struct {
	d          string
	minX, maxX float64 // ink extent, relative to the pen start
	minY, maxY float64 // ink extent, relative to the baseline (y down)
}

func (t text) width() float64 { return t.maxX - t.minX }

// shape lays out s in font f at the given pixel size, with extra spacing
// (in pixels) between letters.
func shape(f *sfnt.Font, s string, size, tracking float64) text {
	var buf sfnt.Buffer
	ppem := fixed.Int26_6(math.Round(size * 64))
	t := text{minX: math.Inf(1), minY: math.Inf(1), maxX: math.Inf(-1), maxY: math.Inf(-1)}
	var b strings.Builder
	pt := func(p fixed.Point26_6, pen float64) (float64, float64) {
		x, y := pen+float64(p.X)/64, float64(p.Y)/64
		t.minX, t.maxX = math.Min(t.minX, x), math.Max(t.maxX, x)
		t.minY, t.maxY = math.Min(t.minY, y), math.Max(t.maxY, y)
		return x, y
	}
	pen := 0.0
	var prev sfnt.GlyphIndex
	for i, r := range s {
		idx, err := f.GlyphIndex(&buf, r)
		if err != nil {
			log.Fatal(err)
		}
		if i > 0 {
			if k, err := f.Kern(&buf, prev, idx, ppem, font.HintingNone); err == nil {
				pen += float64(k) / 64
			}
		}
		segs, err := f.LoadGlyph(&buf, idx, ppem, nil)
		if err != nil {
			log.Fatal(err)
		}
		for j, sg := range segs {
			switch sg.Op {
			case sfnt.SegmentOpMoveTo:
				if j > 0 {
					b.WriteString("Z")
				}
				x, y := pt(sg.Args[0], pen)
				fmt.Fprintf(&b, "M%.2f %.2f", x, y)
			case sfnt.SegmentOpLineTo:
				x, y := pt(sg.Args[0], pen)
				fmt.Fprintf(&b, "L%.2f %.2f", x, y)
			case sfnt.SegmentOpQuadTo:
				x1, y1 := pt(sg.Args[0], pen)
				x, y := pt(sg.Args[1], pen)
				fmt.Fprintf(&b, "Q%.2f %.2f %.2f %.2f", x1, y1, x, y)
			case sfnt.SegmentOpCubeTo:
				x1, y1 := pt(sg.Args[0], pen)
				x2, y2 := pt(sg.Args[1], pen)
				x, y := pt(sg.Args[2], pen)
				fmt.Fprintf(&b, "C%.2f %.2f %.2f %.2f %.2f %.2f", x1, y1, x2, y2, x, y)
			}
		}
		if len(segs) > 0 {
			b.WriteString("Z")
		}
		adv, err := f.GlyphAdvance(&buf, idx, ppem, font.HintingNone)
		if err != nil {
			log.Fatal(err)
		}
		pen += float64(adv)/64 + tracking
		prev = idx
	}
	t.d = b.String()
	return t
}

// capHeight is the ink height above the baseline (descenders left out).
func (t text) capHeight() float64 { return -t.minY }

// at places the text so the top-left corner of its ink is at (x, y).
func (t text) at(x, y float64, fill string) string {
	return fmt.Sprintf(`<path transform="translate(%g %g)" d="%s" fill="%s"/>`, x-t.minX, y-t.minY, t.d, fill)
}

// fit sizes a text run so its ink is w pixels wide.
func fit(f *sfnt.Font, s string, w, trackingEm float64) text {
	const probe = 100.0
	t := shape(f, s, probe, trackingEm*probe)
	size := probe * w / t.width()
	return shape(f, s, size, trackingEm*size)
}

func wordmark(w float64) text { return fit(bold, "Carryover", w, -0.01) }

// accent is a short blue-to-green bar.
func accent(x, y, w, h float64) string {
	return fmt.Sprintf(`<rect x="%g" y="%g" width="%g" height="%g" rx="%g" ry="%g" fill="url(#hop)"/>`, x, y, w, h, h/2, h/2)
}

func cover(w, h float64) string {
	var b strings.Builder
	b.WriteString(background(w, h))
	b.WriteString(dots(w, h, 30))
	b.WriteString(glow("glowBlue", w/2, 300, 380))
	b.WriteString(glow("glowGreen", w*0.85, h*0.9, 360))

	mh := 270.0
	b.WriteString(mark((w-markWidth(mh))/2, 150, mh, true))

	wm := wordmark(460)
	y := 550.0
	b.WriteString(wm.at((w-wm.width())/2, y, textColor))
	y += wm.capHeight() + 48
	b.WriteString(accent(w/2-48, y, 96, 8))
	y += 8 + 40
	tg := fit(regular, tagline, 380, 0)
	b.WriteString(tg.at((w-tg.width())/2, y, mutedColor))
	return doc(w, h, b.String())
}

func wide(w, h float64) string {
	var b strings.Builder
	b.WriteString(background(w, h))
	b.WriteString(dots(w, h, 28))
	b.WriteString(glow("glowBlue", 210, h/2-20, 300))
	b.WriteString(glow("glowGreen", w*0.9, h, 320))

	mh := 190.0
	mw := markWidth(mh)
	wm := wordmark(440)
	tg := fit(regular, tagline, 380, 0)
	gap := 56.0
	total := mw + gap + wm.width()
	x := (w - total) / 2
	b.WriteString(mark(x, (h-mh)/2-6, mh, true))

	tx := x + mw + gap
	block := wm.capHeight() + 32 + 8 + 28 + tg.capHeight()
	y := (h-block)/2 - 6
	b.WriteString(wm.at(tx, y, textColor))
	y += wm.capHeight() + 32
	b.WriteString(accent(tx+4, y, 80, 8))
	y += 8 + 28
	b.WriteString(tg.at(tx, y, mutedColor))
	return doc(w, h, b.String())
}

// hero shows a PC and a handheld with the save hopping from one to the
// other. Steam lays the logo over the bottom left, so that part stays quiet.
func hero(w, h float64) string {
	var b strings.Builder
	b.WriteString(background(w, h))
	b.WriteString(dots(w, h, 34))
	b.WriteString(glow("glowBlue", 1180, 260, 520))
	b.WriteString(glow("glowGreen", 1640, 420, 420))

	// PC monitor.
	const mx, my, mw, mh = 1010.0, 230.0, 330.0, 205.0
	fmt.Fprintf(&b, `<rect x="%g" y="%g" width="%g" height="%g" rx="18" ry="18" fill="#1b1f2c" stroke="#ffffff" stroke-opacity="0.18" stroke-width="4"/>`, mx, my, mw, mh)
	fmt.Fprintf(&b, `<rect x="%g" y="%g" width="%g" height="%g" rx="8" ry="8" fill="%s" fill-opacity="0.16"/>`, mx+18, my+18, mw-36, mh-36, blue)
	fmt.Fprintf(&b, `<path d="M %g %g V %g" stroke="#ffffff" stroke-opacity="0.18" stroke-width="16"/>`, mx+mw/2, my+mh, my+mh+48)
	fmt.Fprintf(&b, `<path d="M %g %g H %g" stroke="#ffffff" stroke-opacity="0.18" stroke-width="12" stroke-linecap="round"/>`, mx+mw/2-70, my+mh+54, mx+mw/2+70)

	// Handheld.
	const hx, hy, hw, hh = 1500.0, 320.0, 360.0, 150.0
	fmt.Fprintf(&b, `<rect x="%g" y="%g" width="%g" height="%g" rx="56" ry="56" fill="#1b1f2c" stroke="#ffffff" stroke-opacity="0.18" stroke-width="4"/>`, hx, hy, hw, hh)
	fmt.Fprintf(&b, `<rect x="%g" y="%g" width="%g" height="%g" rx="8" ry="8" fill="%s" fill-opacity="0.16"/>`, hx+84, hy+22, hw-168, hh-44, green)
	for _, cx := range []float64{hx + 44, hx + hw - 44} {
		fmt.Fprintf(&b, `<circle cx="%g" cy="%g" r="18" fill="#ffffff" fill-opacity="0.14"/>`, cx, hy+52)
		fmt.Fprintf(&b, `<circle cx="%g" cy="%g" r="9" fill="#ffffff" fill-opacity="0.10"/>`, cx, hy+104)
	}

	// The hop from the monitor's screen to the handheld's.
	sx, sy := mx+mw/2+40, my+10.0
	ex, ey := hx+hw/2, hy-36.0
	fmt.Fprintf(&b, `<path d="M %g %g C %g %g %g %g %g %g" fill="none" stroke="url(#hop)" stroke-width="22" stroke-linecap="round"/>`,
		sx, sy, sx+60, sy-190, ex-20, ey-230, ex, ey-28)
	fmt.Fprintf(&b, `<polygon points="%g,%g %g,%g %g,%g" fill="%s" stroke="%s" stroke-width="12" stroke-linejoin="round"/>`,
		ex-30, ey-34, ex+30, ey-34, ex, ey+10, green, green)
	// Play button riding on the monitor's screen.
	cx, cy := mx+mw/2, my+mh/2
	fmt.Fprintf(&b, `<polygon points="%g,%g %g,%g %g,%g" fill="url(#play)" stroke="url(#play)" stroke-width="14" stroke-linejoin="round"/>`,
		cx-22, cy-34, cx-22, cy+34, cx+36, cy)

	// Darken the bottom for the logo.
	fmt.Fprintf(&b, `<rect x="0" y="%g" width="%g" height="%g" fill="url(#shade)"/>`, h*0.55, w, h*0.45)
	return doc(w, h, b.String())
}

// logo is the mark and the wordmark on a transparent background.
func logo(w, h float64) string {
	var b strings.Builder
	pad := 24.0
	mh := h - 2*pad - 40
	mw := markWidth(mh)
	gap := 64.0
	wm := wordmark(w - 2*pad - mw - gap)
	b.WriteString(mark(pad, (h-mh)/2, mh, false))
	b.WriteString(wm.at(pad+mw+gap, (h-wm.capHeight())/2, "#ffffff"))
	return doc(w, h, b.String())
}
