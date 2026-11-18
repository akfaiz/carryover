package kit

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
)

// Collapsible is a one-line explanation with a button that unfolds more text
// below it. open remembers, by summary, which ones are unfolded, so pages that
// are drawn again don't fold them; onToggle (if set) runs after the user
// folds or unfolds one, for a page that must lay itself out again around it.
// moreLabel and lessLabel are the button's two texts. With no details it is
// just the muted summary.
func Collapsible(summary string, details []string, moreLabel, lessLabel string, open map[string]bool, onToggle func()) fyne.CanvasObject {
	if len(details) == 0 {
		return Muted(summary)
	}
	body := VBox()
	for _, d := range details {
		body.Add(Muted(d))
	}
	toggle := LowButton("", nil)
	set := func(on bool) {
		open[summary] = on
		if on {
			toggle.SetText(lessLabel)
			toggle.SetIcon(theme.MenuDropUpIcon())
			body.Show()
		} else {
			toggle.SetText(moreLabel)
			toggle.SetIcon(theme.MenuDropDownIcon())
			body.Hide()
		}
	}
	toggle.OnTapped = func() {
		set(!open[summary])
		if onToggle != nil {
			onToggle()
		}
	}
	set(open[summary])
	// The toggle under the text, where it reads as part of it, not a stray
	// word at the far right.
	return VBox(Muted(summary), body, HBox(toggle))
}
