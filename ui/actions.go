package ui

import (
	"errors"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/lang"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/akfaiz/carryover/services"
)

// addShortcuts sets the main window's keyboard shortcuts: Ctrl+F searches,
// Ctrl+N adds a game, F5 syncs (the page's game, or everything on the
// library) and Esc goes back.
func (u *UI) addShortcuts() {
	c := u.win.Canvas()
	c.AddShortcut(&desktop.CustomShortcut{KeyName: fyne.KeyF, Modifier: fyne.KeyModifierShortcutDefault}, func(fyne.Shortcut) {
		if s, ok := u.page.(searchable); ok {
			s.focusSearch()
		}
	})
	c.AddShortcut(&desktop.CustomShortcut{KeyName: fyne.KeyN, Modifier: fyne.KeyModifierShortcutDefault}, func(fyne.Shortcut) {
		if len(c.Overlays().List()) == 0 {
			showAddGame(u)
		}
	})
	c.SetOnTypedKey(func(e *fyne.KeyEvent) {
		if len(c.Overlays().List()) > 0 {
			return // a dialog handles its own keys
		}
		switch e.Name {
		case fyne.KeyEscape:
			if u.route != "/" {
				u.back()
			}
		case fyne.KeyF5:
			if s, ok := u.page.(syncable); ok {
				s.syncNow()
			}
		}
	})
}

// syncAll syncs every game that is set up on this device; called again while
// it runs, it cancels. It belongs to the window, not to the library page, so
// it keeps running (and is shown in the sidebar) on any page.
func (u *UI) syncAll() {
	if u.syncingAll {
		if !u.cancellingAll && u.sync.Cancel("") {
			u.cancellingAll = true
			u.notify(changeProgress)
		}
		return
	}
	u.syncingAll, u.syncAllProg = true, services.SyncAllProgress{}
	u.renderActivity()
	u.notify(changeProgress)
	async(u.sync.SyncAll, func(res []services.SyncResult, err error) {
		u.syncingAll, u.cancellingAll = false, false
		u.renderActivity()
		u.notify(changeProgress)
		defer u.refresh()
		if errors.Is(err, services.ErrSyncCancelled) {
			u.toast(lang.N("Sync cancelled after {{.Count}} game", len(res), map[string]any{"Count": len(res)}), toastInfo)
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
			showSyncProblems(u, len(res), problems)
			return
		}
		u.toast(lang.N("Synced {{.Count}} game", len(res), map[string]any{"Count": len(res)}), toastOK)
	})
}

// showSyncProblems lists the games a "Sync all" could not sync, each opening
// its page, so a conflict or an error isn't lost in a toast.
func showSyncProblems(u *UI, total int, problems []services.SyncResult) {
	var d dialogHider
	rows := vbox()
	for _, r := range problems {
		state := badge(lang.L("sync error"), badgeErr)
		detail := r.Error
		if r.Action == "conflict" {
			state = badge(lang.L("conflict"), badgeErr)
			detail = lang.L("Changed on this device and in the cloud. Open the game to choose which saves to keep.")
		}
		id := r.GameID
		rows.Add(newTappable(vbox(
			hbox(widget.NewLabelWithStyle(r.Name, fyne.TextAlignLeading, fyne.TextStyle{Bold: true}), container.NewCenter(state)),
			muted(detail),
		), func() {
			d.Hide()
			u.navigate("/game/" + id)
		}))
	}
	scroll := container.NewVScroll(rows)
	scroll.SetMinSize(fyne.NewSize(0, min(320, rows.MinSize().Height)))
	body := vbox(
		muted(lang.N("{{.Count}} game synced.", total-len(problems), map[string]any{"Count": total - len(problems)})+" "+
			lang.L("These need a look — click one to open it:")),
		scroll,
		hbox(layout.NewSpacer(), widget.NewButton(lang.L("Close"), func() { d.Hide() }),
			primary(lang.L("Show them in the library"), func() {
				d.Hide()
				u.setLibraryFilter(filterAttention)
				u.navigate("/")
			})),
	)
	d = showCustom(u, lang.N("{{.Count}} game needs attention", len(problems), map[string]any{"Count": len(problems)}), body, 560)
}

// showAddGame asks what kind of game to add: one installed by Steam, another
// PC game, or a game played through an emulator. Each opens the editor set up
// for it.
func showAddGame(u *UI) {
	var d dialogHider
	pick := func(f func()) func() { return func() { d.Hide(); f() } }
	tile := func(name string, icon fyne.Resource, f func()) fyne.CanvasObject {
		return container.New(layout.NewGridWrapLayout(fyne.NewSize(choiceTileSize+16, choiceTileSize)),
			newChoiceTile(name, icon, false, pick(f)))
	}
	body := vbox(
		hbox(
			tile(lang.L("From Steam"), theme.DownloadIcon(), func() { showSteamImport(u) }),
			tile(lang.L("PC game"), theme.ComputerIcon(), func() { u.navigate("/new") }),
			tile(lang.L("Emulated game"), theme.MediaPlayIcon(), func() { u.navigate("/new/emulator") }),
		),
		muted(lang.L("From Steam: a game Steam installed on this device. PC game: any other game, started from its executable or through Heroic. Emulated game: a game file you play through an emulator such as Eden or Ryujinx.")),
		hbox(layout.NewSpacer(), widget.NewButton(lang.L("Cancel"), func() { d.Hide() })),
	)
	d = showCustom(u, lang.L("Add a game"), body, 500)
}
