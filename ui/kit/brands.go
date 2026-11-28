package kit

import (
	_ "embed"

	"github.com/egoist/mygo/ui"
)

// Brand logos, copied unchanged from Simple Icons 16.38.1 (CC0-1.0,
// https://github.com/simple-icons/simple-icons). The logos are trademarks of
// their owners: use one only to name that product. Like the other icons, they
// draw in the current text color.
var (
	//go:embed brands/steam.svg
	steamSVG []byte
	//go:embed brands/heroicgameslauncher.svg
	heroicSVG []byte
	//go:embed brands/lutris.svg
	lutrisSVG []byte
	//go:embed brands/googledrive.svg
	googleDriveSVG []byte
	//go:embed brands/dropbox.svg
	dropboxSVG []byte

	IconSteam       = ui.MustParseSVG(steamSVG)
	IconHeroic      = ui.MustParseSVG(heroicSVG)
	IconLutris      = ui.MustParseSVG(lutrisSVG)
	IconGoogleDrive = ui.MustParseSVG(googleDriveSVG)
	IconDropbox     = ui.MustParseSVG(dropboxSVG)
)
