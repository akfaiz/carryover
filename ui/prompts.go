package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/lang"
	"fyne.io/fyne/v2/widget"

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
		body = vbox(
			muted(lang.L("Saves changed both on this device and in the cloud since the last sync. Choose which to keep. The other version is stored as a backup, so nothing is lost.")),
			saveSides(p.Conflict.Local, p.Conflict.Remote, true, answer),
			hbox(widget.NewButton(skip, func() { answer("skip") })),
		)
	case p.Kind == "emulator":
		title = lang.L("Play {{.Game}}", map[string]any{"Game": p.GameName})
		body = q.emulatorChoice(p, answer)
	case p.Kind == "lock" && p.Lock != nil:
		title = lang.L("{{.Game}} is being played elsewhere", map[string]any{"Game": p.GameName})
		body = vbox(
			colored(lang.L("{{.Device}} started playing at {{.Time}}. If you play here too, you may get a save conflict when both devices upload.",
				map[string]any{"Device": p.Lock.DeviceName, "Time": fmtDate(p.Lock.Since)}), widget.MediumImportance),
			hbox(widget.NewButton(lang.L("Cancel"), func() { answer("no") }), primary(lang.L("Play anyway"), func() { answer("yes") })),
		)
	default:
		errText := mono(p.Error)
		errText.Importance = widget.DangerImportance
		if p.BeforePlay {
			title = lang.L("Could not sync before playing")
			body = vbox(errText,
				muted(lang.L("You can play offline; your saves will be uploaded next time sync succeeds.")),
				hbox(widget.NewButton(lang.L("Cancel"), func() { answer("no") }), primary(lang.L("Play offline"), func() { answer("yes") })))
		} else {
			title = lang.L("Could not upload saves")
			body = vbox(errText,
				muted(lang.L("Your saves are safe on this device and will upload on the next sync.")),
				hbox(primary(lang.L("OK"), func() { answer("yes") })))
		}
	}
	size := q.u.win.Canvas().Size()
	q.current = dialog.NewCustomWithoutButtons(title, body, q.u.win)
	q.current.Resize(fyne.NewSize(min(560, size.Width-20), 0))
	q.current.Show()
}
