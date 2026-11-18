package ui

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/widget"

	"github.com/akfaiz/carryover/services"
	"github.com/akfaiz/carryover/ui/internal/uitest"
)

// The generic helpers live in uitest, where the page packages use them too; the
// root's tests keep their short names.

func buttonWith(t *testing.T, buttons []*widget.Button, text string) *widget.Button {
	t.Helper()
	return uitest.ButtonWith(t, buttons, text)
}
func tapButton(t *testing.T, h *harness, text string) {
	t.Helper()
	uitest.TapButton(t, h.Harness, text)
}
func tapOverlayButton(t *testing.T, h *harness, text string) {
	t.Helper()
	uitest.TapOverlayButton(t, h.Harness, text)
}
func overlayButtons(h *harness) []*widget.Button { return uitest.OverlayButtons(h.Harness) }
func entryByPlaceholder(t *testing.T, h *harness, placeholder string) *widget.Entry {
	t.Helper()
	return uitest.EntryByPlaceholder(t, h.Harness, placeholder)
}
func labelTexts(h *harness) []string { return uitest.LabelTexts(h.Harness) }
func overlayText(h *harness) string  { return uitest.OverlayText(h.Harness) }
func tapChoice(t *testing.T, h *harness, name string) {
	t.Helper()
	uitest.TapChoice(t, h.Harness, name)
}
func (h *harness) addGame(name, saves string) services.GameView {
	h.t.Helper()
	return h.AddGame(name, saves)
}
func (h *harness) addEmulatorGame() (services.GameView, services.EmulatorView, services.EmulatorView) {
	h.t.Helper()
	return h.AddEmulatorGame()
}
func (h *harness) useLocalCloud() string { h.t.Helper(); return h.UseLocalCloud() }

func reachable(h *harness, start fyne.Focusable) map[fyne.Focusable]bool {
	return uitest.Reachable(h.Harness, start)
}
func describe(f fyne.Focusable) string { return uitest.Describe(f) }
