package kit

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newWindow is a window of a known size for dialogs to sit in.
func newWindow(t *testing.T) fyne.Window {
	t.Helper()
	test.NewApp()
	w := test.NewWindow(widget.NewLabel("page"))
	t.Cleanup(w.Close)
	w.Resize(fyne.NewSize(1000, 700))
	return w
}

func overlayButtons(w fyne.Window) []*widget.Button {
	var out []*widget.Button
	for _, o := range w.Canvas().Overlays().List() {
		out = append(out, all[*widget.Button](o)...)
	}
	return out
}

func tapOverlay(t *testing.T, w fyne.Window, text string) {
	t.Helper()
	for _, b := range overlayButtons(w) {
		if b.Text == text {
			test.Tap(b)
			return
		}
	}
	require.Failf(t, "button not found", "no button %q", text)
}

func TestShowCustomIsCappedByWidthAndWindow(t *testing.T) {
	w := newWindow(t)
	h := ShowCustom(w, "Title", widget.NewLabel("body"), 500)
	require.Len(t, w.Canvas().Overlays().List(), 1)
	h.Hide()
	assert.Empty(t, w.Canvas().Overlays().List())

	w.Resize(fyne.NewSize(300, 300))
	d := ShowCustom(w, "Title", widget.NewLabel("body"), 500)
	assert.LessOrEqual(t, w.Canvas().Overlays().Top().Size().Width, float32(300), "never wider than the window")
	d.Hide()

	assert.Equal(t, fyne.NewSize(820, 600), DialogSize(newWindow(t)), "large windows get the usual size")
	w.Resize(fyne.NewSize(400, 300))
	assert.Equal(t, fyne.NewSize(360, 260), DialogSize(w), "small windows leave a margin")
}

func TestScrollDialog(t *testing.T) {
	w := newWindow(t)
	h := ScrollDialog(w, "Pick", widget.NewLabel("lots"))
	assert.Len(t, w.Canvas().Overlays().List(), 1)
	h.Hide()
	assert.Empty(t, w.Canvas().Overlays().List())
}

func TestPickFileOpensADialog(t *testing.T) {
	w := newWindow(t)
	called := false
	PickFile(w, "Choose", []string{".png"}, func(string) { called = true })
	assert.Len(t, w.Canvas().Overlays().List(), 1)
	assert.False(t, called, "nothing is picked until the user does")
	PickFile(w, "Choose any", nil, func(string) {})
}

func TestConfirmDangerWithACheckbox(t *testing.T) {
	w := newWindow(t)
	var confirmed *bool
	opts := ConfirmOptions{
		Title: "Delete game", Message: "Move it to the trash?", Check: "Also delete saves", Checked: "Delete it for good?",
		KeptHint: "The saves stay.", Action: "Delete", CancelLabel: "Cancel",
		OnConfirm: func(on bool) { confirmed = &on },
	}
	ConfirmDanger(w, opts)
	top := w.Canvas().Overlays().Top()
	assert.Contains(t, texts(top), "Move it to the trash?")
	assert.Contains(t, texts(top), "The saves stay.")

	// Ticking swaps the message and drops the hint; unticking brings them back.
	check := all[*widget.Check](top)
	require.Len(t, check, 1)
	test.Tap(check[0])
	assert.Contains(t, texts(top), "Delete it for good?")
	for _, l := range all[*widget.Label](top) {
		if l.Text == "The saves stay." {
			assert.False(t, l.Visible())
		}
	}
	test.Tap(check[0])
	assert.Contains(t, texts(top), "Move it to the trash?")

	// Cancel closes without confirming.
	tapOverlay(t, w, "Cancel")
	assert.Nil(t, confirmed)
	assert.Empty(t, w.Canvas().Overlays().List())

	// Confirming with the box ticked reports it.
	ConfirmDanger(w, opts)
	test.Tap(all[*widget.Check](w.Canvas().Overlays().Top())[0])
	tapOverlay(t, w, "Delete")
	require.NotNil(t, confirmed)
	assert.True(t, *confirmed)
	assert.Empty(t, w.Canvas().Overlays().List())
}

func TestConfirmDangerWithoutACheckbox(t *testing.T) {
	w := newWindow(t)
	got := true
	ConfirmDanger(w, ConfirmOptions{Title: "Delete", Message: "Sure?", Action: "Remove", CancelLabel: "No", OnConfirm: func(on bool) { got = on }})
	assert.Empty(t, all[*widget.Check](w.Canvas().Overlays().Top()))
	tapOverlay(t, w, "Remove")
	assert.False(t, got)
}

func TestConfirmDiscard(t *testing.T) {
	w := newWindow(t)
	left := 0
	ConfirmDiscard(w, "Discard?", "Unsaved.", "Keep editing", "Discard", func() { left++ })
	assert.Contains(t, texts(w.Canvas().Overlays().Top()), "Unsaved.")
	tapOverlay(t, w, "Keep editing")
	assert.Zero(t, left)
	assert.Empty(t, w.Canvas().Overlays().List())

	ConfirmDiscard(w, "Discard?", "Unsaved.", "Keep editing", "Discard", func() { left++ })
	tapOverlay(t, w, "Discard")
	assert.Equal(t, 1, left)
	assert.Empty(t, w.Canvas().Overlays().List())
}
