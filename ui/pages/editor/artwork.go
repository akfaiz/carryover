package editor

import (
	"path/filepath"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/lang"
	"fyne.io/fyne/v2/widget"

	"github.com/akfaiz/carryover/services"
	"github.com/akfaiz/carryover/ui/kit"
)

// artSpec describes one kind of Steam artwork for the UI.
type artSpec struct {
	kind    services.ArtKind
	label   string
	hint    string
	empty   string    // shown when SteamGridDB has none
	preview fyne.Size // shown at this size, which keeps the kind's proportions
	pick    fyne.Size // thumbnail size in the picker
	contain bool      // show the whole image (logos and icons), not a cropped fill
}

// artSpecs returns the extra artwork kinds (the portrait cover has its own
// section in the editor). Functions, not package vars: text goes through lang.
func artSpecs() []artSpec {
	return []artSpec{
		{services.ArtWide, lang.L("Wide cover"), lang.L("Wide capsule, 460×215. Shown in Steam's recent games and some lists."),
			lang.L("No wide covers for this game yet."), fyne.NewSize(230, 107), fyne.NewSize(184, 86), false},
		{services.ArtHero, lang.L("Background"), lang.L("Library background (hero), 1920×620. Shown at the top of the game's page in Steam."),
			lang.L("No backgrounds for this game yet."), fyne.NewSize(310, 100), fyne.NewSize(248, 80), false},
		{services.ArtLogo, lang.L("Logo"), lang.L("Transparent title logo drawn over the background."),
			lang.L("No logos for this game yet."), fyne.NewSize(240, 90), fyne.NewSize(160, 90), true},
		{services.ArtIcon, lang.L("Icon"), lang.L("Small square icon for the game in Steam's library list."),
			lang.L("No icons for this game yet."), fyne.NewSize(80, 80), fyne.NewSize(80, 80), true},
	}
}

// artSpecFor returns the spec of a kind, including the portrait cover.
func artSpecFor(kind services.ArtKind) artSpec {
	for _, s := range artSpecs() {
		if s.kind == kind {
			return s
		}
	}
	return artSpec{kind: services.ArtCover, label: lang.L("Cover"), empty: lang.L("No portrait covers for this game yet."),
		preview: fyne.NewSize(80, 120), pick: fyne.NewSize(120, 180)}
}

// openArtPicker opens the SteamGridDB picker on a kind's tab. The game chosen
// once is remembered, so the next kind's picker opens straight on its images.
func (e *Page) openArtPicker(kind services.ArtKind) {
	showArtPicker(e.u, artPickerOpts{
		be:   e.vm.be,
		term: e.vm.name, steamAppID: e.vm.storeAppID, gameID: e.vm.gridGameID, gameName: e.vm.gridGameName, kind: kind,
		onGame: func(id int, name string) { e.vm.gridGameID, e.vm.gridGameName = id, name },
		onPick: func(k services.ArtKind, g services.ArtGrid) {
			if k == services.ArtCover {
				e.vm.gridPick, e.vm.coverSrc = &g, ""
				return
			}
			a := e.vm.artEdit(k)
			a.pick, a.src, a.clear = &g, "", false
		},
		onClosed: e.render,
	})
}

// artworkCard lets the user pick the extra Steam artwork (wide cover,
// background, logo, icon) so the game looks right once it is added to Steam.
func (e *Page) artworkCard() fyne.CanvasObject {
	c := kit.VBox(kit.Heading(lang.L("Steam artwork")),
		kit.Muted(lang.L("Written to Steam's grid folder when you add the game to Steam (or press Update Steam shortcut), so it has its images ready. The portrait cover is chosen above.")))
	for _, spec := range artSpecs() {
		a := e.vm.artEdit(spec.kind)
		var preview fyne.CanvasObject
		src := kit.CoverSource{Contain: spec.contain}
		switch {
		case a.clear:
		case a.src != "":
			src.Path, src.Name = a.src, ""
		case a.pick != nil:
			src.URL = a.pick.Thumb
			if src.URL == "" {
				src.URL = a.pick.URL
			}
		case a.path != "":
			src.Path, src.Version = a.path, a.version
		}
		// At its own proportions, not stretched across the column: a 460×215
		// cover shown 1100 px wide and 86 px high was a cropped strip.
		preview = container.NewHBox(kit.FixedSize(spec.preview, kit.CoverView(src, spec.preview)))

		btns := kit.HBox(widget.NewButton(lang.L("Choose file…"), func() {
			e.pickFile(lang.L("Choose image"), []string{".png", ".jpg", ".jpeg", ".webp"}, "", func(p string) {
				a.src, a.pick, a.clear = p, nil, false
				e.render()
			})
		}))
		if e.vm.hasGridKey {
			btns.Add(widget.NewButton(lang.L("Find…"), func() {
				e.openArtPicker(spec.kind)
			}))
		}
		if a.has() {
			btns.Add(kit.LowButton(lang.L("Clear"), func() {
				a.src, a.pick, a.clear = "", nil, a.path != ""
				e.render()
			}))
		}
		status := spec.hint
		switch {
		case a.clear:
			status = lang.L("Removed when you save.")
		case a.src != "":
			status = lang.L("{{.File}} — copied when you save.", map[string]any{"File": filepath.Base(a.src)})
		case a.pick != nil:
			status = lang.L("SteamGridDB image — downloaded when you save.")
		}
		c.Add(kit.VBox(widget.NewSeparator(),
			widget.NewLabelWithStyle(spec.label, fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
			preview, btns, kit.Muted(status)))
	}
	if !e.vm.hasGridKey {
		c.Add(container.NewBorder(nil, nil, nil, kit.LowButton(lang.L("Add key…"), e.askGridKey),
			kit.Muted(lang.L("Add a SteamGridDB API key to search community artwork."))))
	}
	return kit.Card("", c)
}
