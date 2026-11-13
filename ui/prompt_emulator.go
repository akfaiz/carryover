package ui

import (
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/lang"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/akfaiz/carryover/internal/core"
	"github.com/akfaiz/carryover/internal/ui/kit"
	"github.com/akfaiz/carryover/services"
)

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
	tiles := kit.VBox()
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
		if i%kit.ChoicesPerRow == 0 {
			row = kit.HBox()
			tiles.Add(row)
		}
		tile := kit.NewChoiceTile(choiceName(c), icon, isDefault, func() { finish(c.ID) })
		row.Add(container.New(layout.NewGridWrapLayout(fyne.NewSize(kit.ChoiceTileSize, kit.ChoiceTileSize)), tile))
	}

	remaining, paused := p.Seconds, false
	countdown := kit.Muted("")
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
					kit.OnUI(func() {
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
	return kit.VBox(
		kit.Muted(hint),
		tiles,
		countdown,
		kit.HBox(widget.NewButton(lang.L("Cancel"), func() { finish("cancel") }), pause),
	)
}
