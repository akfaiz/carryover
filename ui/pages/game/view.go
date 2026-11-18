package game

import (
	"errors"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/lang"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/akfaiz/carryover/internal/cloudsync/syncer"
	"github.com/akfaiz/carryover/internal/core"
	"github.com/akfaiz/carryover/services"
	"github.com/akfaiz/carryover/ui/components"
	"github.com/akfaiz/carryover/ui/env"
	"github.com/akfaiz/carryover/ui/kit"
	"github.com/akfaiz/carryover/ui/store"
)

// Page is a game: its saves and how they compare with the cloud, backups,
// the Steam shortcut and the play session.
type Page struct {
	u env.Env
	*ViewModel

	wrapSection *kit.AccordionSection // to keep its open state across renders
	scroll      *container.Scroll
	prog        *kit.ProgressBar // current progress widget, updated in place
	progOf      string           // "session" or "busy"
	// backBtn is where a gamepad starts: A there only goes back, so an A
	// pressed once too often (the one that opened the game) never starts it.
	backBtn *widget.Button
}

// GameID is the game the page shows.
func (p *Page) GameID() string { return p.id }

// MoreMenu is the menu behind the page's "⋯" button.
func (p *Page) MoreMenu(g services.GameView, busy bool) *fyne.Menu { return p.moreMenu(g, busy) }

// SelectedProfile is the id of the profile whose saves are shown ("" is the
// game's own saves), for the game as the library lists it.
func (p *Page) SelectedProfile(g services.GameView) string {
	return p.selectedProfile(p.profiles(g)).id
}

func (p *Page) DefaultFocus() fyne.Focusable {
	if p.backBtn == nil {
		return nil
	}
	return p.backBtn
}

// New returns the page of the game with that id.
func New(u env.Env, id string) *Page {
	return &Page{u: u, ViewModel: newViewModel(id)}
}

func (p *Page) View() fyne.CanvasObject {
	p.scroll = kit.ScrollPage(widget.NewLabel(""))
	if g, ok := p.u.Store().Game(p.id); ok {
		p.initFrom(g)
	}
	kit.Async(func() (services.SteamInfo, error) { return p.u.Steam().Info(), nil }, func(info services.SteamInfo, _ error) {
		p.steam = &info
		if len(info.Users) > 0 {
			p.steamUser = info.Users[0].ID
		}
		p.render()
	})
	kit.Async(func() (string, error) { return p.u.Steam().WrapLaunchOptions(p.id) }, func(s string, err error) {
		if err == nil {
			p.wrapOpts = s
			p.render()
		}
	})
	p.render()
	return p.scroll
}

// syncNow is F5: sync every profile that can be synced here.
func (p *Page) SyncNow() {
	g, ok := p.u.Store().Game(p.id)
	if !ok || p.busy != "" || p.u.Store().Settings.Remote == "" || g.Overall.Running {
		return
	}
	p.syncAll(p.profiles(g))
}

func (p *Page) StateChanged(what store.Topic) {
	if what == store.Status && p.u.Store().Status[p.id].Phase != "sync" && p.busy == "" {
		p.cancelling = false // the session's sync phase ended
	}
	if what == store.Progress {
		if p.prog != nil {
			p.updateProgress()
		}
		return
	}
	p.render()
}

// sessionKey is the sync key of the play session the page shows: the profile
// that was started last.
func (p *Page) sessionKey() string {
	if st, ok := p.u.Store().Status[p.id]; ok && st.Key != "" {
		return st.Key
	}
	return p.id
}

func (p *Page) updateProgress() {
	switch p.progOf {
	case "session":
		st := p.u.Store().Status[p.id]
		if st.Phase == "playing" || st.Phase == "choose" {
			return
		}
		ev := p.u.Store().Progress[p.sessionKey()]
		p.prog.Set(st.Message, ev.Bytes, ev.TotalBytes)
	case "busy":
		ev := p.u.Store().Progress[p.key()]
		msg := ev.Message
		if msg == "" {
			msg = lang.L("Syncing…")
			switch p.busy {
			case "restore":
				msg = lang.L("Restoring backup…")
			case "backup":
				msg = lang.L("Backing up…")
			}
		}
		p.prog.Set(msg, ev.Bytes, ev.TotalBytes)
	}
}

