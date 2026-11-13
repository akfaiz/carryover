package ui

import (
	"image/color"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/lang"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/akfaiz/carryover/internal/core"
	"github.com/akfaiz/carryover/services"
)

// choiceTileSize is the side of the square tile of one way to play, and
// choicesPerRow how many fit in a row of the dialog. The rows are laid out by
// hand: a wrapping grid works out its height from the width it had when it was
// first laid out, which leaves a tall gap under the tiles in a dialog.
const (
	choiceTileSize = 124
	choicesPerRow  = 4
)

// choiceTile is a square button for one way to play a game: an icon over its
// name, with a ring around the highlighted one. A real button sits underneath,
// so the tile takes focus and taps like any other button; the icon and the name
// are drawn over it and let taps through.
type choiceTile struct {
	widget.BaseWidget
	name        string
	highlighted bool
	btn         *widget.Button
	body        fyne.CanvasObject
}

func newChoiceTile(name string, icon fyne.Resource, highlighted bool, tapped func()) *choiceTile {
	btn := widget.NewButton("", tapped)
	img := canvas.NewImageFromResource(icon)
	img.FillMode = canvas.ImageFillContain
	img.SetMinSize(fyne.NewSize(40, 40))
	label := widget.NewLabelWithStyle(name, fyne.TextAlignCenter, fyne.TextStyle{Bold: highlighted})
	label.Wrapping = fyne.TextWrapWord
	layers := []fyne.CanvasObject{
		btn,
		container.NewPadded(container.NewVBox(layout.NewSpacer(), container.NewCenter(img), label, layout.NewSpacer())),
	}
	if highlighted {
		ring := canvas.NewRectangle(color.Transparent)
		ring.StrokeColor, ring.StrokeWidth = colAccent, 2
		ring.CornerRadius = theme.InputRadiusSize()
		layers = append(layers, ring)
	}
	t := &choiceTile{name: name, highlighted: highlighted, btn: btn, body: container.NewStack(layers...)}
	t.ExtendBaseWidget(t)
	return t
}

// CreateRenderer implements fyne.Widget.
func (t *choiceTile) CreateRenderer() fyne.WidgetRenderer { return widget.NewSimpleRenderer(t.body) }

// emulatorChoice is the body of the "which emulator?" prompt, used by the
// launcher's Play and by a Steam shortcut: a square tile per way to play the
// game, the default highlighted, and (for a Steam shortcut) a countdown after
// which the default starts. Clicking any choice answers at once; "Stop
// countdown" keeps the dialog open until a choice is made. answer gets a
// choice's ID or "cancel".
func (q *promptQueue) emulatorChoice(p services.Prompt, answer func(string)) fyne.CanvasObject {
	stop := make(chan struct{})
	finish := func(a string) {
		select {
		case <-stop:
		default:
			close(stop)
		}
		answer(a)
	}
	choiceName := func(c services.ProfileChoice) string {
		if c.ID == core.NativeProfile {
			return lang.L("Without an emulator")
		}
		return c.Name
	}

	var defName string
	tiles := vbox()
	var row *fyne.Container
	for i, c := range p.Choices {
		icon := theme.ComputerIcon()
		if c.ID == core.NativeProfile {
			icon = theme.MediaPlayIcon() // the game itself, no emulator
		}
		isDefault := c.ID == p.Default
		if isDefault {
			defName = choiceName(c)
		}
		if i%choicesPerRow == 0 {
			row = hbox()
			tiles.Add(row)
		}
		tile := newChoiceTile(choiceName(c), icon, isDefault, func() { finish(c.ID) })
		row.Add(container.New(layout.NewGridWrapLayout(fyne.NewSize(choiceTileSize, choiceTileSize)), tile))
	}

	remaining, paused := p.Seconds, false
	countdown := muted("")
	update := func() {
		if paused || p.Seconds <= 0 || defName == "" {
			countdown.SetText("")
			return
		}
		countdown.SetText(lang.N("Starting with {{.Emulator}} in {{.Count}} second", remaining,
			map[string]any{"Emulator": defName, "Count": remaining}))
	}
	pause := widget.NewButton(lang.L("Stop countdown"), nil)
	pause.OnTapped = func() {
		paused = true
		pause.Hide()
		update()
	}
	if p.Seconds <= 0 {
		pause.Hide()
	}
	update()

	if p.Seconds > 0 {
		go func() {
			t := time.NewTicker(time.Second)
			defer t.Stop()
			for {
				select {
				case <-stop:
					return
				case <-t.C:
					onUI(func() {
						select {
						case <-stop:
							return
						default:
						}
						if len(q.queue) == 0 || q.queue[0].ID != p.ID {
							close(stop) // the prompt was withdrawn
							return
						}
						if paused {
							return
						}
						remaining--
						if remaining <= 0 {
							finish(p.Default)
							return
						}
						update()
					})
				}
			}
		}()
	}

	hint := lang.L("Choose how to play. The highlighted one is the one you played last.")
	if p.Seconds > 0 {
		hint = lang.L("Choose how to play. The highlighted one starts by itself when the countdown ends.")
	}
	return vbox(
		muted(hint),
		tiles,
		countdown,
		hbox(widget.NewButton(lang.L("Cancel"), func() { finish("cancel") }), pause),
	)
}
