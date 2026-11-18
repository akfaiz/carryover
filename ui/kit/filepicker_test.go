package kit

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The Navigator reads which item of a list or grid is highlighted, which Fyne
// keeps unexported; this fails if a Fyne release renames it.
func TestHighlightOf(t *testing.T) {
	test.NewApp()
	items := func() int { return 5 }
	mk := func() fyne.CanvasObject { return widget.NewLabel("") }
	set := func(int, fyne.CanvasObject) {}
	list := widget.NewList(items, mk, set)
	grid := widget.NewGridWrap(items, mk, set)
	w := test.NewWindow(container.NewGridWithColumns(2, list, grid))
	w.Resize(fyne.NewSize(400, 400))
	for _, f := range []fyne.Focusable{list, grid} {
		before, ok := highlightOf(f)
		require.True(t, ok, "%T", f)
		f.TypedKey(&fyne.KeyEvent{Name: fyne.KeyDown})
		after, _ := highlightOf(f)
		assert.Greater(t, after, before, "%T", f)
	}
	_, ok := highlightOf(widget.NewButton("x", nil))
	assert.False(t, ok)
}

// The D-pad moves a list's highlight, and leaves the list at its end.
func TestNavigatorLeavesAListAtItsEdge(t *testing.T) {
	test.NewApp()
	var picked []int
	list := widget.NewList(func() int { return 3 },
		func() fyne.CanvasObject { return widget.NewLabel("") },
		func(id int, o fyne.CanvasObject) { o.(*widget.Label).SetText(fmt.Sprint(id)) })
	list.OnSelected = func(id int) { picked = append(picked, id) }
	open := widget.NewButton("Open", nil)
	w := test.NewWindow(container.NewBorder(nil, open, nil, nil, list))
	w.Resize(fyne.NewSize(300, 400))
	nav := NewNavigator(w, NavigatorOptions{})
	nav.Focus(list)
	nav.Apply(PadDown)
	nav.Apply(PadDown)
	require.Same(t, list, w.Canvas().Focused(), "moving inside the list")
	nav.Apply(PadAccept)
	assert.Equal(t, []int{2}, picked, "A picks the highlighted item")
	nav.Apply(PadDown)
	assert.Same(t, open, w.Canvas().Focused(), "past the last item: on to Open")
	nav.Apply(PadUp)
	assert.Same(t, list, w.Canvas().Focused())
}

// A Fyne folder picker works with a gamepad: the focus starts on its files,
// every control can be reached, and B cancels it.
func TestFilePickerWithAGamepad(t *testing.T) {
	test.NewApp()
	dir := t.TempDir()
	for _, n := range []string{"Saves", "Mods", "Screenshots"} {
		require.NoError(t, os.Mkdir(filepath.Join(dir, n), 0o755))
	}
	w := test.NewWindow(widget.NewLabel("page"))
	w.Resize(fyne.NewSize(1000, 700))
	nav := NewNavigator(w, NavigatorOptions{})
	nav.present = true
	cancelled := false
	d := dialog.NewFolderOpen(func(u fyne.ListableURI, err error) { cancelled = u == nil && err == nil }, w)
	loc, err := storage.ListerForURI(storage.NewFileURI(dir))
	require.NoError(t, err)
	d.SetLocation(loc)
	ShowFilePicker(w, d)

	c := w.Canvas()
	top := c.Overlays().Top()
	require.NotNil(t, top)
	start := c.Focused()
	require.NotNil(t, start, "the files have the focus")
	assert.Same(t, fileView(top), start)

	seen := map[fyne.Focusable]bool{start: true}
	queue := []fyne.Focusable{start}
	for len(queue) > 0 {
		from := queue[0]
		queue = queue[1:]
		for _, dir := range []PadAction{PadUp, PadDown, PadLeft, PadRight} {
			for range 8 { // a list takes several presses to reach its edge
				nav.Focus(from)
				nav.Apply(dir)
				if to := c.Focused(); to != from {
					if !seen[to] {
						seen[to] = true
						queue = append(queue, to)
					}
					break
				}
			}
		}
	}
	for _, f := range Focusables(top) {
		assert.True(t, seen[f], "%T not reachable", f)
	}

	nav.Apply(PadBack)
	assert.Empty(t, c.Overlays().List(), "B closes the picker")
	assert.True(t, cancelled, "as Cancel would")
}