// run marks the page busy while work runs.
func (p *Page) run(label string, work func() error, done func(error)) {
	p.busy = label
	if label == "sync" || label == "restore" || label == "backup" {
		p.u.Store().ClearProgress(p.key())
	}
	p.render()
	kit.Async0(work, func(err error) {
		p.busy = ""
		p.cancelling = false
		switch {
		case errors.Is(err, services.ErrSyncCancelled):
			p.u.Toast(lang.L("Sync cancelled. Files already transferred are kept; the next sync continues."), kit.ToastInfo)
			err = nil
			for _, st := range p.states {
				st.plan = nil
			}
		case err != nil:
			p.u.ToastErr(err)
		}
		if done != nil {
			done(err)
		}
		p.render()
	})
}

// transferCard is the progress of a sync, restore or backup with its Cancel
// button; it goes where the action was started.
func (p *Page) transferCard() fyne.CanvasObject {
	p.prog, p.progOf = kit.NewProgress(), "busy"
	p.updateProgress()
	return kit.Card("", kit.VBox(p.prog, kit.HBox(p.cancelButton(lang.L("Cancel"), func() bool { return p.u.Sync().Cancel(p.key()) }))))
}

func (p *Page) render() {
	u := p.u
	p.prog, p.progOf = nil, ""
	if p.wrapSection != nil {
		p.wrapOpen = p.wrapSection.Open()
	}
	orig, ok := u.Store().Game(p.id)
	if !ok {
		msg := lang.L("Loading…")
		if u.Store().Loaded {
			msg = lang.L("Game not found.")
		}
		kit.ReplaceScroll(p.scroll, kit.Stack(kit.PageHead(u.Back, ""), kit.Muted(msg)))
		return
	}
	g := orig
	g.Status = g.Overall // the game as a whole: its own saves and every emulator
	profs := p.profiles(orig)
	busy := p.busy != ""
	hasRemote := u.Store().Settings.Remote != ""
	canSync := canSync(profs)

	// Hero.
	p.backBtn = widget.NewButtonWithIcon(lang.L("Back"), theme.NavigateBackIcon(), u.Back)
	p.backBtn.Importance = widget.LowImportance
	info := kit.VBox(
		kit.WrapTitle(g.Name),
		kit.HBox(components.StatusBadge(g), kit.Small(lang.L("Last sync: {{.When}}", map[string]any{"When": components.TimeAgo(g.Status.LastSyncAt)}))),
	)
	devices := otherDevicesCard(g)
	if devices == nil && len(g.OtherDevices) > 0 {
		info.Add(otherDevicesLine(g))
	}
	if !g.Ready {
		info.Add(kit.Colored(g.SetupHint, widget.WarningImportance))
	}
	if g.Status.LastError != "" && !g.Status.Running {
		info.Add(kit.Colored(g.Status.LastError, widget.DangerImportance))
	}
	if g.Status.LocalChanges && !g.Status.Running {
		info.Add(kit.Colored(lang.L("The saves on this device changed after the last sync, so the game was probably played without Carryover. Use “Sync now” to upload them."), widget.WarningImportance))
	}
	actions := kit.HBox()
	steamLaunched := g.Device.LaunchMode == "steamwrap"
	switch {
	case g.Status.Running:
		b := kit.Primary(lang.L("● Running"), nil)
		b.Disable()
		actions.Add(b)
	case g.Ready && steamLaunched:
		b := kit.Primary(lang.L("Play in Steam"), func() { components.OpenSteam(u, orig) })
		kit.SetEnabled(b, services.SteamRunURL(orig) != "")
		actions.Add(b)
	case g.Ready:
		// A game that can be played in several ways asks which one first.
		b := widget.NewButtonWithIcon(lang.L("Play"), theme.MediaPlayIcon(), func() { p.run("play", func() error { return u.Sess().Play(p.id) }, nil) })
		b.Importance = widget.HighImportance
		kit.SetEnabled(b, !busy)
		actions.Add(b)
	default:
		actions.Add(kit.Primary(lang.L("Set up for this device"), func() { u.Navigate("/edit/" + p.id) }))
	}
	syncLabel := lang.L("Sync now")
	if p.busy == "sync" {
		syncLabel = lang.L("Syncing…")
	}
	syncBtn := widget.NewButtonWithIcon(syncLabel, theme.ViewRefreshIcon(), func() { p.syncAll(profs) })
	kit.SetEnabled(syncBtn, !busy && canSync && hasRemote && !g.Status.Running)
	actions.Add(syncBtn)
	actions.Add(widget.NewButtonWithIcon(lang.L("Edit"), theme.DocumentCreateIcon(), func() { u.Navigate("/edit/" + p.id) }))
	actions.Add(p.moreButton(orig, busy))
	info.Add(actions)
	if steamLaunched && g.Ready && services.SteamRunURL(orig) == "" {
		info.Add(kit.Muted(lang.L("This game is launched from Steam on this device (Proton). Use “Add to Steam” below, then start it from Steam.")))
	}
	if p.busy == "delete" {
		info.Add(kit.FixedWidth(480, kit.BusyLine(lang.L("Deleting…"))))
	}

	sessKey := p.sessionKey()
	st, hasSt := u.Store().Status[p.id]
	ev := u.Store().Progress[sessKey]
	if hasSt && (st.Phase != "done" || g.Status.Running) && st.Phase != "error" {
		box := kit.VBox(widget.NewLabelWithStyle(st.Message, fyne.TextAlignLeading, fyne.TextStyle{Bold: true}))
		if st.Phase != "playing" && st.Phase != "choose" {
			p.prog, p.progOf = kit.NewProgress(), "session"
			p.prog.Set("", ev.Bytes, ev.TotalBytes)
			box.Add(p.prog)
		}
		if st.Phase == "sync" {
			// Before the game starts this cancels the launch; afterwards the upload.
			label := lang.L("Cancel upload")
			if st.BeforePlay {
				label = lang.L("Cancel and don't play")
			}
			box.Add(kit.HBox(p.cancelButton(label, func() bool { return u.Sess().Cancel(sessKey) })))
		}
		info.Add(kit.FixedWidth(480, kit.Card("", box)))
	} else if p.busy == "sync" {
		info.Add(kit.FixedWidth(480, p.transferCard()))
	}

	// A smaller cover than the library's leaves room for the saves on a Steam
	// Deck's 800 px high screen.
	cover := kit.CoverView(kit.CoverSource{Path: g.CoverPath, Version: g.CoverVersion, Name: g.Name}, fyne.NewSize(120, 180))
	hero := container.NewBorder(nil, nil, container.NewVBox(cover), nil, container.NewPadded(info))
	// Back on its own row at the top left, where other pages have it.
	page := kit.Stack(kit.VBox(kit.HBox(p.backBtn), hero))

	// The saves and backups of one profile; a game with emulators switches
	// between them above both columns, so the columns start level.
	sel := p.selectedProfile(profs)
	if len(profs) > 1 {
		page.Add(p.profileSwitcher(profs, sel))
	}
	left := kit.Stack(p.savesCard(sel, busy, hasRemote, len(profs) > 1, g), p.backupsCard(sel, busy, hasRemote))
	right := kit.Stack(p.steamCard(orig, busy))
	if devices != nil {
		right.Add(devices)
	}
	page.Add(kit.NewCols(420, []float32{3, 2}, left, right))

	kit.ReplaceScroll(p.scroll, page)
}

