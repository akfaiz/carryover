package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/lang"
	"fyne.io/fyne/v2/theme"

	"github.com/akfaiz/carryover/internal/ui/kit"
	"github.com/akfaiz/carryover/services"
)

// statusBadge is the sync state badge of a game.
func statusBadge(g services.GameView) fyne.CanvasObject { return kit.Badge(statusOf(g)) }

// statusOf is the text and kind of statusBadge.
func statusOf(g services.GameView) (string, int) {
	switch {
	case g.Status.Running:
		return lang.L("Running"), kit.BadgeInfo
	case !g.Ready:
		return lang.L("Needs setup"), kit.BadgeWarn
	case g.Status.Conflict:
		return lang.L("Conflict"), kit.BadgeErr
	case g.Status.LastError != "":
		return lang.L("Sync error"), kit.BadgeErr
	case g.Status.LocalChanges:
		return lang.L("Not uploaded"), kit.BadgeWarn
	case g.Status.LastSyncAt != "":
		return lang.L("Synced"), kit.BadgeOK
	}
	return lang.L("Not synced"), kit.BadgeNone
}

// statusIcon is a small coloured icon for a game's sync state (nil when there
// is nothing to say), for places too small for statusBadge.
func statusIcon(g services.GameView) fyne.Resource {
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

// needsAttention reports whether the game's state asks the user to do
// something: set it up here, settle a conflict, look at an error or upload
// saves changed outside Carryover.
func needsAttention(g services.GameView) bool {
	st := g.Overall
	if st.Running {
		return false
	}
	return !g.Ready || st.Conflict || st.LastError != "" || st.LocalChanges
}
