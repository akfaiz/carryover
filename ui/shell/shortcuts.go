package shell

import (
	"errors"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/lang"

	"github.com/akfaiz/carryover/services"
	"github.com/akfaiz/carryover/ui/env"
	"github.com/akfaiz/carryover/ui/kit"
)

// addShortcuts sets the main window's keyboard shortcuts: Ctrl+F searches,
// Ctrl+N adds a game, F5 syncs (the page's game, or everything on the
// library), F11 switches full screen and Esc goes back or closes the dialog.
func (u *Shell) addShortcuts() {
	c := u.win.Canvas()
	c.AddShortcut(&desktop.CustomShortcut{KeyName: fyne.KeyF, Modifier: fyne.KeyModifierShortcutDefault}, func(fyne.Shortcut) {
		if s, ok := u.page.(env.Searchable); ok {
			s.FocusSearch()
		}
	})
	c.AddShortcut(&desktop.CustomShortcut{KeyName: fyne.KeyN, Modifier: fyne.KeyModifierShortcutDefault}, func(fyne.Shortcut) {
		if len(c.Overlays().List()) == 0 {
			u.cfg.AddGame(u)
		}
	})
	// Keys reach this only while nothing is focused. The arrow keys and Enter
	// then pick a control to start from, like a gamepad's D-pad (see
	// newNavigator); Escape goes back or closes the dialog.
	c.SetOnTypedKey(func(e *fyne.KeyEvent) {
		if len(c.Overlays().List()) > 0 {
			u.nav.HandleKey(e)
			return
		}
		switch e.Name {
		case fyne.KeyEscape:
			if u.route != "/" {
				u.back()
			}
		case fyne.KeyF5:
			if s, ok := u.page.(env.Syncable); ok {
				s.SyncNow()
			}
		case fyne.KeyF11:
			u.toggleFullScreen()
		default:
			u.nav.HandleKey(e)
		}
	})
}

// syncAll syncs every game that is set up on this device; called again while
// it runs, it cancels. It belongs to the window, not to the library page, so
// it keeps running (and is shown in the sidebar) on any page.
func (u *Shell) syncAll() {
	if u.store.SyncAll.Running {
		if !u.store.SyncAll.Cancelling && u.sync.Cancel("") {
			u.store.SyncAllCancelling()
		}
		return
	}
	u.store.BeginSyncAll()
	kit.Async(u.sync.SyncAll, func(res []services.SyncResult, err error) {
		u.store.EndSyncAll()
		defer u.store.Refresh()
		if errors.Is(err, services.ErrSyncCancelled) {
			u.toast(lang.N("Sync cancelled after {{.Count}} game", len(res), map[string]any{"Count": len(res)}), kit.ToastInfo)
			return
		}
		if services.IsOffline(err) {
			u.toast(lang.L("Offline — nothing synced"), kit.ToastInfo)
			return
		}
		if err != nil {
			u.toastErr(err)
			return
		}
		var problems []services.SyncResult
		for _, r := range res {
			if r.Error != "" || r.Action == "conflict" {
				problems = append(problems, r)
			}
		}
		if len(problems) > 0 {
			u.cfg.SyncProblems(u, len(res), problems)
			return
		}
		u.toast(lang.N("Synced {{.Count}} game", len(res), map[string]any{"Count": len(res)}), kit.ToastOK)
	})
}
