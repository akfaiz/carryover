package ui

import (
	"fmt"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	"github.com/akfaiz/playrelay/internal/syncer"
	"github.com/akfaiz/playrelay/services"
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
		title = "Save conflict — " + p.GameName
		newer := ""
		if p.Conflict.Local.Newest.After(p.Conflict.Remote.Newest) {
			newer = "local"
		} else if p.Conflict.Remote.Newest.After(p.Conflict.Local.Newest) {
			newer = "cloud"
		}
		side := func(key, label string, s syncer.Summary) fyne.CanvasObject {
			head := hbox(widget.NewLabelWithStyle(label, fyne.TextAlignLeading, fyne.TextStyle{Bold: true}))
			if newer == key {
				head.Add(container.NewCenter(badge("newer", badgeOK)))
			}
			from := ""
			if s.DeviceName != "" {
				from = "from " + s.DeviceName
			}
			keep := primary("Keep "+key+" saves", func() { answer(key) })
			info := widget.NewLabel(fmt.Sprintf("%d files · %s\nLast modified %s", s.Files, formatBytes(s.Bytes), fmtDate(s.Newest)))
			info.Wrapping = fyne.TextWrapWord
			return card("", head, muted(from), info, keep)
		}
		skip := "Decide later"
		if p.BeforePlay {
			skip = "Skip sync and play"
		}
		body = vbox(
			muted("Saves changed both on this device and in the cloud since the last sync. Choose which to keep. "+
				"The other version is stored as a backup, so nothing is lost."),
			container.NewGridWithColumns(2, side("local", "This device", p.Conflict.Local), side("cloud", "Cloud", p.Conflict.Remote)),
			hbox(widget.NewButton(skip, func() { answer("skip") })),
		)
	case p.Kind == "lock" && p.Lock != nil:
		title = p.GameName + " is being played elsewhere"
		body = vbox(
			colored(fmt.Sprintf("%s started playing at %s. If you play here too, you may get a save conflict when both devices upload.",
				p.Lock.DeviceName, fmtDate(p.Lock.Since)), widget.MediumImportance),
			hbox(widget.NewButton("Cancel", func() { answer("no") }), primary("Play anyway", func() { answer("yes") })),
		)
	default:
		errText := mono(p.Error)
		errText.Importance = widget.DangerImportance
		if p.BeforePlay {
			title = "Could not sync before playing"
			body = vbox(errText,
				muted("You can play offline; your saves will be uploaded next time sync succeeds."),
				hbox(widget.NewButton("Cancel", func() { answer("no") }), primary("Play offline", func() { answer("yes") })))
		} else {
			title = "Could not upload saves"
			body = vbox(errText,
				muted("Your saves are safe on this device and will upload on the next sync."),
				hbox(primary("OK", func() { answer("yes") })))
		}
	}
	size := q.u.win.Canvas().Size()
	q.current = dialog.NewCustomWithoutButtons(title, body, q.u.win)
	q.current.Resize(fyne.NewSize(min(560, size.Width-20), 0))
	q.current.Show()
}
