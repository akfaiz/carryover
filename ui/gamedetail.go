package ui

import (
	"errors"
	"slices"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/lang"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/akfaiz/carryover/internal/cloudsync/syncer"
	"github.com/akfaiz/carryover/internal/core"
	"github.com/akfaiz/carryover/services"
)

type gameDetail struct {
	u  *UI
	id string
	// profile is the emulator whose saves the page shows ("" = the game's own
	// launch config); profileSet: the user (or the default) has chosen one.
	profile    string
	profileSet bool

	busy          string
	plan          *services.PlanView
	backups       []syncer.Backup
	backupsLoaded bool
	steam         *services.SteamInfo
	steamUser     string
	steamMode     string
	steamTarget   string // "game" (default for a game with emulators) or "profile"
	wrapOpts      string
	confirmDelete bool
	deleteSaves   bool // the delete also removes the cloud saves
	cancelling    bool // Cancel pressed; waiting for the sync to stop
	wrapOpen      bool
	wrapSection   *accordionSection // to keep its open state across renders

	scroll *container.Scroll
	prog   *progressBar // current progress widget, updated in place
	progOf string       // "session" or "busy"
}

func newGameDetail(u *UI, id string) *gameDetail {
	return &gameDetail{u: u, id: id, steamMode: "direct"}
}

// key is what the services address the shown profile by (core.SyncKey).
func (p *gameDetail) key() string { return core.SyncKey(p.id, p.profile) }

// chooseProfile picks the profile to show the first time: the one played last
// on this device, else the game's own, else its first emulator.
func (p *gameDetail) chooseProfile(g services.GameView) {
	if p.profileSet {
		return
	}
	p.profileSet = true
	has := func(id string) bool {
		return slices.ContainsFunc(g.Emulators, func(pv services.ProfileView) bool { return pv.EmulatorID == id })
	}
	switch {
	case g.LastEmulator != "" && has(g.LastEmulator):
		p.profile = g.LastEmulator
	case len(g.Slots) == 0 && len(g.Emulators) > 0:
		p.profile = g.Emulators[0].EmulatorID
	}
}

// profileView presents the game as seen through one emulator profile: its save
// locations, readiness, status and Steam shortcut replace the game's own.
func profileView(g services.GameView, emuID string) services.GameView {
	if emuID == "" {
		return g
	}
	for _, pv := range g.Emulators {
		if pv.EmulatorID != emuID {
			continue
		}
		g.Slots, g.ResolvedPaths, g.PathErrors = pv.Slots, pv.ResolvedPaths, pv.PathErrors
		g.Ready, g.SetupHint, g.Status, g.SafetyDir = pv.Ready, pv.SetupHint, pv.Status, pv.SafetyDir
		g.Device.LaunchMode, g.Device.SteamAppID = "direct", pv.SteamAppID
	}
	return g
}

func (p *gameDetail) view() fyne.CanvasObject {
	p.scroll = scrollPage(widget.NewLabel(""))
	if g, ok := p.u.game(p.id); ok {
		switch g.Device.LaunchMode {
		case "steamwrap":
			p.steamMode = "steamwrap"
			p.wrapOpen = g.StoreAppID != ""
		case "heroic":
			p.steamMode = "heroic"
		}
	}
	async(func() (services.SteamInfo, error) { return p.u.steam.Info(), nil }, func(info services.SteamInfo, _ error) {
		p.steam = &info
		if len(info.Users) > 0 {
			p.steamUser = info.Users[0].ID
		}
		p.render()
	})
	async(func() (string, error) { return p.u.steam.WrapLaunchOptions(p.id) }, func(s string, err error) {
		if err == nil {
			p.wrapOpts = s
			p.render()
		}
	})
	p.render()
	return p.scroll
}

func (p *gameDetail) stateChanged(what change) {
	if what == changeStatus && p.u.status[p.key()].Phase != "sync" && p.busy == "" {
		p.cancelling = false // the session's sync phase ended
	}
	if what == changeProgress && p.prog != nil {
		p.updateProgress()
		return
	}
	p.render()
}

func (p *gameDetail) updateProgress() {
	ev := p.u.progress[p.key()]
	switch p.progOf {
	case "session":
		st := p.u.status[p.key()]
		if st.Phase == "playing" {
			return
		}
		p.prog.set(st.Message, ev.Bytes, ev.TotalBytes)
	case "busy":
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
		p.prog.set(msg, ev.Bytes, ev.TotalBytes)
	}
}

