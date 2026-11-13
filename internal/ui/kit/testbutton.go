package kit

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/widget"
)

// TestButton is a button that runs a check in the background and reports the
// outcome in Result: "Testing…" on the button while it runs, then a green or
// red message. Place Button and Result wherever the layout wants them; both
// survive page re-renders, so keep the testButton on the page.
type TestButton struct {
	Button *widget.Button
	Result *widget.Label

	idle, okText, runningText string
	prepare                   func() func() error
	allowed                   bool
	running                   bool
}

// NewTestButton creates a TestButton labelled idle. prepare runs on the UI
// goroutine when the button is tapped (read widget state there) and returns
// the check that runs in the background. okText is shown when it succeeds and
// runningText is on the button while the check runs.
func NewTestButton(idle, okText, runningText string, prepare func() func() error) *TestButton {
	t := &TestButton{idle: idle, okText: okText, runningText: runningText, prepare: prepare, allowed: true}
	t.Button = widget.NewButton(idle, t.run)
	t.Result = widget.NewLabel("")
	t.Result.Wrapping = fyne.TextWrapWord
	t.Result.Hide()
	return t
}

// SetAllowed enables the button unless a check is running.
func (t *TestButton) SetAllowed(on bool) {
	t.allowed = on
	t.update()
}

// Reset hides the last result, e.g. when the thing being tested changed.
func (t *TestButton) Reset() { t.Result.Hide() }

func (t *TestButton) update() { SetEnabled(t.Button, t.allowed && !t.running) }

func (t *TestButton) run() {
	if t.running {
		return
	}
	work := t.prepare()
	t.running = true
	t.Button.SetText(t.runningText)
	t.Result.Hide()
	t.update()
	Async0(work, func(err error) {
		t.running = false
		t.Button.SetText(t.idle)
		if err != nil {
			t.Result.Importance = widget.DangerImportance
			t.Result.SetText(err.Error())
		} else {
			t.Result.Importance = widget.SuccessImportance
			t.Result.SetText(t.okText)
		}
		t.Result.Show()
		t.update()
	})
}
