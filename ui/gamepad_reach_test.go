package ui

import (
	"fmt"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/widget"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/akfaiz/carryover/internal/ui/kit"
	"github.com/akfaiz/carryover/services"
)

// reachable walks the D-pad from start in every direction and returns every
// control it gets to.
func reachable(h *harness, start fyne.Focusable) map[fyne.Focusable]bool {
	c := h.u.win.Canvas()
	seen := map[fyne.Focusable]bool{start: true}
	queue := []fyne.Focusable{start}
	for len(queue) > 0 {
		from := queue[0]
		queue = queue[1:]
		for _, dir := range []kit.PadAction{kit.PadUp, kit.PadDown, kit.PadLeft, kit.PadRight} {
			h.u.nav.Focus(from) // scrolled into view, as when the D-pad got there
			h.u.nav.Apply(dir)
			if to := c.Focused(); to != nil && !seen[to] {
				seen[to] = true
				queue = append(queue, to)
			}
		}
	}
	return seen
}

// describe names a control for a failure message.
func describe(f fyne.Focusable) string {
	switch w := f.(type) {
	case *widget.Button:
		return fmt.Sprintf("button %q", w.Text)
	case *widget.Check:
		return fmt.Sprintf("check %q", w.Text)
	case *widget.Select:
		return fmt.Sprintf("select %q", w.Selected)
	case *widget.Entry:
		return fmt.Sprintf("entry %q", w.PlaceHolder)
	case *widget.Hyperlink:
		return fmt.Sprintf("link %q", w.Text)
	}
	return fmt.Sprintf("%T", f)
}

// Every control on every page can be reached with the D-pad from where a
// gamepad starts: nothing is stranded where only a mouse or touch gets to it.
func TestGamepadReachesEveryControl(t *testing.T) {
	h := newHarness(t)
	h.useLocalCloud() // the remote list, sync buttons
	g, _, _ := h.addEmulatorGame()
	plain := h.addGame("Celeste", t.TempDir())
	gone := h.addGame("Hades", t.TempDir())
	require.NoError(t, h.u.lib.Delete(gone.ID)) // something in the trash
	h.u.refresh()
	h.settle()
	for _, route := range []string{
		"/", "/game/" + g.ID, "/game/" + plain.ID, "/edit/" + g.ID, "/edit/" + plain.ID,
		"/new", "/new/emulator", "/emulators", "/cloud", "/trash", "/settings",
	} {
		t.Run(route, func(t *testing.T) {
			h.open(route)
			h.u.nav.FocusDefault()
			start := h.u.win.Canvas().Focused()
			require.NotNil(t, start)
			got := reachable(h, start)
			var missed []string
			for _, f := range kit.Focusables(h.u.win.Content()) {
				if !got[f] {
					missed = append(missed, describe(f))
				}
			}
			assert.Empty(t, missed, "not reachable with the D-pad")
		})
	}
}

// Every control of a dialog can be reached with the D-pad from where the first
// press starts, and B closes the dialog.
func TestGamepadReachesDialogControls(t *testing.T) {
	cases := map[string]func(h *harness){
		"add game":        func(h *harness) { showAddGame(h.u) },
		"SteamGridDB key": func(h *harness) { showGridKeyDialog(h.u, nil) },
		"add emulator":    func(h *harness) { showEmulatorDialog(h.u, func(services.EmulatorView) {}) },
		"add remote":      func(h *harness) { newRemoteWizard(&cloudPage{u: h.u}).show() },
		"discard changes": func(h *harness) { confirmDiscard(h.u, func() {}) },
		"confirm a delete": func(h *harness) {
			confirmDanger(h.u, confirmOpts{title: "Delete?", message: "Sure?", onConfirm: func(bool) {}})
		},
		"confirm with more": func(h *harness) {
			confirmDanger(h.u, confirmOpts{title: "Delete?", message: "Sure?", check: "Saves too", onConfirm: func(bool) {}})
		},
	}
	for name, open := range cases {
		t.Run(name, func(t *testing.T) {
			h := newHarness(t)
			open(h)
			h.settle()
			c := h.u.win.Canvas()
			top := c.Overlays().Top()
			require.NotNil(t, top)
			if c.Focused() == nil {
				h.u.nav.Apply(kit.PadDown) // the first press picks where to start
			}
			start := c.Focused()
			require.NotNil(t, start)
			got := reachable(h, start)
			var missed []string
			for _, f := range kit.Focusables(top) {
				if !got[f] {
					missed = append(missed, describe(f))
				}
			}
			assert.Empty(t, missed, "not reachable with the D-pad")

			h.u.nav.Apply(kit.PadBack)
			h.settle()
			assert.Empty(t, c.Overlays().List(), "B closes it")
		})
	}
}

// B in the emulator dialog is its Cancel: it asks before dropping what was
// typed.
func TestGamepadBackAsksBeforeDroppingAnEmulator(t *testing.T) {
	h := newHarness(t)
	showEmulatorDialog(h.u, func(services.EmulatorView) {})
	h.settle()
	c := h.u.win.Canvas()
	for _, e := range overlayWidgets[*widget.Entry](h) {
		e.SetText("Eden")
		break
	}
	h.u.nav.Apply(kit.PadBack)
	require.Len(t, c.Overlays().List(), 2, "Discard changes? opens over the form")
	assert.Contains(t, overlayText(h), "Your changes on this page haven't been saved.")
	h.u.nav.Apply(kit.PadBack)
	assert.Len(t, c.Overlays().List(), 1, "B keeps editing")
}
