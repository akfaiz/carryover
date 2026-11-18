package prompts

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/lang"
	"fyne.io/fyne/v2/widget"

	"github.com/akfaiz/carryover/services"
	"github.com/akfaiz/carryover/ui/components"
	"github.com/akfaiz/carryover/ui/env"
	"github.com/akfaiz/carryover/ui/kit"
)

// Queue shows session prompts one at a time as modal dialogs.
type Queue struct {
	// Wrapper is set in the session wrapper window (opened by a Steam shortcut),
	// where prompts also say how to answer with a controller.
	Wrapper bool

	e       env.Env
	queue   []services.Prompt
	current *dialog.CustomDialog
	nav     *kit.Nav // keyboard and gamepad control of the current prompt
}

// New returns an empty queue that shows its dialogs on the environment's window.
func New(e env.Env) *Queue { return &Queue{e: e} }

// Pending lists the prompts waiting, the one on screen first.
func (q *Queue) Pending() []services.Prompt { return q.queue }

// Current is the dialog on screen, nil when there is none.
func (q *Queue) Current() *dialog.CustomDialog { return q.current }

// CloseCurrent takes the current prompt's dialog down.
func (q *Queue) CloseCurrent() {
	if q.nav != nil {
		q.nav.Stop()
		q.nav = nil
	}
	if q.current != nil {
		q.current.Hide()
		q.current = nil
	}
}

// Add queues a prompt and shows it when no other is on screen.
func (q *Queue) Add(p services.Prompt) {
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

// Remove drops a prompt that was withdrawn (its sync was cancelled).
func (q *Queue) Remove(id string) {
	for i, p := range q.queue {
		if p.ID != id {
			continue
		}
		q.queue = append(q.queue[:i], q.queue[i+1:]...)
		if i == 0 && q.current != nil {
			q.CloseCurrent()
			q.next()
		}
		return
	}
}

func (q *Queue) next() {
	if len(q.queue) == 0 || q.e.Window() == nil {
		q.current = nil
		return
	}
	p := q.queue[0]
	answer := func(a string) {
		q.queue = q.queue[1:]
		q.CloseCurrent()
		go q.e.Sess().Respond(p.ID, a)
		q.next()
	}
	// Every prompt works with the keyboard and a gamepad (a Steam shortcut on a
	// Steam Deck is played with one): arrows or the D-pad move between the
	// buttons, Enter or A press one, Escape or B give the safe answer. The
	// focus starts on that safe button, so a stray press doesn't overwrite saves.
	var back string
	var first fyne.Focusable
	nav := kit.NewNav(q.e.Window(), func() { answer(back) })
	button := func(label string, imp widget.Importance, a string) *kit.NavButton {
		return nav.Button(label, imp, func() { answer(a) })
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
		skipBtn := button(skip, widget.MediumImportance, "skip")
		first, back = skipBtn, "skip"
		body = kit.VBox(
			kit.Muted(conflictIntro(p.Conflict)),
			components.SaveSidesNav(nav, p.Conflict.Local, p.Conflict.Remote, true, answer),
			kit.HBox(skipBtn),
		)
	case p.Kind == "emulator":
		title = lang.L("Play {{.Game}}", map[string]any{"Game": p.GameName})
		body, first = q.emulatorChoice(p, answer, nav)
		back = "cancel"
	case p.Kind == "lock" && p.Lock != nil:
		title = lang.L("{{.Game}} is being played elsewhere", map[string]any{"Game": p.GameName})
		cancel := button(lang.L("Cancel"), widget.MediumImportance, "no")
		first, back = cancel, "no"
		body = kit.VBox(
			kit.Colored(lang.L("{{.Device}} started playing at {{.Time}}. If you play here too, you may get a save conflict when both devices upload.",
				map[string]any{"Device": p.Lock.DeviceName, "Time": components.FmtDate(p.Lock.Since)}), widget.MediumImportance),
			kit.HBox(cancel, button(lang.L("Play anyway"), widget.HighImportance, "yes")),
		)
	default:
		errText := kit.Mono(p.Error)
		errText.Importance = widget.DangerImportance
		if p.BeforePlay {
			title = lang.L("Could not sync before playing")
			cancel := button(lang.L("Cancel"), widget.MediumImportance, "no")
			first, back = cancel, "no"
			body = kit.VBox(errText,
				kit.Muted(lang.L("You can play offline; your saves will be uploaded next time sync succeeds.")),
				kit.HBox(cancel, button(lang.L("Play offline"), widget.HighImportance, "yes")))
		} else {
			title = lang.L("Could not upload saves")
			ok := button(lang.L("OK"), widget.HighImportance, "yes")
			first, back = ok, "yes"
			body = kit.VBox(errText,
				kit.Muted(lang.L("Your saves are safe on this device and will upload on the next sync.")),
				kit.HBox(ok))
		}
	}
	if q.Wrapper {
		// Opened by a Steam shortcut: likely driven with a controller.
		body = kit.VBox(body, kit.Muted(lang.L("D-pad or arrow keys to choose, A or Enter to confirm, B or Esc to go back.")))
	}
	size := q.e.Window().Canvas().Size()
	q.current = dialog.NewCustomWithoutButtons(title, body, q.e.Window())
	q.current.Resize(fyne.NewSize(min(560, size.Width-20), 0))
	q.current.Show()
	q.nav = nav
	nav.Start(first)
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
