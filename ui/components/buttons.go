package components

import (
	"fyne.io/fyne/v2/lang"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// RemoveButton is a "Remove" button tinted red (inside kit.Tonal), so it
// doesn't read like the button next to it.
func RemoveButton(onRemove func()) *widget.Button {
	b := widget.NewButtonWithIcon(lang.L("Remove"), theme.DeleteIcon(), onRemove)
	b.Importance = widget.DangerImportance
	return b
}
