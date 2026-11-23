package editor

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"

	"github.com/akfaiz/carryover/ui/i18n"

	"github.com/akfaiz/carryover/ui/kit"
)

// renderBar draws the bar under the form: an error or a hint about what is
// still missing, Cancel and Save.
func (e *Page) renderBar() {
	if e.bar == nil || e.root == nil || e.vm.loading {
		return
	}
	saveLabel := i18n.L("Save")
	if e.vm.saving {
		saveLabel = i18n.L("Saving…")
	}
	save := kit.Primary(saveLabel, e.save)
	kit.SetEnabled(save, e.vm.CanSave())
	note := layout.NewSpacer()
	switch e.vm.BarNote() {
	case NoteError:
		note = kit.Colored(e.vm.errText, widget.DangerImportance)
	case NoteName:
		note = kit.Muted(i18n.L("Give the game a name to save it."))
	case NoteEmulator:
		note = kit.Muted(i18n.L("Add the emulator you play it with."))
	case NoteSaveLocation:
		note = kit.Muted(i18n.L("No save location is set for this device yet. You can save anyway; the game shows “needs setup” here until you set one."))
	}
	line := canvas.NewRectangle(kit.ColLine)
	line.SetMinSize(fyne.NewSize(0, 1))
	row := container.NewBorder(nil, nil, nil, kit.HBox(widget.NewButton(i18n.L("Cancel"), e.u.Back), save), note)
	// As wide as the form above it, so Save stays under it on a wide window.
	width := float32(780)
	if e.vm.id != "" {
		width = 1400
	}
	e.bar.Objects = []fyne.CanvasObject{canvas.NewRectangle(kit.ColBg2),
		container.NewBorder(line, nil, nil, nil, container.New(layout.NewCustomPaddedLayout(10, 10, 28, 28), kit.MaxWidth(width, row)))}
	// The bar's height can change (a longer note wraps; it starts empty): lay
	// the page out again so the bar gets the height it needs.
	e.root.Refresh()
}

func (e *Page) save() {
	u := e.u
	e.vm.errText = ""
	in, err := e.vm.Input()
	if err != nil {
		e.vm.errText = i18n.L("Add at least one emulator, or choose “PC or Steam game”.")
		e.renderBar()
		return
	}
	e.vm.saving = true
	e.renderBar()
	coverSrc, gridPick := e.vm.coverSrc, e.vm.gridPick

	artJobs := e.vm.artworkJobs()
	kit.Async(func() (SaveOutcome, error) {
		return e.vm.Submit(in, coverSrc, gridPick, artJobs)
	}, func(out SaveOutcome, err error) {
		g, coverErr, artErr := out.Game, out.CoverErr, out.ArtErr
		e.vm.saving = false
		if err != nil {
			e.vm.errText = err.Error()
			e.renderBar()
			return
		}
		e.vm.saved = true
		if coverErr != nil {
			u.Toast(i18n.L("Cover: {{.Err}}", map[string]any{"Err": coverErr}), kit.ToastErr)
		}
		if artErr != nil {
			u.Toast(i18n.L("Artwork: {{.Err}}", map[string]any{"Err": artErr}), kit.ToastErr)
		}
		u.Store().Refresh()
		u.Toast(i18n.L("Saved"), kit.ToastOK)
		// In place of the editor: Back from the game goes where the editor was
		// opened from, not into the editor again.
		u.Replace("/game/" + g.ID)
	})
}