// profileSwitcher is a row of buttons, one per way to play the game, each
// with an icon for its sync state; the selected one's saves are shown below.
func (p *Page) profileSwitcher(profs []profileEntry, sel profileEntry) fyne.CanvasObject {
	var opts []kit.ChipOption
	for _, pe := range profs {
		opts = append(opts, kit.ChipOption{Value: pe.id, Label: tagOf(pe), Icon: components.StatusIcon(pe.view)})
	}
	return kit.VBox(kit.Small(lang.L("Saves of")), kit.Chips(opts, sel.id, func(id string) {
		p.selected, p.chosen = id, true
		p.render()
	}))
}

// moreButton opens a menu with the game's less frequent actions.
func (p *Page) moreButton(g services.GameView, busy bool) fyne.CanvasObject {
	var b *widget.Button
	b = widget.NewButtonWithIcon("", theme.MoreHorizontalIcon(), func() {
		c := fyne.CurrentApp().Driver().CanvasForObject(b)
		widget.ShowPopUpMenuAtRelativePosition(p.moreMenu(g, busy), c, fyne.NewPos(0, b.Size().Height), b)
	})
	return b
}

// moreMenu lists the game's less frequent actions: open its save folders, and
// delete it.
func (p *Page) moreMenu(g services.GameView, busy bool) *fyne.Menu {
	var items []*fyne.MenuItem
	for _, sl := range g.Slots {
		if dir := g.ResolvedPaths[sl.Key]; dir != "" && g.PathErrors[sl.Key] == "" {
			label := lang.L("Open save folder")
			if len(g.Slots) > 1 {
				label = lang.L("Open save folder: {{.Label}}", map[string]any{"Label": components.LabelOf(sl.Label, sl.Key)})
			}
			items = append(items, fyne.NewMenuItem(label, func() { p.u.OpenFolder(dir) }))
		}
	}
	if len(items) > 0 {
		items = append(items, fyne.NewMenuItemSeparator())
	}
	del := fyne.NewMenuItem(lang.L("Delete from library…"), p.askDelete)
	del.Icon = theme.DeleteIcon()
	del.Disabled = busy || g.Overall.Running
	return fyne.NewMenu("", append(items, del)...)
}

