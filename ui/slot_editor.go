package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/lang"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/akfaiz/carryover/internal/saves/pathvar"
	"github.com/akfaiz/carryover/internal/ui/kit"
	"github.com/akfaiz/carryover/services"
)

// slotForm is the editable state of one save location.
type slotForm struct {
	key, label, include, exclude, path string
	isNew                              bool
	filters                            *kit.AccordionSection // set while a slotEditor shows this slot
	filtersOpen                        bool
}

// rememberOpen keeps the filters section open or closed across re-renders.
func (s *slotForm) rememberOpen() {
	if s.filters != nil {
		s.filtersOpen = s.filters.Open()
	}
}

// slotEditor edits one save location: label, key, this device's path and the
// include/exclude filters. It writes straight into the slotForm.
type slotEditor struct {
	widget.BaseWidget
	content fyne.CanvasObject
}

// newSlotEditor builds the editor for s. uniqueKey derives a key that is free
// among the other slots; a nil onRemove hides the delete button. pathChanged
// (if set) is told when the path on this device changes.
func newSlotEditor(u *UI, gameID string, tokens []pathvar.Token, s *slotForm, others []services.DevicePaths,
	uniqueKey func(base string, self *slotForm) string, onRemove, pathChanged func()) *slotEditor {
	label := widget.NewEntry()
	label.SetPlaceHolder(lang.L("Label"))
	label.SetText(s.label)
	keyBadge := container.NewCenter(kit.Badge(s.key, kit.BadgeNone))
	label.OnChanged = func(v string) {
		s.label = v
		if s.isNew {
			s.key = uniqueKey(slug(v), s)
			keyBadge.Objects = []fyne.CanvasObject{kit.Badge(s.key, kit.BadgeNone)}
			keyBadge.Refresh()
		}
	}
	right := []fyne.CanvasObject{keyBadge}
	if onRemove != nil {
		rm := widget.NewButtonWithIcon("", theme.DeleteIcon(), onRemove)
		rm.Importance = widget.DangerImportance
		right = append(right, rm)
	}

	c := kit.VBox(
		kit.RowFill(label, right...),
		kit.Field(lang.L("Path on this device"), newPathInput(u, gameID, tokens, s.path, "{appdata}/Game/Saves", func(v string) {
			s.path = v
			if pathChanged != nil {
				pathChanged()
			}
		})),
	)
	for _, o := range others {
		if p := o.Paths[s.key]; p != "" {
			l := kit.Mono(lang.L("On {{.Device}}: {{.Path}}", map[string]any{"Device": o.DeviceName, "Path": p}))
			l.Importance = widget.LowImportance
			l.SizeName = theme.SizeNameCaptionText
			c.Add(l)
		}
	}

	inc := kit.MonoMultiLine("*.sav", s.include, 2, func(v string) { s.include = v })
	exc := kit.MonoMultiLine("*.log\nshadercache/**", s.exclude, 2, func(v string) { s.exclude = v })
	s.filters = kit.NewAccordionSection(lang.L("Filters"), container.NewGridWithColumns(2,
		kit.Field(lang.L("Include only (globs, one per line)"), inc), kit.Field(lang.L("Exclude (globs)"), exc)), s.filtersOpen)
	c.Add(s.filters)

	e := &slotEditor{content: c}
	e.ExtendBaseWidget(e)
	return e
}

// CreateRenderer implements fyne.Widget.
func (e *slotEditor) CreateRenderer() fyne.WidgetRenderer { return widget.NewSimpleRenderer(e.content) }
