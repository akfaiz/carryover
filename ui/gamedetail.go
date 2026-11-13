package ui

import (
	"errors"
	"slices"
	"sync"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/lang"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/akfaiz/carryover/internal/cloudsync/syncer"
	"github.com/akfaiz/carryover/internal/core"
	"github.com/akfaiz/carryover/services"
)

type gameDetail struct {
	u  *UI
	id string
	// The page shows the saves of every profile of the game (its own and one
	// per emulator) one after the other. active is the profile the running or
	// last action works on ("" = the game's own saves); it is read from other
	// goroutines (a sync of several profiles moves it along), hence the mutex.
	mu     sync.Mutex
	active string
	states map[string]*profState // by emulator id, "" = the game's own saves

	busy          string
	steam         *services.SteamInfo
	steamUser     string
	steamMode     string
	steamTarget   string // "game" (default for a game with emulators) or the emulator the shortcut is for
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

// profState is what the page knows about one profile: a comparison with the
// cloud and its backups, once asked for.
type profState struct {
	plan          *services.PlanView
	backups       []syncer.Backup
	backupsLoaded bool
}

// state returns the state of a profile ("" = the game's own saves).
func (p *gameDetail) state(prof string) *profState {
	if p.states == nil {
		p.states = map[string]*profState{}
	}
	if p.states[prof] == nil {
		p.states[prof] = &profState{}
	}
	return p.states[prof]
}

func (p *gameDetail) setActive(prof string) {
	p.mu.Lock()
	p.active = prof
	p.mu.Unlock()
}

func (p *gameDetail) activeProfile() string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.active
}

// key is what the services address the profile an action works on by
// (core.SyncKey).
func (p *gameDetail) key() string { return core.SyncKey(p.id, p.activeProfile()) }

// profileEntry is one profile of the game as the page lists it.
type profileEntry struct {
	id   string // emulator id, "" = the game's own saves
	tag  string // what tells it apart from the others; "" for a game without emulators
	view services.GameView
	// ready: its saves can be synced on this device.
	ready bool
}

// profiles lists the profiles the page shows: the game's own saves (if it has
// any) and each emulator, every one with a tag naming it when the game has
// emulators.
func (p *gameDetail) profiles(g services.GameView) []profileEntry {
	var out []profileEntry
	if len(g.Slots) > 0 || len(g.Emulators) == 0 {
		tag := ""
		if len(g.Emulators) > 0 {
			tag = lang.L("Without an emulator")
		}
		out = append(out, profileEntry{id: "", tag: tag, view: g, ready: g.Ready})
	}
	for _, pv := range g.Emulators {
		v := profileView(g, pv.EmulatorID)
		out = append(out, profileEntry{id: pv.EmulatorID, tag: pv.Name, view: v, ready: pv.Ready})
	}
	return out
}

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
	if what == changeStatus && p.u.status[p.id].Phase != "sync" && p.busy == "" {
		p.cancelling = false // the session's sync phase ended
	}
	if what == changeProgress && p.prog != nil {
		p.updateProgress()
		return
	}
	p.render()
}

// sessionKey is the sync key of the play session the page shows: the profile
// that was started last.
func (p *gameDetail) sessionKey() string {
	if st, ok := p.u.status[p.id]; ok && st.Key != "" {
		return st.Key
	}
	return p.id
}