// askDelete asks whether to move the game to the trash or delete it with its
// cloud saves, then does it.
func (p *Page) askDelete() {
	g, ok := p.u.Store().Game(p.id)
	if !ok {
		return
	}
	components.ConfirmDanger(p.u, components.ConfirmOpts{
		Title:    lang.L("Delete game"),
		Message:  lang.L("Move {{.Game}} to the trash on all devices? You can restore it from the Trash page. Cloud saves and backups are kept.", map[string]any{"Game": g.Name}),
		Check:    lang.L("Also delete its cloud saves and backups"),
		Checked:  lang.L("Delete {{.Game}} permanently on all devices, with its cloud saves and backups (for every emulator too)? It can't be restored.", map[string]any{"Game": g.Name}),
		KeptHint: lang.L("The cloud saves stay until you delete them. Once it is gone for good, they are listed under Trash → Leftover cloud saves."),
		OnConfirm: func(saves bool) {
			del := p.u.Lib().Delete
			if saves {
				del = p.u.Lib().DeleteWithSaves
			}
			p.run("delete", func() error { return del(p.id) }, func(err error) {
				if err == nil {
					p.u.Replace("/") // the game's page is gone: Back shouldn't return to it
				}
			})
		},
	})
}

// cancelButton stops a running sync; it shows "Cancelling…" until it stops.
func (p *Page) cancelButton(label string, cancel func() bool) *widget.Button {
	if p.cancelling {
		b := widget.NewButton(lang.L("Cancelling…"), nil)
		b.Disable()
		return b
	}
	return widget.NewButtonWithIcon(label, theme.CancelIcon(), func() {
		if cancel() {
			p.cancelling = true
			p.render()
		}
	})
}

// savesCard shows the save locations of one profile and compares them with the
// cloud. With several profiles it also says how this one is doing, unless
// that reads exactly like the page header's line for the whole game.
func (p *Page) savesCard(pe profileEntry, busy, hasRemote, several bool, whole services.GameView) fyne.CanvasObject {
	u := p.u
	g := pe.view
	st := p.state(pe.id)
	cmpLabel := lang.L("Compare with cloud")
	if p.busyFor("plan", pe.id) {
		cmpLabel = lang.L("Checking…")
	}
	cmp := widget.NewButton(cmpLabel, func() { p.check(pe.id) })
	kit.SetEnabled(cmp, !busy && g.Ready && hasRemote)
	rows := container.NewVBox()
	for _, sl := range g.Slots {
		rows.Add(saveLocationRow(u, components.LabelOf(sl.Label, sl.Key), g.ResolvedPaths[sl.Key], g.PathErrors[sl.Key]))
	}
	c := kit.VBox(kit.CardHead(lang.L("Saves"), cmp))
	if several {
		text, _ := components.StatusOf(g)
		wholeText, _ := components.StatusOf(whole)
		if text != wholeText || components.TimeAgo(g.Status.LastSyncAt) != components.TimeAgo(whole.Status.LastSyncAt) {
			c.Add(kit.HBox(components.StatusBadge(g), kit.Small(lang.L("Last sync: {{.When}}", map[string]any{"When": components.TimeAgo(g.Status.LastSyncAt)}))))
		}
		if !g.Ready {
			c.Add(kit.Colored(g.SetupHint, widget.WarningImportance))
		}
		if g.Status.LastError != "" && !g.Status.Running {
			c.Add(kit.Colored(g.Status.LastError, widget.DangerImportance))
		}
		if g.Status.LocalChanges && !g.Status.Running {
			c.Add(kit.Colored(lang.L("These saves changed on this device after the last sync. Use “Sync now” to upload them."), widget.WarningImportance))
		}
	}
	c.Add(rows)
	if p.busyFor("plan", pe.id) {
		c.Add(kit.BusyLine(lang.L("Comparing with the cloud…")))
	}
	if pl := st.plan; pl != nil {
		c.Add(widget.NewSeparator())
		c.Add(newPlanView(pl, pl.Action == "conflict" || g.Status.Conflict, func(resolution string) { p.doSync(pe.id, resolution) }))
	} else if g.Status.Conflict {
		c.Add(widget.NewSeparator())
		c.Add(kit.Colored(lang.L("Unresolved save conflict. Compare with cloud to choose a version."), widget.WarningImportance))
	}
	return kit.Card("", c)
}

