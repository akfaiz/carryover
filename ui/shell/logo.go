package shell

import (
	"log"

	"github.com/egoist/mygo/ui"

	"github.com/akfaiz/carryover/ui/kit"
)

// themeLogos is the sidebar's logo for each theme, nil where there is none.
type themeLogos struct{ dark, light *ui.SVG }

func parseLogos(dark, light []byte) themeLogos {
	parse := func(data []byte) *ui.SVG {
		if data == nil {
			return nil
		}
		svg, err := ui.ParseSVG(data)
		if err != nil {
			log.Printf("logo: %v", err)
		}
		return svg
	}
	return themeLogos{parse(dark), parse(light)}
}

// current is the logo for the palette that shows.
func (l themeLogos) current() *ui.SVG {
	if kit.LightOn {
		return l.light
	}
	return l.dark
}
