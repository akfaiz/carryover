package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/lang"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/akfaiz/carryover/internal/core"
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
	var playRow fyne.CanvasObject = play
	if n := len(g.Emulators); n > 0 && g.Ready && !g.Status.Running && (n > 1 || len(g.Slots) > 0) {
		playRow = container.NewBorder(nil, nil, nil, profileMenuButton(u, g, open), play)
	}
	t := &gameTile{content: vbox(cover, name, hbox(statusBadge(g), small(when)), playRow)}
	t.ExtendBaseWidget(t)
	return t
}

// profileMenuButton is the small button next to Play that lists the ways to
// play a game with emulators: its own launch config and each emulator.
func profileMenuButton(u *UI, g services.GameView, open func()) *widget.Button {
	var btn *widget.Button
	start := func(emulator string) {
		async0(func() error { return u.sess.PlayWith(g.ID, emulator) }, func(err error) {
			if err != nil {
				u.toastErr(err)
				return
			}
			open()
		})
	}
	btn = widget.NewButtonWithIcon("", theme.MenuDropDownIcon(), func() {
		var items []*fyne.MenuItem
		if len(g.Slots) > 0 {
			items = append(items, fyne.NewMenuItem(lang.L("Play without an emulator"), func() { start(core.NativeProfile) }))
		}
		for _, pv := range g.Emulators {
			label := lang.L("Play with {{.Emulator}}", map[string]any{"Emulator": pv.Name})
			it := fyne.NewMenuItem(label, func() { start(pv.EmulatorID) })
			it.Disabled = !pv.Ready
			items = append(items, it)
		}
		m := widget.NewPopUpMenu(fyne.NewMenu("", items...), u.win.Canvas())
		m.ShowAtRelativePosition(fyne.NewPos(0, btn.Size().Height), btn)
	})
	return btn
}

// CreateRenderer implements fyne.Widget.
func (t *gameTile) CreateRenderer() fyne.WidgetRenderer { return widget.NewSimpleRenderer(t.content) }