// check compares a profile's saves with the cloud.
func (p *Page) check(prof string) {
	key := core.SyncKey(p.id, prof)
	p.setActive(prof)
	var plan services.PlanView
	p.run("plan", func() error {
		var err error
		plan, err = p.u.Sync().Plan(key)
		return err
	}, func(err error) {
		if err == nil {
			p.state(prof).plan = &plan
		}
	})
}

// syncAll syncs every profile that can be synced on this device, one after the
// other: the game's own saves and each emulator's.
func (p *Page) syncAll(profs []profileEntry) {
	ids := syncTargets(profs)
	if len(ids) == 0 {
		return
	}
	p.setActive(ids[0])
	var done, actions []string
	p.run("sync", func() error {
		for _, id := range ids {
			p.setActive(id) // the progress shown follows the profile being synced
			a, err := p.u.Sync().Sync(core.SyncKey(p.id, id), "")
			if err != nil {
				return err
			}
			done, actions = append(done, id), append(actions, a)
		}
		return nil
	}, func(err error) {
		if err == nil {
			p.synced(done, actions)
		}
	})
}

// doSync syncs one profile; resolution settles a conflict ("local" or "cloud").
func (p *Page) doSync(prof, resolution string) {
	key := core.SyncKey(p.id, prof)
	p.setActive(prof)
	var action string
	p.run("sync", func() error {
		var err error
		action, err = p.u.Sync().Sync(key, resolution)
		return err
	}, func(err error) {
		if err == nil {
			p.synced([]string{prof}, []string{action})
		}
	})
}

// synced tells how the syncs of the profiles went (actions[i] is what profs[i]
// did) and drops what they made out of date: the comparison and the backup
// lists. A conflict is compared with the cloud at once, so the choice of which
// saves to keep is right there.
func (p *Page) synced(profs, actions []string) {
	kind, conflict := classifySync(actions)
	switch kind {
	case syncConflict:
		p.u.Toast(lang.L("Save conflict: choose which version to keep below"), kit.ToastErr)
	case syncBoth:
		p.u.Toast(lang.L("Synced saves"), kit.ToastOK)
	case syncPushed:
		p.u.Toast(lang.L("Uploaded saves"), kit.ToastOK)
	case syncPulled:
		p.u.Toast(lang.L("Downloaded saves"), kit.ToastOK)
	default:
		p.u.Toast(lang.L("Already up to date"), kit.ToastOK)
	}
	for _, prof := range profs {
		p.state(prof).plan = nil
	}
	p.loadBackups(profs...)
	if conflict >= 0 {
		prof := profs[conflict]
		p.selected, p.chosen = prof, true
		p.check(prof)
	}
}

