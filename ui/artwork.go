package ui

import (
	"path/filepath"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/lang"
	"fyne.io/fyne/v2/widget"

	"github.com/akfaiz/carryover/internal/stores/steamgrid"
	"github.com/akfaiz/carryover/services"
)

// artSpec describes one kind of Steam artwork for the UI.
type artSpec struct {
	kind    steamgrid.Kind
	label   string
	hint    string
	empty   string // shown when SteamGridDB has none
	preview fyne.Size
	pick    fyne.Size // thumbnail size in the picker
	contain bool      // show the whole image (logos and icons), not a cropped fill
}

// artSpecs returns the extra artwork kinds (the portrait cover has its own
// section in the editor). Functions, not package vars: text goes through lang.
func artSpecs() []artSpec {
	return []artSpec{
		{steamgrid.KindWide, lang.L("Wide cover"), lang.L("Wide capsule, 460×215. Shown in Steam's recent games and some lists."),
			lang.L("No wide covers for this game yet."), fyne.NewSize(184, 86), fyne.NewSize(184, 86), false},
		{steamgrid.KindHero, lang.L("Background"), lang.L("Library background (hero), 1920×620. Shown at the top of the game's page in Steam."),
			lang.L("No backgrounds for this game yet."), fyne.NewSize(248, 80), fyne.NewSize(248, 80), false},
		{steamgrid.KindLogo, lang.L("Logo"), lang.L("Transparent title logo drawn over the background."),
			lang.L("No logos for this game yet."), fyne.NewSize(160, 80), fyne.NewSize(160, 90), true},
		{steamgrid.KindIcon, lang.L("Icon"), lang.L("Small square icon for the game in Steam's library list."),
			lang.L("No icons for this game yet."), fyne.NewSize(64, 64), fyne.NewSize(80, 80), true},
	}
}

// artSpecFor returns the spec of a kind, including the portrait cover.
func artSpecFor(kind steamgrid.Kind) artSpec {
	for _, s := range artSpecs() {
		if s.kind == kind {
			return s
		}
	}
	return artSpec{kind: steamgrid.KindCover, label: lang.L("Cover"), empty: lang.L("No portrait covers for this game yet."),
		preview: fyne.NewSize(80, 120), pick: fyne.NewSize(120, 180)}
}

// artEdit is the editor's pending state for one kind of artwork.
type artEdit struct {
	path    string // stored file, "" if none
	version int64
	src     string          // local image chosen by the user, copied on save
	pick    *steamgrid.Grid // SteamGridDB image, downloaded on save
	clear   bool            // remove the stored image on save
}

func (a *artEdit) changed() bool { return a.src != "" || a.pick != nil || a.clear }

// has reports whether the kind will have an image after saving.
func (a *artEdit) has() bool {
	if a.clear {
		return false
	}
	return a.src != "" || a.pick != nil || a.path != ""
}

func (e *editor) artEdit(kind steamgrid.Kind) *artEdit {
	if e.art == nil {
		e.art = map[steamgrid.Kind]*artEdit{}
	}
	a := e.art[kind]
	if a == nil {
		a = &artEdit{}
		e.art[kind] = a
	}
	return a
}

// openArtPicker opens the SteamGridDB picker on a kind's tab. The game chosen
// once is remembered, so the next kind's picker opens straight on its images.
func (e *editor) openArtPicker(kind steamgrid.Kind) {
	showArtPicker(e.u, artPickerOpts{
		term: e.name, steamAppID: e.storeAppID, gameID: e.gridGameID, gameName: e.gridGameName, kind: kind,
		onGame: func(id int, name string) { e.gridGameID, e.gridGameName = id, name },
		onPick: func(k steamgrid.Kind, g steamgrid.Grid) {
			if k == steamgrid.KindCover {
				e.gridPick, e.coverSrc = &g, ""
				return
			}
			a := e.artEdit(k)
			a.pick, a.src, a.clear = &g, "", false
		},
		onClosed: e.render,
	})
}

// artworkCard lets the user pick the extra Steam artwork (wide cover,
// background, logo, icon) so the game looks right once it is added to Steam.
func (e *editor) artworkCard() fyne.CanvasObject {
	c := vbox(heading(lang.L("Steam artwork")),
		muted(lang.L("Written to Steam's grid folder when you add the game to Steam (or press Update Steam shortcut), so it has its images ready. The portrait cover is chosen above.")))
	for _, spec := range artSpecs() {
		a := e.artEdit(spec.kind)
		var preview fyne.CanvasObject
		src := coverSource{contain: spec.contain}
		switch {
		case a.clear:
		case a.src != "":
			src.path, src.name = a.src, ""
		case a.pick != nil:
			src.url = a.pick.Thumb
			if src.url == "" {
				src.url = a.pick.URL
			}
		case a.path != "":
			src.path, src.version = a.path, a.version
		}
		preview = coverView(src, spec.preview)

		btns := hbox(widget.NewButton(lang.L("Choose file…"), func() {
			e.pickFile(lang.L("Choose image"), []string{".png", ".jpg", ".jpeg", ".webp"}, func(p string) {
				a.src, a.pick, a.clear = p, nil, false
				e.render()
			})
		}))
		if e.hasGridKey {
			btns.Add(widget.NewButton(lang.L("Find…"), func() {
				e.openArtPicker(spec.kind)
			}))
		}
		if a.has() {
			btns.Add(lowButton(lang.L("Clear"), func() {
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
		c.Add(vbox(widget.NewSeparator(),
			widget.NewLabelWithStyle(spec.label, fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
			preview, btns, muted(status)))
	}
	if !e.hasGridKey {
		c.Add(container.NewBorder(nil, nil, nil, lowButton(lang.L("Add key…"), e.askGridKey),
			muted(lang.L("Add a SteamGridDB API key to search community artwork."))))
	}
	return card("", c)
}

// artworkJobs lists what to change on save, for every kind with a pending change.
func (e *editor) artworkJobs() map[steamgrid.Kind]artEdit {
	jobs := map[steamgrid.Kind]artEdit{}
	for k, a := range e.art {
		if a.changed() {
			jobs[k] = *a
		}
	}
	return jobs
}

// applyArtwork performs jobs on a saved game; it returns the first error but
// keeps going so one bad image doesn't lose the others.
func applyArtwork(lib *services.LibraryService, gameID string, jobs map[steamgrid.Kind]artEdit) error {
	var first error
	note := func(err error) {
		if err != nil && first == nil {
			first = err
		}
	}
	for kind, a := range jobs {
		k := string(kind)
		switch {
		case a.src != "":
			_, err := lib.SetArt(gameID, k, a.src)
			note(err)
		case a.pick != nil:
			_, err := lib.SetArtFromURL(gameID, k, a.pick.URL)
			note(err)
		case a.clear:
			_, err := lib.ClearArt(gameID, k)
			note(err)
		}
	}
	return first
}
