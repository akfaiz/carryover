package ui

import (
	"fmt"
	"net/url"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/lang"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/akfaiz/carryover/services"
)

// gameTile is a library grid entry: cover, name and sync status (click to
// open the game) and a play (or "Set up") button. The library keeps its tiles
// and updates them in place with set, so a status change or a search doesn't
// rebuild the grid and reload every cover.
type gameTile struct {
	widget.BaseWidget
	u        *UI
	g        services.GameView
	coverKey string
	cover    *fyne.Container
	name     *widget.Label
	status   *fyne.Container
	play     *widget.Button
	content  fyne.CanvasObject
}

func newGameTile(u *UI, g services.GameView) *gameTile {
	t := &gameTile{u: u, cover: container.NewStack(), status: hbox()}
	t.name = widget.NewLabelWithStyle("", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	t.name.Truncation = fyne.TextTruncateEllipsis
	t.play = widget.NewButton("", t.playTapped)
	open := newTappable(vbox(t.cover, t.name, t.status), func() { u.navigate("/game/" + t.g.ID) })
	t.content = vbox(open, t.play)
	t.set(g)
	t.ExtendBaseWidget(t)
	return t
}

// set shows g, reloading the cover only when it changed.
func (t *gameTile) set(g services.GameView) {
	g.Status = g.Overall // the tile shows the game as a whole: its own saves and all its emulators
	t.g = g
	if key := fmt.Sprintf("%s?%d|%s", g.CoverPath, g.CoverVersion, g.Name); key != t.coverKey {
		t.coverKey = key
		t.cover.Objects = []fyne.CanvasObject{coverView(coverSource{path: g.CoverPath, version: g.CoverVersion, name: g.Name}, fyne.NewSize(160, 240))}
		t.cover.Refresh()
	}
	t.name.SetText(g.Name)
	when := ""
	if g.Ready {
		when = timeAgo(g.Status.LastSyncAt)
	}
	t.status.Objects = []fyne.CanvasObject{statusBadge(g), small(when)}
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
	setEnabled(t.play, enabled)
	t.play.Refresh()
}

func (t *gameTile) playTapped() {
	u, g := t.u, t.g
	switch {
	case !g.Ready:
		u.navigate("/edit/" + g.ID)
	case g.Device.LaunchMode == "steamwrap":
		openSteam(u, g)
	default:
		t.play.Disable()
		// A game that can be played in several ways asks which one in a dialog.
		async0(func() error { return u.sess.Play(g.ID) }, func(err error) {
			if err != nil {
				u.toastErr(err)
				t.play.Enable()
				return
			}
			u.navigate("/game/" + g.ID)
		})
	}
}

// openSteam asks Steam to start a game that Steam launches on this device.
func openSteam(u *UI, g services.GameView) {
	link := services.SteamRunURL(g)
	if link == "" {
		u.toast(lang.L("Steam can't start this game yet: add it to Steam on the game's page."), toastErr)
		return
	}
	pu, err := url.Parse(link)
	if err == nil {
		err = u.app.OpenURL(pu)
	}
	if err != nil {
		u.toastErr(err)
		return
	}
	u.toast(lang.L("Starting {{.Game}} in Steam…", map[string]any{"Game": g.Name}), toastInfo)
}

// CreateRenderer implements fyne.Widget.
func (t *gameTile) CreateRenderer() fyne.WidgetRenderer { return widget.NewSimpleRenderer(t.content) }