// run marks the page busy while work runs.
func (p *gameDetail) run(label string, work func() error, done func(error)) {
	p.busy = label
	if label == "sync" || label == "restore" || label == "backup" {
		delete(p.u.progress, p.key())
	}
	p.render()
	async0(work, func(err error) {
		p.busy = ""
		p.cancelling = false
		switch {
		case errors.Is(err, services.ErrSyncCancelled):
			p.u.toast(lang.L("Sync cancelled. Files already transferred are kept; the next sync continues."), toastInfo)
			err = nil
			p.plan = nil
		case err != nil:
			p.u.toastErr(err)
		}
		if done != nil {
			done(err)
		}
		p.render()
	})
}

func (p *gameDetail) render() {
	u := p.u
	p.prog, p.progOf = nil, ""
	if p.wrapSection != nil {
		p.wrapOpen = p.wrapSection.Open()
	}
	g, ok := u.game(p.id)
	if !ok {
		msg := lang.L("Loading…")
		if u.loaded {
			msg = lang.L("Game not found.")
		}
		replaceScroll(p.scroll, stack(pageHead(u.back, ""), muted(msg)))
		return
	}
	p.chooseProfile(g)
	orig := g // the game itself, for the profile selector
	g = profileView(g, p.profile)
	busy := p.busy != ""
	hasRemote := u.settings.Remote != ""

	// Hero.
	info := vbox(
		hbox(lowButton(lang.L("← Library"), func() { u.navigate("/") })),
		wrapTitle(g.Name),
		hbox(statusBadge(g), small(lang.L("Last sync: {{.When}}", map[string]any{"When": timeAgo(g.Status.LastSyncAt)}))),
	)
	if sel := p.profileSelector(orig); sel != nil {
		info.Add(sel)
	}
	if !g.Ready {
		info.Add(colored(g.SetupHint, widget.WarningImportance))
	}
	if g.Status.LastError != "" && !g.Status.Running {
		info.Add(colored(g.Status.LastError, widget.DangerImportance))
	}
	if g.Status.LocalChanges && !g.Status.Running {
		info.Add(colored(lang.L("The saves on this device changed after the last sync, so the game was probably played without Carryover. Use “Sync now” to upload them."), widget.WarningImportance))
	}
	actions := hbox()
	switch {
	case g.Status.Running:
		b := primary(lang.L("● Running"), nil)
		b.Disable()
		actions.Add(b)
	case g.Ready && g.Device.LaunchMode != "steamwrap":
		b := primary(lang.L("▶ Play"), func() { p.run("play", func() error { return u.sess.Play(p.key()) }, nil) })
		setEnabled(b, !busy)
		actions.Add(b)
	case !g.Ready:
		actions.Add(primary(lang.L("Set up for this device"), func() { u.navigate("/edit/" + p.id) }))
	}
	syncLabel := lang.L("Sync now")
	if p.busy == "sync" {
		syncLabel = lang.L("Syncing…")
	}
	syncBtn := widget.NewButtonWithIcon(syncLabel, theme.ViewRefreshIcon(), func() { p.doSync("") })
	setEnabled(syncBtn, !busy && g.Ready && hasRemote && !g.Status.Running)
	actions.Add(syncBtn)
	actions.Add(widget.NewButtonWithIcon(lang.L("Edit"), theme.DocumentCreateIcon(), func() { u.navigate("/edit/" + p.id) }))
	info.Add(actions)
	if g.Device.LaunchMode == "steamwrap" && g.Ready {
		info.Add(muted(lang.L("This game is launched from Steam on this device (Proton). Use “Add to Steam” below, then start it from Steam.")))
	}

	st, hasSt := u.status[p.key()]
	ev := u.progress[p.key()]
	if hasSt && (st.Phase != "done" || g.Status.Running) && st.Phase != "error" {
		box := vbox(widget.NewLabelWithStyle(st.Message, fyne.TextAlignLeading, fyne.TextStyle{Bold: true}))
		if st.Phase != "playing" {
			p.prog, p.progOf = newProgress(), "session"
			p.prog.set("", ev.Bytes, ev.TotalBytes)
			box.Add(p.prog)
		}
		if st.Phase == "sync" {
			// Before the game starts this cancels the launch; afterwards the upload.
			label := lang.L("Cancel upload")
			if st.Message == "Checking cloud saves…" {
				label = lang.L("Cancel and don't play")
			}
			box.Add(hbox(p.cancelButton(label, func() bool { return u.sess.Cancel(p.key()) })))
		}
		info.Add(fixedWidth(480, card("", box)))
	} else if p.busy == "sync" || p.busy == "restore" || p.busy == "backup" {
		p.prog, p.progOf = newProgress(), "busy"
		p.updateProgress()
		info.Add(fixedWidth(480, card("", vbox(p.prog, hbox(p.cancelButton(lang.L("Cancel"), func() bool { return u.sync.Cancel(p.key()) }))))))
	}

	cover := coverView(coverSource{path: g.CoverPath, version: g.CoverVersion, name: g.Name}, fyne.NewSize(170, 255))
	hero := container.NewBorder(nil, nil, container.NewVBox(cover), nil, container.NewPadded(info))

	left := stack(p.savesCard(g, busy, hasRemote), p.backupsCard(g, busy, hasRemote))
	right := stack(p.steamCard(g, orig, busy))
	if len(g.OtherDevices) > 0 {
		right.Add(otherDevicesCard(g))
	}
	right.Add(p.deleteCard(g))

	replaceScroll(p.scroll, stack(hero, newCols(420, []float32{3, 2}, left, right)))
}