func (p *gameDetail) updateProgress() {
	switch p.progOf {
	case "session":
		st := p.u.status[p.id]
		if st.Phase == "playing" || st.Phase == "choose" {
			return
		}
		ev := p.u.progress[p.sessionKey()]
		p.prog.set(st.Message, ev.Bytes, ev.TotalBytes)
	case "busy":
		ev := p.u.progress[p.key()]
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
			for _, st := range p.states {
				st.plan = nil
			}
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
	orig, ok := u.game(p.id)
	if !ok {
		msg := lang.L("Loading…")
		if u.loaded {
			msg = lang.L("Game not found.")
		}
		replaceScroll(p.scroll, stack(pageHead(u.back, ""), muted(msg)))
		return
	}
	g := orig
	g.Status = g.Overall // the game as a whole: its own saves and every emulator
	profs := p.profiles(orig)
	busy := p.busy != ""
	hasRemote := u.settings.Remote != ""
	canSync := slices.ContainsFunc(profs, func(pe profileEntry) bool { return pe.ready })

	// Hero.
	info := vbox(
		hbox(lowButton(lang.L("← Library"), func() { u.navigate("/") })),
		wrapTitle(g.Name),
		hbox(statusBadge(g), small(lang.L("Last sync: {{.When}}", map[string]any{"When": timeAgo(g.Status.LastSyncAt)}))),
	)
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
		// A game that can be played in several ways asks which one first.
		b := primary(lang.L("▶ Play"), func() { p.run("play", func() error { return u.sess.Play(p.id) }, nil) })
		setEnabled(b, !busy)
		actions.Add(b)
	case !g.Ready:
		actions.Add(primary(lang.L("Set up for this device"), func() { u.navigate("/edit/" + p.id) }))
	}
	syncLabel := lang.L("Sync now")
	if p.busy == "sync" {
		syncLabel = lang.L("Syncing…")
	}
	syncBtn := widget.NewButtonWithIcon(syncLabel, theme.ViewRefreshIcon(), func() { p.syncAll(profs) })
	setEnabled(syncBtn, !busy && canSync && hasRemote && !g.Status.Running)
	actions.Add(syncBtn)
	actions.Add(widget.NewButtonWithIcon(lang.L("Edit"), theme.DocumentCreateIcon(), func() { u.navigate("/edit/" + p.id) }))
	info.Add(actions)
	if g.Device.LaunchMode == "steamwrap" && g.Ready {
		info.Add(muted(lang.L("This game is launched from Steam on this device (Proton). Use “Add to Steam” below, then start it from Steam.")))
	}

	sessKey := p.sessionKey()
	st, hasSt := u.status[p.id]
	ev := u.progress[sessKey]
	if hasSt && (st.Phase != "done" || g.Status.Running) && st.Phase != "error" {
		box := vbox(widget.NewLabelWithStyle(st.Message, fyne.TextAlignLeading, fyne.TextStyle{Bold: true}))
		if st.Phase != "playing" && st.Phase != "choose" {
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
			box.Add(hbox(p.cancelButton(label, func() bool { return u.sess.Cancel(sessKey) })))
		}
		info.Add(fixedWidth(480, card("", box)))
	} else if p.busy == "sync" || p.busy == "restore" || p.busy == "backup" {
		p.prog, p.progOf = newProgress(), "busy"
		p.updateProgress()
		info.Add(fixedWidth(480, card("", vbox(p.prog, hbox(p.cancelButton(lang.L("Cancel"), func() bool { return u.sync.Cancel(p.key()) }))))))
	}

	cover := coverView(coverSource{path: g.CoverPath, version: g.CoverVersion, name: g.Name}, fyne.NewSize(170, 255))
	hero := container.NewBorder(nil, nil, container.NewVBox(cover), nil, container.NewPadded(info))

	// The saves and backups of every profile, each tagged with its emulator.
	left := stack()
	for _, pe := range profs {
		left.Add(p.savesCard(pe, busy, hasRemote))
		left.Add(p.backupsCard(pe, busy, hasRemote))
	}
	right := stack(p.steamCard(orig, busy))
	if len(g.OtherDevices) > 0 {
		right.Add(otherDevicesCard(g))
	}
	right.Add(p.deleteCard(g))

	replaceScroll(p.scroll, stack(hero, newCols(420, []float32{3, 2}, left, right)))
}

