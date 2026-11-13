package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/lang"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/akfaiz/carryover/internal/cloudsync/cloud"
	"github.com/akfaiz/carryover/internal/ui/kit"
)

// remoteRow is one configured rclone remote: name (with an "active" badge),
// type (with an "unsupported" badge) and a delete button.
type remoteRow struct {
	widget.BaseWidget
	content fyne.CanvasObject
}

func newRemoteRow(r cloud.Remote, active bool, onDelete func()) *remoteRow {
	name := kit.HBox(widget.NewLabelWithStyle(r.Name, fyne.TextAlignLeading, fyne.TextStyle{Bold: true}))
	if active {
		name.Add(container.NewCenter(kit.Badge(lang.L("active"), kit.BadgeOK)))
	}
	typ := kit.HBox(kit.Small(r.Type))
	if !r.Supported {
		typ.Add(container.NewCenter(kit.Badge(lang.L("unsupported"), kit.BadgeWarn)))
	}
	del := widget.NewButtonWithIcon(lang.L("Delete"), theme.DeleteIcon(), onDelete)
	row := &remoteRow{content: container.NewBorder(nil, nil, nil, del, container.NewGridWithColumns(2, name, typ))}
	row.ExtendBaseWidget(row)
	return row
}

// CreateRenderer implements fyne.Widget.
func (r *remoteRow) CreateRenderer() fyne.WidgetRenderer { return widget.NewSimpleRenderer(r.content) }
