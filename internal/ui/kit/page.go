package kit

import (
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// Async runs work off the UI thread and then done on it.
func Async[T any](work func() (T, error), done func(T, error)) {
	go func() {
		v, err := work()
		OnUI(func() { done(v, err) })
	}()
}

// Async0 is async for work without a result.
func Async0(work func() error, done func(error)) {
	go func() {
		err := work()
		OnUI(func() { done(err) })
	}()
}

// PagePadded gives page content the same margin on every side.
func PagePadded(content fyne.CanvasObject) fyne.CanvasObject {
	const m = 28
	return container.New(layout.NewCustomPaddedLayout(m, m, m, m), content)
}

// ScrollPage wraps page content in a padded vertical scroll.
func ScrollPage(content fyne.CanvasObject) *container.Scroll {
	return container.NewVScroll(PagePadded(content))
}

// ReplaceScroll swaps a scroll's content, keeping its position.
func ReplaceScroll(s *container.Scroll, content fyne.CanvasObject) {
	off := s.Offset
	s.Content = PagePadded(content)
	s.Refresh()
	s.Offset = off
	s.Refresh()
}

// Debounce returns a function that runs f on the UI thread d after its last call.
func Debounce(d time.Duration, f func()) func() {
	var t *time.Timer
	return func() {
		if t != nil {
			t.Stop()
		}
		t = time.AfterFunc(d, func() { OnUI(f) })
	}
}

// PageHead is a page title with an optional back button and actions.
func PageHead(back func(), text string, actions ...fyne.CanvasObject) fyne.CanvasObject {
	var objs []fyne.CanvasObject
	if back != nil {
		b := widget.NewButtonWithIcon("", theme.NavigateBackIcon(), back)
		b.Importance = widget.LowImportance
		objs = append(objs, b)
	}
	objs = append(objs, Title(text), layout.NewSpacer())
	objs = append(objs, actions...)
	return container.NewHBox(objs...)
}

// OnUI runs f on the Fyne UI goroutine. It is a variable so tests can
// serialize UI work (Fyne's test driver runs fyne.Do inline on the caller).
var OnUI = fyne.Do
