package components

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"

	"github.com/akfaiz/carryover/ui/i18n"

	"github.com/akfaiz/carryover/services"
	"github.com/akfaiz/carryover/ui/kit"
)

// StatusBadge is the sync state badge of a game.
func StatusBadge(g services.GameView) fyne.CanvasObject { return kit.Badge(StatusOf(g)) }

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

// StatusIcon is a small coloured icon for a game's sync state (nil when there
// is nothing to say), for places too small for StatusBadge.
func StatusIcon(g services.GameView) fyne.Resource {
	switch {
	case g.Status.Running:
		return theme.MediaPlayIcon()
	case !g.Ready, g.Status.LocalChanges:
		return theme.NewWarningThemedResource(theme.WarningIcon())
	case g.Status.Conflict, g.Status.LastError != "":
		return theme.NewErrorThemedResource(theme.ErrorIcon())
	case g.Status.LastSyncAt != "":
		return theme.NewSuccessThemedResource(theme.ConfirmIcon())
	}
	return nil
}
