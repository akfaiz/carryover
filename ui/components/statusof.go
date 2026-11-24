package components

import (
	"github.com/akfaiz/carryover/services"
	"github.com/akfaiz/carryover/ui/i18n"
	"github.com/akfaiz/carryover/ui/kit"
)

// StatusOf is the text and kind of StatusBadge.
func StatusOf(g services.GameView) (string, int) {
	switch {
	case g.Status.Running:
		return i18n.L("Running"), kit.BadgeInfo
	case !g.Ready:
		return i18n.L("Needs setup"), kit.BadgeWarn
	case g.Status.Conflict:
		return i18n.L("Conflict"), kit.BadgeErr
	case g.Status.LastError != "":
		return i18n.L("Sync error"), kit.BadgeErr
	case g.Status.LocalChanges:
		return i18n.L("Not uploaded"), kit.BadgeWarn
	case g.Status.LastSyncAt != "":
		return i18n.L("Synced"), kit.BadgeOK
	}
	return i18n.L("Not synced"), kit.BadgeNone
}
