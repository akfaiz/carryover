package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/lang"
	"fyne.io/fyne/v2/widget"

	"github.com/akfaiz/carryover/services"
)

// gameTile is a library grid entry: cover, name, sync status and a play
// (or "Set up") button.
type gameTile struct {
	widget.BaseWidget
	content fyne.CanvasObject
}

func newGameTile(u *UI, g services.GameView) *gameTile {
	g.Status = g.Overall // the tile shows the game as a whole: its own saves and all its emulators
	open := func() { u.navigate("/game/" + g.ID) }
	cover := newTappable(coverView(coverSource{path: g.CoverPath, version: g.CoverVersion, name: g.Name}, fyne.NewSize(160, 240)), open)
	name := widget.NewLabelWithStyle(g.Name, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	name.Truncation = fyne.TextTruncateEllipsis

	label := lang.L("▶ Play")
	switch {
	case g.Status.Running:
		label = lang.L("Running")
	case !g.Ready:
		label = lang.L("Set up")
	}
	var play *widget.Button
	play = widget.NewButton(label, func() {
		if !g.Ready {
			u.navigate("/edit/" + g.ID)
			return
		}
		play.Disable()
		// A game that can be played in several ways asks which one in a dialog.
		async0(func() error { return u.sess.Play(g.ID) }, func(err error) {
			if err != nil {
				u.toastErr(err)
				play.Enable()
				return
			}
			open()
		})
	})
	if g.Ready && !g.Status.Running {
		play.Importance = widget.HighImportance
	}
	if g.Status.Running || (g.Ready && g.Device.LaunchMode == "steamwrap") {
		play.Disable()
	}
	when := ""
	if g.Ready {
		when = timeAgo(g.Status.LastSyncAt)
	}
	t := &gameTile{content: vbox(cover, name, hbox(statusBadge(g), small(when)), play)}
	t.ExtendBaseWidget(t)
	return t
}

// CreateRenderer implements fyne.Widget.
func (t *gameTile) CreateRenderer() fyne.WidgetRenderer { return widget.NewSimpleRenderer(t.content) }