// backupsCard lists and takes the backups of one profile's cloud save. The
// list loads by itself the first time the card is shown.
func (p *Page) backupsCard(pe profileEntry, busy, hasRemote bool) fyne.CanvasObject {
	g := pe.view
	st := p.state(pe.id)
	if hasRemote && !st.backupsLoaded && !st.backupsLoading {
		p.loadBackups(pe.id)
	}
	refresh := widget.NewButtonWithIcon("", theme.ViewRefreshIcon(), func() { p.loadBackups(pe.id) })
	kit.SetEnabled(refresh, hasRemote && !st.backupsLoading)
	nowLabel := lang.L("Back up now")
	if p.busyFor("backup", pe.id) {
		nowLabel = lang.L("Backing up…")
	}
	nowBtn := widget.NewButton(nowLabel, func() { p.backupNow(pe.id) })
	kit.SetEnabled(nowBtn, !busy && hasRemote && g.Ready && !g.Status.Running)

	summary := lang.L("Cloud backups are off. A copy of this device's saves is kept here before a download replaces them.")
	details := []string{lang.L("Cloud backups are off. Before a download replaces this device's saves, a copy is kept on this device (the last {{.Count}}). Turn on cloud backups in Settings to keep backups in the cloud.",
		map[string]any{"Count": syncer.SafetyKeep})}
	if n := p.u.Store().Settings.BackupRetention; n > 0 {
		summary = lang.N("Cloud backups are on: the last {{.Count}} backup is kept.", n, map[string]any{"Count": n})
		details = []string{lang.N("A backup is taken in the cloud whenever saves are about to be overwritten (keeping the last {{.Count}} backup).", n,
			map[string]any{"Count": n})}
	}
	details = append(details, lang.L("Backups you make with “Back up now” are pinned. Pinned backups are never deleted automatically."))
	c := kit.VBox(kit.CardHead(lang.L("Backups"), nowBtn, refresh), components.Help(p.u, summary, details...))
	if g.SafetyDir != "" {
		dir := g.SafetyDir
		c.Add(kit.HBox(widget.NewButtonWithIcon(lang.L("Open copies on this device"), theme.FolderOpenIcon(), func() { p.u.OpenFolder(dir) })))
	}
	if (p.busy == "restore" || p.busy == "backup") && p.activeProfile() == pe.id {
		c.Add(p.transferCard())
	}
	if p.busyFor("pin", pe.id) {
		c.Add(kit.BusyLine(lang.L("Updating backups…")))
	}
	switch {
	case !hasRemote:
		c.Add(kit.Muted(lang.L("Set up cloud storage to keep backups in the cloud.")))
	case st.backupsLoading && !st.backupsLoaded:
		c.Add(kit.BusyLine(lang.L("Loading backups…")))
	case st.backupsErr != "":
		c.Add(kit.Colored(st.backupsErr, widget.DangerImportance))
	case len(st.backups) == 0:
		c.Add(kit.Muted(lang.L("No backups yet.")))
	}
	for _, b := range st.backups {
		c.Add(backupRow(b, p.busyFor("restore", pe.id), !busy && g.Ready, !busy,
			func() { p.restore(pe.id, b) }, func() { p.setPinned(pe.id, b) }, func() { p.deleteBackup(pe.id, b) }))
	}
	return kit.Card("", c)
}

// loadBackups reads the backup lists of the profiles in the background: it
// doesn't make the page busy, so Play and Sync stay available meanwhile.
func (p *Page) loadBackups(profs ...string) {
	if p.u.Store().Settings.Remote == "" {
		return
	}
	for _, prof := range profs {
		st := p.state(prof)
		if st.backupsLoading {
			continue
		}
		st.backupsLoading = true
		key := core.SyncKey(p.id, prof)
		kit.Async(func() ([]syncer.Backup, error) { return p.u.Sync().Backups(key) }, func(list []syncer.Backup, err error) {
			st.backupsLoading, st.backupsLoaded, st.backupsErr = false, true, ""
			if err != nil {
				st.backupsErr = err.Error()
			} else {
				st.backups = list
			}
			p.render()
		})
	}
}

// backupNow stores a pinned backup of this device's saves.
func (p *Page) backupNow(prof string) {
	key := core.SyncKey(p.id, prof)
	p.setActive(prof)
	p.run("backup", func() error { return p.u.Sync().BackupNow(key) }, func(err error) {
		if err == nil {
			p.u.Toast(lang.L("Backup saved and pinned"), kit.ToastOK)
			p.loadBackups(prof)
		}
	})
}

// deleteBackup asks, then permanently deletes a backup for every device.
func (p *Page) deleteBackup(prof string, b syncer.Backup) {
	msg := lang.L("Delete the backup from {{.When}}? It is shared with all your devices and can't be recovered.", map[string]any{"When": components.FmtDate(b.Time)})
	if b.Pinned {
		msg = lang.L("This backup is pinned. Delete the backup from {{.When}} anyway? It is shared with all your devices and can't be recovered.", map[string]any{"When": components.FmtDate(b.Time)})
	}
	components.ConfirmDanger(p.u, components.ConfirmOpts{Title: lang.L("Delete backup"), Message: msg, OnConfirm: func(bool) {
		key := core.SyncKey(p.id, prof)
		p.setActive(prof)
		p.run("pin", func() error { return p.u.Sync().DeleteBackup(key, b.Name) }, func(err error) {
			if err == nil {
				p.u.Toast(lang.L("Backup deleted"), kit.ToastOK)
				p.loadBackups(prof)
				p.u.Store().Refresh() // the last copy on this device may be gone
			}
		})
	}})
}