// taggedHead is a card heading with a badge naming the profile the card is
// about (when the game has several) and right-aligned actions.
func taggedHead(title, tag string, actions ...fyne.CanvasObject) fyne.CanvasObject {
	objs := []fyne.CanvasObject{heading(title)}
	if tag != "" {
		objs = append(objs, badge(tag, badgeInfo))
	}
	objs = append(objs, layout.NewSpacer())
	return container.NewHBox(append(objs, actions...)...)
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

// savesCard shows the save locations of one profile and compares them with the
// cloud. A game with emulators gets one per profile, tagged with its name and
// showing that profile's own status.
func (p *gameDetail) savesCard(pe profileEntry, busy, hasRemote bool) fyne.CanvasObject {
	u := p.u
	g := pe.view
	st := p.state(pe.id)
	cmpLabel := lang.L("Compare with cloud")
	if p.busy == "plan" && p.activeProfile() == pe.id {
		cmpLabel = lang.L("Checking…")
	}
	cmp := widget.NewButton(cmpLabel, func() { p.check(pe.id) })
	setEnabled(cmp, !busy && g.Ready && hasRemote)
	rows := container.NewVBox()
	for _, sl := range g.Slots {
		rows.Add(saveLocationRow(u, labelOf(sl.Label, sl.Key), g.ResolvedPaths[sl.Key], g.PathErrors[sl.Key]))
	}
	c := vbox(taggedHead(lang.L("Saves"), pe.tag, cmp))
	if pe.tag != "" {
		// The page header shows the game as a whole: say how this profile is doing.
		c.Add(hbox(statusBadge(g), small(lang.L("Last sync: {{.When}}", map[string]any{"When": timeAgo(g.Status.LastSyncAt)}))))
		if !g.Ready {
			c.Add(colored(g.SetupHint, widget.WarningImportance))
		}
		if g.Status.LastError != "" && !g.Status.Running {
			c.Add(colored(g.Status.LastError, widget.DangerImportance))
		}
		if g.Status.LocalChanges && !g.Status.Running {
			c.Add(colored(lang.L("These saves changed on this device after the last sync. Use “Sync now” to upload them."), widget.WarningImportance))
		}
	}
	c.Add(rows)
	if pl := st.plan; pl != nil {
		c.Add(widget.NewSeparator())
		c.Add(newPlanView(pl, pl.Action == "conflict" || g.Status.Conflict, func(resolution string) { p.doSync(pe.id, resolution) }))
	} else if g.Status.Conflict {
		c.Add(widget.NewSeparator())
		c.Add(colored(lang.L("Unresolved save conflict. Compare with cloud to choose a version."), widget.WarningImportance))
	}
	return card("", c)
}

func (p *gameDetail) check(prof string) {
	key := core.SyncKey(p.id, prof)
	p.setActive(prof)
	var plan services.PlanView
	p.run("plan", func() error {
		var err error
		plan, err = p.u.sync.Plan(key)
		return err
	}, func(err error) {
		if err == nil {
			p.state(prof).plan = &plan
		}
	})
}

// syncAll syncs every profile that can be synced on this device, one after the
// other: the game's own saves and each emulator's.
func (p *gameDetail) syncAll(profs []profileEntry) {
	var ids []string
	for _, pe := range profs {
		if pe.ready {
			ids = append(ids, pe.id)
		}
	}
	if len(ids) == 0 {
		return
	}
	p.setActive(ids[0])
	var actions []string
	p.run("sync", func() error {
		for _, id := range ids {
			p.setActive(id) // the progress shown follows the profile being synced
			a, err := p.u.sync.Sync(core.SyncKey(p.id, id), "")
			if err != nil {
				return err
			}
			actions = append(actions, a)
		}
		return nil
	}, func(err error) {
		if err == nil {
			p.synced(actions, ids...)
		}
	})
}

// doSync syncs one profile; resolution settles a conflict ("local" or "cloud").
func (p *gameDetail) doSync(prof, resolution string) {
	key := core.SyncKey(p.id, prof)
	p.setActive(prof)
	var action string
	p.run("sync", func() error {
		var err error
		action, err = p.u.sync.Sync(key, resolution)
		return err
	}, func(err error) {
		if err == nil {
			p.synced([]string{action}, prof)
		}
	})
}

// synced tells how the syncs of the profiles went and drops what they made out
// of date: the comparison, and the backup lists that were open.
func (p *gameDetail) synced(actions []string, profs ...string) {
	var push, pull, conflict bool
	for _, a := range actions {
		switch a {
		case "conflict":
			conflict = true
		case "none":
		case "push":
			push = true
		default:
			pull = true
		}
	}
	switch {
	case conflict:
		p.u.toast(lang.L("Save conflict: choose which version to keep below"), toastErr)
	case push && pull:
		p.u.toast(lang.L("Synced saves"), toastOK)
	case push:
		p.u.toast(lang.L("Uploaded saves"), toastOK)
	case pull:
		p.u.toast(lang.L("Downloaded saves"), toastOK)
	default:
		p.u.toast(lang.L("Already up to date"), toastOK)
	}
	var reload []string
	for _, prof := range profs {
		st := p.state(prof)
		st.plan = nil
		if st.backupsLoaded {
			reload = append(reload, prof)
		}
	}
	if len(reload) > 0 {
		p.loadBackups(reload...)
	}
}

// backupsCard lists and takes the backups of one profile's cloud save.
func (p *gameDetail) backupsCard(pe profileEntry, busy, hasRemote bool) fyne.CanvasObject {
	g := pe.view
	st := p.state(pe.id)
	mine := p.activeProfile() == pe.id
	label := lang.L("Show backups")
	if st.backupsLoaded {
		label = lang.L("Refresh")
	}
	btn := widget.NewButton(label, func() { p.loadBackups(pe.id) })
	setEnabled(btn, !busy && hasRemote)
	nowLabel := lang.L("Back up now")
	if p.busy == "backup" && mine {
		nowLabel = lang.L("Backing up…")
	}
	nowBtn := widget.NewButton(nowLabel, func() { p.backupNow(pe.id) })
	setEnabled(nowBtn, !busy && hasRemote && g.Ready && !g.Status.Running)
	info := lang.L("Cloud backups are off. Before a download replaces this device's saves, a copy is kept on this device (the last {{.Count}}). Turn on cloud backups in Settings to keep backups in the cloud.",
		map[string]any{"Count": syncer.SafetyKeep})
	if n := p.u.settings.BackupRetention; n > 0 {
		info = lang.N("A backup is taken in the cloud whenever saves are about to be overwritten (keeping the last {{.Count}} backup).", n,
			map[string]any{"Count": n})
	}
	c := vbox(taggedHead(lang.L("Backups"), pe.tag, nowBtn, btn), muted(info), muted(lang.L("Backups you make with “Back up now” are pinned. Pinned backups are never deleted automatically.")))
	if g.SafetyDir != "" {
		dir := g.SafetyDir
		c.Add(hbox(widget.NewButtonWithIcon(lang.L("Open copies on this device"), theme.FolderOpenIcon(), func() { p.u.openFolder(dir) })))
	}
	if st.backupsLoaded {
		if len(st.backups) == 0 {
			c.Add(muted(lang.L("No backups yet.")))
		}
		for _, b := range st.backups {
			c.Add(backupRow(b, p.busy == "restore" && mine, !busy && g.Ready, !busy,
				func() { p.restore(pe.id, b) }, func() { p.setPinned(pe.id, b) }, func() { p.deleteBackup(pe.id, b) }))
		}
	}
	return card("", c)
}

// loadBackups reads the backup lists of the profiles.
func (p *gameDetail) loadBackups(profs ...string) {
	if len(profs) == 0 {
		return
	}
	p.setActive(profs[0])
	lists := map[string][]syncer.Backup{}
	p.run("backups", func() error {
		for _, prof := range profs {
			list, err := p.u.sync.Backups(core.SyncKey(p.id, prof))
			if err != nil {
				return err
			}
			lists[prof] = list
		}
		return nil
	}, func(err error) {
		if err != nil {
			return
		}
		for prof, list := range lists {
			st := p.state(prof)
			st.backups, st.backupsLoaded = list, true
		}
	})
}

// backupNow stores a pinned backup of this device's saves.
func (p *gameDetail) backupNow(prof string) {
	key := core.SyncKey(p.id, prof)
	p.setActive(prof)
	p.run("backup", func() error { return p.u.sync.BackupNow(key) }, func(err error) {
		if err == nil {
			p.u.toast(lang.L("Backup saved and pinned"), toastOK)
			p.loadBackups(prof)
		}
	})
}

// deleteBackup asks, then permanently deletes a backup for every device.
func (p *gameDetail) deleteBackup(prof string, b syncer.Backup) {
	msg := lang.L("Delete the backup from {{.When}}? It is shared with all your devices and can't be recovered.", map[string]any{"When": fmtDate(b.Time)})
	if b.Pinned {
		msg = lang.L("This backup is pinned. Delete the backup from {{.When}} anyway? It is shared with all your devices and can't be recovered.", map[string]any{"When": fmtDate(b.Time)})
	}
	dialog.ShowConfirm(lang.L("Delete backup"), msg, func(ok bool) {
		if !ok {
			return
		}
		key := core.SyncKey(p.id, prof)
		p.setActive(prof)
		p.run("pin", func() error { return p.u.sync.DeleteBackup(key, b.Name) }, func(err error) {
			if err == nil {
				p.u.toast(lang.L("Backup deleted"), toastOK)
				p.loadBackups(prof)
				p.u.refresh() // the last copy on this device may be gone
			}
		})
	}, p.u.win)
}

func (p *gameDetail) setPinned(prof string, b syncer.Backup) {
	key := core.SyncKey(p.id, prof)
	p.setActive(prof)
	p.run("pin", func() error { return p.u.sync.SetBackupPinned(key, b.Name, !b.Pinned) }, func(err error) {
		if err == nil {
			p.loadBackups(prof)
		}
	})
}

func (p *gameDetail) restore(prof string, b syncer.Backup) {
	key := core.SyncKey(p.id, prof)
	p.setActive(prof)
	p.run("restore", func() error { return p.u.sync.RestoreBackup(key, b.Name) }, func(err error) {
		if err == nil {
			p.u.toast(lang.L("Backup restored and uploaded"), toastOK)
			p.loadBackups(prof)
		}
	})
}

// steamCard is the Steam shortcut form. A game with emulators gets one
// shortcut for the whole game by default; it asks which emulator to use when it
// starts. The shortcut of one emulator is the other choice.
func (p *gameDetail) steamCard(orig services.GameView, busy bool) fyne.CanvasObject {
	u := p.u
	key, form := p.id, orig
	target, asks, hideMode := "game", false, false
	if len(orig.Emulators) > 0 {
		if !slices.ContainsFunc(orig.Emulators, func(pv services.ProfileView) bool { return pv.EmulatorID == p.steamTarget }) {
			p.steamTarget = "game"
		}
		if p.steamTarget == "game" {
			asks = true // the whole game's shortcut
		} else {
			target = p.steamTarget
			key, form = core.SyncKey(p.id, target), profileView(orig, target)
		}
		hideMode = len(orig.Slots) == 0 || !asks
	}
	c := vbox(newSteamForm(steamFormOpts{
		Game: form, Info: p.steam, User: p.steamUser, Mode: p.steamMode, Busy: busy,
		Profiles: orig.Emulators, Target: target, AsksAtLaunch: asks, HideMode: hideMode,
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
	saves := newCheck(lang.L("Also delete its cloud saves and backups"), p.deleteSaves, func(on bool) { p.deleteSaves = on; p.render() })
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
