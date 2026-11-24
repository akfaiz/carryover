package kit

import "github.com/egoist/mygo/ui"

// TestButton is a button that runs a check in the background and reports the
// outcome next to it: "Testing…" on the button while it runs, then a green or
// red message. Keep it on the page: it holds the last result between frames.
type TestButton struct {
	idle, okText, runningText string
	prepare                   func() func() error
	allowed                   bool
	running                   bool
	result                    string
	failed                    bool
}

// NewTestButton creates a TestButton labelled idle. prepare runs on the UI
// goroutine when the button is pressed (read the field there) and returns the
// check that runs in the background. okText is shown when it succeeds and
// runningText is on the button while the check runs.
func NewTestButton(idle, okText, runningText string, prepare func() func() error) *TestButton {
	return &TestButton{idle: idle, okText: okText, runningText: runningText, prepare: prepare, allowed: true}
}

// SetAllowed enables the button unless a check is running.
func (t *TestButton) SetAllowed(on bool) { t.allowed = on }

// Reset hides the last result, e.g. when the thing being tested changed.
func (t *TestButton) Reset() { t.result = "" }

// Button draws the button; key tells it from the others on the page.
func (t *TestButton) Button(c *ui.Context, nav *Navigator, key any) ui.Element {
	label := t.idle
	if t.running {
		label = t.runningText
	}
	return Btn(c, nav, [2]any{"test", key}, BtnSecondary, nil, label, t.run, BtnDisabled(!t.allowed), BtnBusy(t.running))
}

// Result draws the outcome of the last check, if there is one.
func (t *TestButton) Result(c *ui.Context) {
	switch {
	case t.result == "":
	case t.failed:
		Danger(c, t.result)
	default:
		Colored(c, t.result, col(ColOK))
	}
}

func (t *TestButton) run() {
	if t.running || !t.allowed {
		return
	}
	work := t.prepare()
	t.running, t.result = true, ""
	Async0(work, func(err error) {
		t.running = false
		if err != nil {
			t.result, t.failed = err.Error(), true
		} else {
			t.result, t.failed = t.okText, false
		}
	})
}
