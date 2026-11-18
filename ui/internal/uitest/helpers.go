package uitest

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
	"github.com/rclone/rclone/fs/config"
	"github.com/stretchr/testify/require"

	"github.com/akfaiz/carryover/internal/cloudsync/cloud"
	"github.com/akfaiz/carryover/internal/library"
	"github.com/akfaiz/carryover/services"
	"github.com/akfaiz/carryover/ui/kit"
)

// ButtonWith finds the button with that text among buttons.
func ButtonWith(t *testing.T, buttons []*widget.Button, text string) *widget.Button {
	t.Helper()
	for _, b := range buttons {
		if b.Text == text {
			return b
		}
	}
	require.Failf(t, "button not found", "no button %q", text)
	return nil
}

// TapButton taps the button of the page with that text.
func TapButton(t *testing.T, h *Harness, text string) {
	t.Helper()
	test.Tap(ButtonWith(t, PageWidgets[*widget.Button](h), text))
}

// TapOverlayButton taps the button of an open dialog with that text.
func TapOverlayButton(t *testing.T, h *Harness, text string) {
	t.Helper()
	test.Tap(ButtonWith(t, OverlayButtons(h), text))
}

// OverlayButtons lists the buttons of the open dialogs: plain ones and those of
// a dialog driven by a kit.Nav (keyboard and gamepad control).
func OverlayButtons(h *Harness) []*widget.Button {
	out := OverlayWidgets[*widget.Button](h)
	for _, nb := range OverlayWidgets[*kit.NavButton](h) {
		out = append(out, &nb.Button)
	}
	return out
}

// ButtonTexts lists the texts of the buttons of the open dialogs.
func ButtonTexts(h *Harness) []string {
	var out []string
	for _, b := range OverlayButtons(h) {
		out = append(out, b.Text)
	}
	return out
}

// ButtonTextsOnPage lists the texts of the buttons of the page.
func ButtonTextsOnPage(h *Harness) []string {
	var out []string
	for _, b := range PageWidgets[*widget.Button](h) {
		out = append(out, b.Text)
	}
	return out
}

// EntryByPlaceholder finds the entry of the page with that placeholder.
func EntryByPlaceholder(t *testing.T, h *Harness, placeholder string) *widget.Entry {
	t.Helper()
	for _, e := range PageWidgets[*widget.Entry](h) {
		if e.PlaceHolder == placeholder {
			return e
		}
	}
	require.Failf(t, "entry not found", "no entry with placeholder %q", placeholder)
	return nil
}

// LabelTexts lists the texts of the labels of the page.
func LabelTexts(h *Harness) []string {
	var out []string
	for _, l := range PageWidgets[*widget.Label](h) {
		out = append(out, l.Text)
	}
	return out
}

// OverlayText is the text of the visible labels in open dialogs.
func OverlayText(h *Harness) string {
	var out []string
	for _, l := range OverlayWidgets[*widget.Label](h) {
		if l.Visible() {
			out = append(out, l.Text)
		}
	}
	return strings.Join(out, "\n")
}

// HasText reports whether a canvas text or label with exactly this text is
// somewhere under o.
func HasText(o fyne.CanvasObject, text string) bool {
	switch x := o.(type) {
	case *canvas.Text:
		return x.Text == text
	case *fyne.Container:
		for _, c := range x.Objects {
			if HasText(c, text) {
				return true
			}
		}
	case fyne.Widget:
		for _, c := range test.WidgetRenderer(x).Objects() {
			if HasText(c, text) {
				return true
			}
		}
	}
	return false
}

// HasBadge reports whether a badge with this text is shown in the open dialog.
func HasBadge(h *Harness, text string) bool {
	for _, o := range h.Shell.Window().Canvas().Overlays().List() {
		if HasText(o, text) {
			return true
		}
	}
	return false
}

// TapChoice taps the tile of a choice dialog (Add a game, which emulator).
func TapChoice(t *testing.T, h *Harness, name string) {
	t.Helper()
	for _, c := range OverlayWidgets[*kit.ChoiceTile](h) {
		if c.Name == name {
			test.Tap(c.Btn)
			h.Settle()
			return
		}
	}
	require.Failf(t, "kit.Choice not found", "no tile %q", name)
}

