package kit

// Design tokens: the one place sizes, spacing and radii are chosen. Pages and
// widgets use these names instead of numbers, so every screen shares one scale.

// Type scale (DIPs).
const (
	TextXS    float32 = 11 // badges
	TextSM    float32 = 12 // captions, labels, paths
	TextBase  float32 = 14 // body, buttons, nav
	TextLG    float32 = 16
	TextCard  float32 = 18 // card and dialog titles
	TextTitle float32 = 24 // page titles
	TextHero  float32 = 28 // game title, session window
	// TextDisplay is the game title of the session window in full screen.
	TextDisplay float32 = 44
)

// Spacing scale (DIPs).
const (
	S1 float32 = 4
	S2 float32 = 8
	S3 float32 = 12
	S4 float32 = 16
	S6 float32 = 24
	S8 float32 = 32
)

// Corner radii (DIPs).
const (
	RadiusSM float32 = 6
	Radius   float32 = 8
	RadiusLG float32 = 12
)

// Page content widths (DIPs): full for a grid that fills any window (the
// library), wide for the game page, form for settings-like pages. A page
// narrower than the window is centered in it.
const (
	PageFull float32 = 1 << 16
	PageWide float32 = 1400
	PageForm float32 = 880
)

// Icon sizes (DIPs).
const (
	IconSM float32 = 16
	IconMD float32 = 18
)