func (p *Page) setPinned(prof string, b syncer.Backup) {
	key := core.SyncKey(p.id, prof)
	p.setActive(prof)
	p.run("pin", func() error { return p.u.Sync().SetBackupPinned(key, b.Name, !b.Pinned) }, func(err error) {
		if err == nil {
			p.loadBackups(prof)
		}
	})
}

// restore asks, then puts a backup back on this device and in the cloud.
func (p *Page) restore(prof string, b syncer.Backup) {
	components.ConfirmDanger(p.u, components.ConfirmOpts{
		Title:   lang.L("Restore backup"),
		Message: lang.L("Replace the saves on this device and in the cloud with the backup from {{.When}}? The saves on this device are backed up first.", map[string]any{"When": components.FmtDate(b.Time)}),
		Action:  lang.L("Restore"),
		OnConfirm: func(bool) {
			key := core.SyncKey(p.id, prof)
			p.setActive(prof)
			p.run("restore", func() error { return p.u.Sync().RestoreBackup(key, b.Name) }, func(err error) {
				if err == nil {
					p.u.Toast(lang.L("Backup restored and uploaded"), kit.ToastOK)
					p.loadBackups(prof)
				}
			})
		},
	})
}

// steamCard is the Steam shortcut form. A game with emulators gets one
// shortcut for the whole game by default; it asks which emulator to use when it
// starts. The shortcut of one emulator is the other choice.
func (p *Page) steamCard(orig services.GameView, busy bool) fyne.CanvasObject {
	u := p.u
	sc := p.steamChoice(orig)
	key, form, target, asks, hideMode := sc.key, sc.form, sc.target, sc.asks, sc.hideMode
	c := kit.VBox(newSteamForm(steamFormOpts{
		Game: form, Info: p.steam, User: p.steamUser, Mode: p.steamMode, Busy: busy,
		Profiles: orig.Emulators, Target: target, AsksAtLaunch: asks, HideMode: hideMode,
		Expanded: p.steamExpanded,
		OnExpand: func() { p.steamExpanded = true; p.render() },
		OnTarget: func(v string) { p.steamTarget = v; p.render() },
		OnUser:   func(v string) { p.steamUser = v },
		OnMode:   func(v string) { p.steamMode = v; p.render() },
		OnAdd: func() {
			var res core.AddToSteamResult
			p.run("steam", func() error {
				var err error
				mode := p.steamMode
				if hideMode {
					mode = "direct"
				}
				res, err = u.Steam().AddToSteam(key, p.steamUser, mode)
				return err
			}, func(err error) {
				if err != nil {
					return
				}
				p.steamExpanded = false
				if res.SteamRunning {
					u.Toast(lang.L("Added to Steam. Restart Steam to see it."), kit.ToastOK)
				} else {
					u.Toast(lang.L("Added to Steam"), kit.ToastOK)
				}
			})
		},
		OnRemove: func() {
			var n int
			p.run("steam", func() error {
				var err error
				n, err = u.Steam().RemoveFromSteam(key, p.steamUser)
				return err
			}, func(err error) {
				if err == nil && n > 0 {
					u.Toast(lang.L("Removed from Steam"), kit.ToastOK)
				} else if err == nil {
					u.Toast(lang.L("No shortcut found"), kit.ToastOK)
				}
			})
		},
	}))
	if p.busy == "steam" {
		c.Add(kit.BusyLine(lang.L("Updating Steam…")))
	}
	if p.wrapOpts != "" {
		p.wrapSection = kit.NewAccordionSection(lang.L("Already own it on Steam?"), kit.VBox(
			kit.Muted(lang.L("Paste this into the game's Steam launch options to sync around Steam's own launch:")),
			components.CopyField(u, p.wrapOpts)), p.wrapOpen)
		c.Add(p.wrapSection)
	}
	return kit.Card(lang.L("Steam"), c)
}

// Redraw draws the page again from its state (env.Redrawable).
func (p *Page) Redraw() { p.render() }

// tagOf names a profile on the switcher: the emulator, or the game's own saves.
func tagOf(pe profileEntry) string {
	if pe.noEmu {
		return lang.L("Without an emulator")
	}
	return pe.tag
}
