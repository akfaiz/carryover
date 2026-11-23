package cloud

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/akfaiz/carryover/ui/i18n"

	"github.com/akfaiz/carryover/internal/cloudsync/cloud"
	"github.com/akfaiz/carryover/ui/kit"
)

// remoteRow is one configured rclone remote: name (with an "active" badge),
// type (with an "unsupported" badge) and a delete button.
type remoteRow struct{ kit.Composite }

func newRemoteRow(r cloud.Remote, active bool, onDelete func()) *remoteRow {
	name := kit.HBox(widget.NewLabelWithStyle(r.Name, fyne.TextAlignLeading, fyne.TextStyle{Bold: true}))
	if active {
		name.Add(container.NewCenter(kit.Badge(i18n.L("Active"), kit.BadgeOK)))
	}
	typ := kit.HBox(kit.Small(r.Type))
	if !r.Supported {
		typ.Add(container.NewCenter(kit.Badge(i18n.L("Unsupported"), kit.BadgeWarn)))
	}
	del := widget.NewButtonWithIcon(i18n.L("Delete"), theme.DeleteIcon(), onDelete)
	row := &remoteRow{}
	row.Init(row, container.NewBorder(nil, nil, nil, del, container.NewGridWithColumns(2, name, typ)))
	return row
}
