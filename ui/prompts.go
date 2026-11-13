package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/lang"
	"fyne.io/fyne/v2/widget"

	"github.com/akfaiz/carryover/internal/ui/kit"
	"github.com/akfaiz/carryover/services"
)

// promptQueue shows session prompts one at a time as modal dialogs.
type promptQueue struct {
	u       *UI
	queue   []services.Prompt
	current *dialog.CustomDialog
}

func (q *promptQueue) add(p services.Prompt) {
	for _, x := range q.queue {
		if x.ID == p.ID {
			return
		}
	}
	q.queue = append(q.queue, p)
	if q.current == nil {
		q.next()
	}
}

// remove drops a prompt that was withdrawn (its sync was cancelled).
func (q *promptQueue) remove(id string) {
	for i, p := range q.queue {
		if p.ID != id {
			continue
		}
		q.queue = append(q.queue[:i], q.queue[i+1:]...)
		if i == 0 && q.current != nil {
			q.current.Hide()
			q.current = nil
			q.next()
		}
		return
	}
}

func (q *promptQueue) next() {
	if len(q.queue) == 0 || q.u.win == nil {
		q.current = nil
		return
	}
	p := q.queue[0]
	answer := func(a string) {
		q.queue = q.queue[1:]
		if q.current != nil {
			q.current.Hide()
		}
		q.current = nil
		go q.u.sess.Respond(p.ID, a)
		q.next()
	}
	var title string
	var body fyne.CanvasObject
	switch {
	case p.Kind == "conflict" && p.Conflict != nil:
		title = lang.L("Save conflict — {{.Game}}", map[string]any{"Game": p.GameName})
		skip := lang.L("Decide later")
		if p.BeforePlay {
			skip = lang.L("Skip sync and play")
		}
		body = kit.VBox(
			kit.Muted(conflictIntro(p.Conflict)),
			saveSides(p.Conflict.Local, p.Conflict.Remote, true, answer),
			kit.HBox(widget.NewButton(skip, func() { answer("skip") })),
		)
	case p.Kind == "emulator":
		title = lang.L("Play {{.Game}}", map[string]any{"Game": p.GameName})
		body = q.emulatorChoice(p, answer)
	case p.Kind == "lock" && p.Lock != nil:
		title = lang.L("{{.Game}} is being played elsewhere", map[string]any{"Game": p.GameName})
		body = kit.VBox(
			kit.Colored(lang.L("{{.Device}} started playing at {{.Time}}. If you play here too, you may get a save conflict when both devices upload.",
				map[string]any{"Device": p.Lock.DeviceName, "Time": fmtDate(p.Lock.Since)}), widget.MediumImportance),
			kit.HBox(widget.NewButton(lang.L("Cancel"), func() { answer("no") }), kit.Primary(lang.L("Play anyway"), func() { answer("yes") })),
		)
	default:
		errText := kit.Mono(p.Error)
		errText.Importance = widget.DangerImportance
		if p.BeforePlay {
			title = lang.L("Could not sync before playing")
			body = kit.VBox(errText,
				kit.Muted(lang.L("You can play offline; your saves will be uploaded next time sync succeeds.")),
				kit.HBox(widget.NewButton(lang.L("Cancel"), func() { answer("no") }), kit.Primary(lang.L("Play offline"), func() { answer("yes") })))
		} else {
			title = lang.L("Could not upload saves")
			body = kit.VBox(errText,
				kit.Muted(lang.L("Your saves are safe on this device and will upload on the next sync.")),
				kit.HBox(kit.Primary(lang.L("OK"), func() { answer("yes") })))
		}
	}
	size := q.u.win.Canvas().Size()
	q.current = dialog.NewCustomWithoutButtons(title, body, q.u.win)
	q.current.Resize(fyne.NewSize(min(560, size.Width-20), 0))
	q.current.Show()
}

// conflictIntro explains a conflict. A device whose saves are gone while the
// cloud still has them (a missing or unmounted save folder, a path that no
// longer points at the saves) isn't a change of both sides: say so, because
// "keep this device" would delete the cloud saves.
func conflictIntro(c *services.ConflictInfo) string {
	if c.Local.Files == 0 && c.Remote.Files > 0 {
		return lang.L("There are no saves on this device any more, but the cloud still has them. If the save folder is missing or its path changed, keep the cloud saves to restore them. Keeping this device deletes the cloud copy.")
	}
	return lang.L("Saves changed both on this device and in the cloud since the last sync. Choose which to keep. The other version is stored as a backup, so nothing is lost.")
}
