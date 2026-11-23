package components

import (
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/akfaiz/carryover/ui/i18n"
)

// RemoveButton is a "Remove" button tinted red (inside kit.Tonal), so it
// doesn't read like the button next to it.
func RemoveButton(onRemove func()) *widget.Button {
	b := widget.NewButtonWithIcon(i18n.L("Remove"), theme.DeleteIcon(), onRemove)
	b.Importance = widget.DangerImportance
	return b
}
