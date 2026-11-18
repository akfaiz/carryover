package library

import (
	"fmt"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/lang"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/akfaiz/carryover/services"
	"github.com/akfaiz/carryover/ui/components"
	"github.com/akfaiz/carryover/ui/env"
	"github.com/akfaiz/carryover/ui/kit"
)

// Tile is a library grid entry: cover, name and sync status (click to
// open the game) and a play (or "Set up") button. The library keeps its tiles
// and updates them in place with set, so a status change or a search doesn't
// rebuild the grid and reload every cover.
type Tile struct {
	kit.Composite
	u        env.Env
	g        services.GameView
	coverKey string
	cover    *fyne.Container
	name     *widget.Label
	status   *fyne.Container
	play     *widget.Button
	open     *kit.Tappable // cover, name and status: opens the game
}

func newTile(u env.Env, g services.GameView) *Tile {
	t := &Tile{u: u, cover: container.NewStack(), status: kit.HBox()}
	t.name = widget.NewLabelWithStyle("", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	t.play = widget.NewButton("", t.playTapped)
	// Long names get a second line rather than a tooltip: there is no hover
	// on a Steam Deck.
	t.open = kit.NewTappable(kit.VBox(t.cover, kit.TwoLines(t.name), t.status), func() { u.Navigate("/game/" + t.g.ID) })
	// One Play per game: tinted, so a full grid isn't a wall of solid blue.
	t.Init(t, kit.VBox(t.open, kit.Tonal(t.play)))
	t.set(g)
	return t
}

// Game is the game the tile shows.
func (t *Tile) Game() services.GameView { return t.g }

// Open is the tile's cover, name and status: tapping it opens the game.
func (t *Tile) Open() *kit.Tappable { return t.open }

// set shows g, reloading the cover only when it changed.
func (t *Tile) set(g services.GameView) {
	g.Status = g.Overall // the tile shows the game as a whole: its own saves and all its emulators
	t.g = g
	if key := fmt.Sprintf("%s?%d|%s", g.CoverPath, g.CoverVersion, g.Name); key != t.coverKey {
		t.coverKey = key
		t.cover.Objects = []fyne.CanvasObject{kit.CoverView(kit.CoverSource{Path: g.CoverPath, Version: g.CoverVersion, Name: g.Name}, fyne.NewSize(160, 240))}
		t.cover.Refresh()
	}
	t.name.SetText(g.Name)
	when := ""
	if g.Ready {
		when = components.TimeAgo(g.Status.LastSyncAt)
	}
	t.status.Objects = []fyne.CanvasObject{components.StatusBadge(g), kit.Small(when)}
	t.status.Refresh()

	label, icon, imp, enabled := lang.L("Play"), theme.MediaPlayIcon(), widget.HighImportance, true
	switch {
	case g.Status.Running:
		label, icon, imp, enabled = lang.L("Running"), nil, widget.MediumImportance, false
	case !g.Ready:
		label, icon, imp = lang.L("Set up"), nil, widget.MediumImportance
	case g.Device.LaunchMode == "steamwrap":
		// Steam starts it (and Carryover through its launch options).
		label, imp, enabled = lang.L("Play in Steam"), widget.MediumImportance, services.SteamRunURL(g) != ""
	}
	t.play.SetText(label)
	t.play.SetIcon(icon)
	t.play.Importance = imp
	kit.SetEnabled(t.play, enabled)
	t.play.Refresh()
}

func (t *Tile) playTapped() {
	u, g := t.u, t.g
	switch {
	case !g.Ready:
		u.Navigate("/edit/" + g.ID)
	case g.Device.LaunchMode == "steamwrap":
		components.OpenSteam(u, g)
	default:
		t.play.Disable()
		// A game that can be played in several ways asks which one in a dialog.
		kit.Async0(func() error { return u.Sess().Play(g.ID) }, func(err error) {
			if err != nil {
				u.ToastErr(err)
				t.play.Enable()
				return
			}
			u.Navigate("/game/" + g.ID)
		})
	}
}