// profileSelector lets the user pick which emulator's saves the page shows; it
// is nil for a game without emulators.
func (p *gameDetail) profileSelector(g services.GameView) fyne.CanvasObject {
	if len(g.Emulators) == 0 {
		return nil
	}
	var names, ids []string
	if len(g.Slots) > 0 || g.Device.Exe != "" {
		names, ids = append(names, lang.L("Without an emulator")), append(ids, "")
	}
	for _, pv := range g.Emulators {
		names, ids = append(names, pv.Name), append(ids, pv.EmulatorID)
	}
	sel := widget.NewSelect(names, func(n string) {
		i := slices.Index(names, n)
		if i < 0 || ids[i] == p.profile {
			return
		}
		p.profile, p.plan, p.backupsLoaded, p.backups = ids[i], nil, false, nil
		p.render()
	})
	if i := slices.Index(ids, p.profile); i >= 0 {
		sel.Selected = names[i]
	}
	sel.Refresh()
	return field(lang.L("Play and sync with"), fixedWidth(300, sel))
}

// cancelButton stops a running sync; it shows "Cancelling…" until it stops.
func (p *gameDetail) cancelButton(label string, cancel func() bool) *widget.Button {
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

func labelOf(label, key string) string {
	if label != "" {
		return label
	}
	return key
}

func (p *gameDetail) savesCard(g services.GameView, busy, hasRemote bool) fyne.CanvasObject {
	u := p.u
	cmpLabel := lang.L("Compare with cloud")
	if p.busy == "plan" {
		cmpLabel = lang.L("Checking…")
	}
	cmp := widget.NewButton(cmpLabel, p.check)
	setEnabled(cmp, !busy && g.Ready && hasRemote)
	rows := container.NewVBox()
	for _, s := range g.Slots {
		rows.Add(saveLocationRow(u, labelOf(s.Label, s.Key), g.ResolvedPaths[s.Key], g.PathErrors[s.Key]))
	}
	c := vbox(cardHead(lang.L("Saves"), cmp), rows)
	if p.plan != nil {
		pl := p.plan
		c.Add(widget.NewSeparator())
		c.Add(newPlanView(pl, pl.Action == "conflict" || g.Status.Conflict, p.doSync))
	} else if g.Status.Conflict {
		c.Add(widget.NewSeparator())
		c.Add(colored(lang.L("Unresolved save conflict. Compare with cloud to choose a version."), widget.WarningImportance))
	}
	return card("", c)
}

func (p *gameDetail) check() {
	var plan services.PlanView
	p.run("plan", func() error {
		var err error
		plan, err = p.u.sync.Plan(p.key())
		return err
	}, func(err error) {
		if err == nil {
			p.plan = &plan
		}
	})
}

func (p *gameDetail) doSync(resolution string) {
	u := p.u
	var action string
	p.run("sync", func() error {
		var err error
		action, err = u.sync.Sync(p.key(), resolution)
		return err
	}, func(err error) {
		if err != nil {
			return
		}
		switch action {
		case "conflict":
			u.toast(lang.L("Save conflict: choose which version to keep below"), toastErr)
		case "none":
			u.toast(lang.L("Already up to date"), toastOK)
		case "push":
			u.toast(lang.L("Uploaded saves"), toastOK)
		default:
			u.toast(lang.L("Downloaded saves"), toastOK)
		}
		p.plan = nil
		if p.backupsLoaded {
			p.loadBackups()
		}
	})
}

func (p *gameDetail) backupsCard(g services.GameView, busy, hasRemote bool) fyne.CanvasObject {
	label := lang.L("Show backups")
	if p.backupsLoaded {
		label = lang.L("Refresh")
	}
	btn := widget.NewButton(label, p.loadBackups)
	setEnabled(btn, !busy && hasRemote)
	nowLabel := lang.L("Back up now")
	if p.busy == "backup" {
		nowLabel = lang.L("Backing up…")
	}
	nowBtn := widget.NewButton(nowLabel, p.backupNow)
	setEnabled(nowBtn, !busy && hasRemote && g.Ready && !g.Status.Running)
	info := lang.L("Cloud backups are off. Before a download replaces this device's saves, a copy is kept on this device (the last {{.Count}}). Turn on cloud backups in Settings to keep backups in the cloud.",
		map[string]any{"Count": syncer.SafetyKeep})
	if n := p.u.settings.BackupRetention; n > 0 {
		info = lang.N("A backup is taken in the cloud whenever saves are about to be overwritten (keeping the last {{.Count}} backup).", n,
			map[string]any{"Count": n})
	}
	c := vbox(cardHead(lang.L("Backups"), nowBtn, btn), muted(info), muted(lang.L("Backups you make with “Back up now” are pinned. Pinned backups are never deleted automatically.")))
	if g.SafetyDir != "" {
		dir := g.SafetyDir
		c.Add(hbox(widget.NewButtonWithIcon(lang.L("Open copies on this device"), theme.FolderOpenIcon(), func() { p.u.openFolder(dir) })))
	}
	if p.backupsLoaded {
		if len(p.backups) == 0 {
			c.Add(muted(lang.L("No backups yet.")))
		}
		for _, b := range p.backups {
			c.Add(backupRow(b, p.busy == "restore", !busy && g.Ready, !busy, func() { p.restore(b) }, func() { p.setPinned(b) }, func() { p.deleteBackup(b) }))
		}
	}
	return card("", c)
}

func (p *gameDetail) loadBackups() {
	var list []syncer.Backup
	p.run("backups", func() error {
		var err error
		list, err = p.u.sync.Backups(p.key())
		return err
	}, func(error) {
		p.backups, p.backupsLoaded = list, true
	})
}

// backupNow stores a pinned backup of this device's saves.
func (p *gameDetail) backupNow() {
	p.run("backup", func() error { return p.u.sync.BackupNow(p.key()) }, func(err error) {
		if err == nil {
			p.u.toast(lang.L("Backup saved and pinned"), toastOK)
			p.loadBackups()
		}
	})
}

// deleteBackup asks, then permanently deletes a backup for every device.
func (p *gameDetail) deleteBackup(b syncer.Backup) {
	msg := lang.L("Delete the backup from {{.When}}? It is shared with all your devices and can't be recovered.", map[string]any{"When": fmtDate(b.Time)})
	if b.Pinned {
		msg = lang.L("This backup is pinned. Delete the backup from {{.When}} anyway? It is shared with all your devices and can't be recovered.", map[string]any{"When": fmtDate(b.Time)})
	}
	dialog.ShowConfirm(lang.L("Delete backup"), msg, func(ok bool) {
		if !ok {
			return
		}
		p.run("pin", func() error { return p.u.sync.DeleteBackup(p.key(), b.Name) }, func(err error) {
			if err == nil {
				p.u.toast(lang.L("Backup deleted"), toastOK)
				p.loadBackups()
				p.u.refresh() // the last copy on this device may be gone
			}
		})
	}, p.u.win)
}

func (p *gameDetail) setPinned(b syncer.Backup) {
	p.run("pin", func() error { return p.u.sync.SetBackupPinned(p.key(), b.Name, !b.Pinned) }, func(err error) {
		if err == nil {
			p.loadBackups()
		}
	})
}

func (p *gameDetail) restore(b syncer.Backup) {
	p.run("restore", func() error { return p.u.sync.RestoreBackup(p.key(), b.Name) }, func(err error) {
		if err == nil {
			p.u.toast(lang.L("Backup restored and uploaded"), toastOK)
			p.loadBackups()
		}
	})
}

// steamCard is the Steam shortcut form. g is the game as seen through the
// selected profile, orig the game itself. A game with emulators gets one
// shortcut for the whole game by default; it asks which emulator to use when it
// starts. The profile's own shortcut is the other choice.
func (p *gameDetail) steamCard(g, orig services.GameView, busy bool) fyne.CanvasObject {
	u := p.u
	key, form := p.key(), g
	target, asks, hideMode := "game", false, false
	if len(orig.Emulators) > 0 {
		if p.steamTarget == "" {
			p.steamTarget = "game"
		}
		if p.profile == "" || p.steamTarget == "game" {
			key, form, asks = p.id, orig, true // the whole game's shortcut
		} else {
			target = p.profile
		}
		hideMode = len(orig.Slots) == 0 || !asks
	}
	c := vbox(newSteamForm(steamFormOpts{
		Game: form, Info: p.steam, User: p.steamUser, Mode: p.steamMode, Busy: busy,
		Profiles: orig.Emulators, Target: target, AsksAtLaunch: asks, HideMode: hideMode,
		OnTarget: func(v string) {
			// "Only <emulator>" is that emulator's profile: show it, so the shortcut and the page agree.
			if v == "game" {
				p.steamTarget = "game"
			} else if v != p.profile {
				p.steamTarget, p.profile, p.plan, p.backupsLoaded, p.backups = "profile", v, nil, false, nil
			} else {
				p.steamTarget = "profile"
			}
			p.render()
		},
		OnUser: func(v string) { p.steamUser = v },
		OnMode: func(v string) { p.steamMode = v; p.render() },
		OnAdd: func() {
			var res core.AddToSteamResult
			p.run("steam", func() error {
				var err error
				mode := p.steamMode
				if hideMode {
					mode = "direct"
				}
				res, err = u.steam.AddToSteam(key, p.steamUser, mode)
				return err
			}, func(err error) {
				if err != nil {
					return
				}
				if res.SteamRunning {
					u.toast(lang.L("Added to Steam. Restart Steam to see it."), toastOK)
				} else {
					u.toast(lang.L("Added to Steam"), toastOK)
				}
			})
		},
		OnRemove: func() {
			var n int
			p.run("steam", func() error {
				var err error
				n, err = u.steam.RemoveFromSteam(key, p.steamUser)
				return err
			}, func(err error) {
				if err == nil && n > 0 {
					u.toast(lang.L("Removed from Steam"), toastOK)
				} else if err == nil {
					u.toast(lang.L("No shortcut found"), toastOK)
				}
			})
		},
	}))
	if p.wrapOpts != "" {
		p.wrapSection = newAccordionSection(lang.L("Already own it on Steam?"), vbox(
			muted(lang.L("Paste this into the game's Steam launch options to sync around Steam's own launch:")),
			newCopyField(u, p.wrapOpts)), p.wrapOpen)
		c.Add(p.wrapSection)
	}
	return card(lang.L("Steam"), c)
}

func (p *gameDetail) deleteCard(g services.GameView) fyne.CanvasObject {
	if !p.confirmDelete {
		return card("", hbox(dangerButton(lang.L("Delete from library…"), func() { p.confirmDelete, p.deleteSaves = true, false; p.render() })))
	}
	msg := lang.L("Move {{.Game}} to the trash on all devices? You can restore it from the Trash page. Cloud saves and backups are kept.", map[string]any{"Game": g.Name})
	if p.deleteSaves {
		msg = lang.L("Delete {{.Game}} permanently on all devices, with its cloud saves and backups (for every emulator too)? It can't be restored.", map[string]any{"Game": g.Name})
	}
	saves := rebuildingCheck(lang.L("Also delete its cloud saves and backups"), p.deleteSaves, func(on bool) { p.deleteSaves = on; p.render() })
	hint := muted("")
	if !p.deleteSaves {
		hint = muted(lang.L("The cloud saves stay until you delete them. Once it is gone for good, they are listed under Trash → Leftover cloud saves."))
	}
	return card("",
		colored(msg, widget.MediumImportance),
		saves,
		hint,
		hbox(widget.NewButton(lang.L("Cancel"), func() { p.confirmDelete, p.deleteSaves = false, false; p.render() }),
			dangerButton(lang.L("Delete"), func() {
				del := p.u.lib.Delete
				if p.deleteSaves {
					del = p.u.lib.DeleteWithSaves
				}
				p.run("delete", func() error { return del(p.id) }, func(err error) {
					if err == nil {
						p.u.reloadLeftoverCount()
						p.u.navigate("/")
					}
				})
			})))
}
