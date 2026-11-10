package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/widget"
)

// testButton is a button that runs a check in the background and reports the
// outcome in Result: "Testing…" on the button while it runs, then a green or
// red message. Place Button and Result wherever the layout wants them; both
// survive page re-renders, so keep the testButton on the page.
type testButton struct {
	Button *widget.Button
	Result *widget.Label

	idle, okText string
	prepare      func() func() error
	allowed      bool
	running      bool
}

// newTestButton creates a testButton labelled idle. prepare runs on the UI
// goroutine when the button is tapped (read widget state there) and returns
// the check that runs in the background. okText is shown when it succeeds.
func newTestButton(idle, okText string, prepare func() func() error) *testButton {
	t := &testButton{idle: idle, okText: okText, prepare: prepare, allowed: true}
	t.Button = widget.NewButton(idle, t.run)
	t.Result = widget.NewLabel("")
	t.Result.Wrapping = fyne.TextWrapWord
	t.Result.Hide()
	return t
}

// SetAllowed enables the button unless a check is running.
func (t *testButton) SetAllowed(on bool) {
	t.allowed = on
	t.update()
}

// Reset hides the last result, e.g. when the thing being tested changed.
func (t *testButton) Reset() { t.Result.Hide() }

func (t *testButton) update() { setEnabled(t.Button, t.allowed && !t.running) }

func (t *testButton) run() {
	if t.running {
		return
	}
	work := t.prepare()
	t.running = true
	t.Button.SetText("Testing…")
	t.Result.Hide()
	t.update()
	async0(work, func(err error) {
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