// AddGame adds a game with one save location at saves.
func (h *Harness) AddGame(name string, saves string) services.GameView {
	h.T.Helper()
	g, err := h.Shell.Lib().Save(services.GameInput{
		Name:  name,
		Slots: []library.SaveSlot{{Key: "saves", Label: "Saves"}},
		Device: services.DeviceInput{
			Paths: map[string]string{"saves": saves}, Exe: "/x/game.exe", LaunchMode: library.LaunchDirect,
		},
	})
	require.NoError(h.T, err)
	h.Shell.Store().Refresh()
	return g
}

// AddEmulatorGame adds a game with its own saves and two emulators.
func (h *Harness) AddEmulatorGame() (services.GameView, services.EmulatorView, services.EmulatorView) {
	h.T.Helper()
	eden, err := h.Shell.Emus().Save(services.EmulatorSetup{Name: "Eden", Exe: "eden"})
	require.NoError(h.T, err)
	ryu, err := h.Shell.Emus().Save(services.EmulatorSetup{Name: "Ryujinx", Exe: "ryujinx"})
	require.NoError(h.T, err)
	slots := []library.SaveSlot{{Key: "saves", Label: "Saves"}}
	g, err := h.Shell.Lib().Save(services.GameInput{
		Name: "Mario Kart", Slots: slots,
		Device: services.DeviceInput{Paths: map[string]string{"saves": h.T.TempDir()}, Exe: "/x/game.exe", LaunchMode: library.LaunchDirect},
		Emulators: []services.GameEmulatorInput{
			{EmulatorID: eden.ID, Slots: slots, Rom: "mk.nsp", Paths: map[string]string{"saves": h.T.TempDir()}},
			{EmulatorID: ryu.ID, Slots: slots, Rom: "mk.nsp", Paths: map[string]string{"saves": h.T.TempDir()}},
		},
	})
	require.NoError(h.T, err)
	h.Shell.Store().Refresh()
	h.Settle()
	return g, eden, ryu
}

// MustGame returns a game of the library, failing the test if it is missing.
func MustGame(t *testing.T, h *Harness, id string) services.GameView {
	t.Helper()
	g, ok := h.Shell.Store().Game(id)
	require.True(t, ok)
	return g
}

// UseLocalCloud points the app at a local folder acting as the cloud and
// returns it.
func (h *Harness) UseLocalCloud() string {
	h.T.Helper()
	config.FileSetValue("cloud", "type", "local")
	h.T.Cleanup(func() { config.DeleteRemote("cloud") })
	dir := filepath.Join(h.T.TempDir(), "cloud")
	st := h.Core.Settings.Get()
	st.Remote, st.RemoteRoot = "cloud", filepath.ToSlash(dir)
	require.NoError(h.T, h.Core.Settings.Update(st))
	h.Shell.Store().Refresh()
	return dir
}

// AddSFTPRemotes adds rclone remotes of type sftp. They never connect: the
// hosts don't exist. The caller removes them if it needs to.
func AddSFTPRemotes(t *testing.T, names ...string) {
	t.Helper()
	for _, name := range names {
		_, err := cloud.CreateRemote(context.Background(), name, "sftp", map[string]string{"host": name + ".invalid"})
		require.NoError(t, err)
	}
}

// Reachable walks the D-pad from start in every direction and returns every
// control it gets to.
func Reachable(h *Harness, start fyne.Focusable) map[fyne.Focusable]bool {
	c := h.Shell.Window().Canvas()
	seen := map[fyne.Focusable]bool{start: true}
	queue := []fyne.Focusable{start}
	for len(queue) > 0 {
		from := queue[0]
		queue = queue[1:]
		for _, dir := range []kit.PadAction{kit.PadUp, kit.PadDown, kit.PadLeft, kit.PadRight} {
			h.Shell.Navigator().Focus(from) // scrolled into view, as when the D-pad got there
			h.Shell.Navigator().Apply(dir)
			if to := c.Focused(); to != nil && !seen[to] {
				seen[to] = true
				queue = append(queue, to)
			}
		}
	}
	return seen
}

// Describe names a control for a failure message.
func Describe(f fyne.Focusable) string {
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
