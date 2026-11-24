// Package prompts shows the session prompts (a save conflict, the cloud lock,
// a sync error, which emulator to play with) one at a time as modal dialogs.
// This is the MyGo build: the queue turns the first prompt into a kit.Dialog
// that the window draws, and every answer comes from a control the gamepad
// reaches.
package prompts

import (
	"github.com/egoist/mygo/ui"

	"github.com/akfaiz/carryover/services"
	"github.com/akfaiz/carryover/ui/components"
	"github.com/akfaiz/carryover/ui/env"
	"github.com/akfaiz/carryover/ui/i18n"
	"github.com/akfaiz/carryover/ui/kit"
)

// Queue shows session prompts one at a time as modal dialogs.
type Queue struct {
	// Wrapper is set in the session wrapper window (opened by a Steam shortcut),
	// where prompts also say how to answer with a controller.
	Wrapper bool

	e       env.Env
	queue   []services.Prompt
	current *kit.Dialog
	emu     *emuState // the "which emulator?" prompt on screen
}

// New returns an empty queue that shows its dialogs on the environment's window.
func New(e env.Env) *Queue { return &Queue{e: e} }

// Pending lists the prompts waiting, the one on screen first.
func (q *Queue) Pending() []services.Prompt { return q.queue }

// Current is the dialog on screen, nil when there is none.
func (q *Queue) Current() *kit.Dialog { return q.current }

// CloseCurrent takes the current prompt's dialog down.
func (q *Queue) CloseCurrent() {
	if q.emu != nil {
		q.emu.stopCountdown(true)
		q.emu = nil
		q.e.Navigator().OnInput = nil
	}
	if q.current != nil {
		q.current.Close()
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
	if len(q.queue) == 0 {
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
	// Steam Deck is played with one): the D-pad moves between the buttons, A
	// presses one, B gives the safe answer. The focus starts on that safe
	// button, so a stray press doesn't overwrite saves.
	var back string
	d := &kit.Dialog{Width: 560}
	// btn draws one answer button; safe marks the one the dialog starts on.
	btn := func(c *ui.Context, nav *kit.Navigator, label, a string, primary, safe bool) {
		kind := kit.BtnSecondary
		if primary {
			kind = kit.BtnPrimary
		}
		var opts []kit.BtnOption
		if safe {
			opts = append(opts, kit.BtnItem(kit.FocusSafe))
		}
		kit.Btn(c, nav, [2]any{"prompt", label}, kind, nil, label, func() { answer(a) }, opts...)
	}
	var body func(c *ui.Context, nav *kit.Navigator)
	switch {
	case p.Kind == "conflict" && p.Conflict != nil:
		d.Title = i18n.L("Save conflict — {{.Game}}", map[string]any{"Game": p.GameName})
		skip := i18n.L("Decide later")
		if p.BeforePlay {
			skip = i18n.L("Skip sync and play")
		}
		back = "skip"
		body = func(c *ui.Context, nav *kit.Navigator) {
			kit.Muted(c, conflictIntro(p.Conflict))
			components.SaveSides(c, nav, "prompt", p.Conflict.Local, p.Conflict.Remote, true, answer)
			kit.Footer(c, func() { btn(c, nav, skip, "skip", false, true) })
		}
	case p.Kind == "emulator":
		d.Title = i18n.L("Play {{.Game}}", map[string]any{"Game": p.GameName})
		back = "cancel"
		q.emu = newEmuState(q, p, answer)
		body = q.emu.body
	case p.Kind == "lock" && p.Lock != nil:
		d.Title = i18n.L("{{.Game}} is being played elsewhere", map[string]any{"Game": p.GameName})
		back = "no"
		body = func(c *ui.Context, nav *kit.Navigator) {
			ui.Text(c, i18n.L("{{.Device}} started playing at {{.Time}}. If you play here too, you may get a save conflict when both devices upload.",
				map[string]any{"Device": p.Lock.DeviceName, "Time": components.FmtDate(p.Lock.Since)})).Wrap()
			kit.Footer(c, func() {
				btn(c, nav, i18n.L("Cancel"), "no", false, true)
				btn(c, nav, i18n.L("Play anyway"), "yes", true, false)
			})
		}
	default:
		if p.BeforePlay {
			d.Title = i18n.L("Could not sync before playing")
			back = "no"
			body = func(c *ui.Context, nav *kit.Navigator) {
				kit.Danger(c, p.Error).Font("monospace")
				kit.Muted(c, i18n.L("You can play offline; your saves will be uploaded next time sync succeeds."))
				kit.Footer(c, func() {
					btn(c, nav, i18n.L("Cancel"), "no", false, true)
					btn(c, nav, i18n.L("Play offline"), "yes", true, false)
				})
			}
		} else {
			d.Title = i18n.L("Could not upload saves")
			back = "yes"
			body = func(c *ui.Context, nav *kit.Navigator) {
				kit.Danger(c, p.Error).Font("monospace")
				kit.Muted(c, i18n.L("Your saves are safe on this device and will upload on the next sync."))
				kit.Footer(c, func() { btn(c, nav, i18n.L("OK"), "yes", true, true) })
			}
		}
	}
	d.Back = func() { answer(back) }
	d.Body = func(c *ui.Context, nav *kit.Navigator) {
		body(c, nav)
		if q.Wrapper {
			// Opened by a Steam shortcut: likely driven with a controller.
			kit.PadHints(c, []kit.PadHint{
				{Button: "D-pad", Label: i18n.L("Choose")},
				{Button: "A / Enter", Label: i18n.L("Confirm")},
				{Button: "B / Esc", Label: i18n.L("Back")},
			}).Padding(kit.S2, 0, 0, 0)
		}
	}
	q.current = q.e.Dialogs().Show(d)
}

// conflictIntro explains a conflict. A device whose saves are gone while the
// cloud still has them (a missing or unmounted save folder, a path that no
// longer points at the saves) isn't a change of both sides: say so, because
// "keep this device" would delete the cloud saves.
func conflictIntro(c *services.ConflictInfo) string {
	if c.Local.Files == 0 && c.Remote.Files > 0 {
		return i18n.L("There are no saves on this device any more, but the cloud still has them. If the save folder is missing or its path changed, keep the cloud saves to restore them. Keeping this device deletes the cloud copy.")
	}
	return i18n.L("Saves changed both on this device and in the cloud since the last sync. Choose which to keep. The other version is stored as a backup, so nothing is lost.")
}
