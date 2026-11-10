package ui

import (
	"errors"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/lang"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/akfaiz/carryover/internal/core"
	"github.com/akfaiz/carryover/internal/syncer"
	"github.com/akfaiz/carryover/services"
)

type gameDetail struct {
	u  *UI
	id string

	busy          string
	plan          *services.PlanView
	backups       []syncer.Backup
	backupsLoaded bool
	steam         *services.SteamInfo
	steamUser     string
	steamMode     string
	wrapOpts      string
	confirmDelete bool
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

func (p *gameDetail) view() fyne.CanvasObject {
	p.scroll = scrollPage(widget.NewLabel(""))
	if g, ok := p.u.game(p.id); ok && g.Device.LaunchMode == "steamwrap" {
		p.steamMode = "steamwrap"
		p.wrapOpen = g.StoreAppID != ""
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

func (p *gameDetail) updateProgress() {
	ev := p.u.progress[p.id]
	switch p.progOf {
	case "session":
		st := p.u.status[p.id]
		if st.Phase == "playing" {
			return
		}
		p.prog.set(st.Message, ev.Bytes, ev.TotalBytes)
	case "busy":
		msg := ev.Message
		if msg == "" {
			msg = lang.L("Syncing…")
			if p.busy == "restore" {
				msg = lang.L("Restoring backup…")
			}
		}
		p.prog.set(msg, ev.Bytes, ev.TotalBytes)
	}
}

// run marks the page busy while work runs.
func (p *gameDetail) run(label string, work func() error, done func(error)) {
	p.busy = label
	if label == "sync" || label == "restore" {
		delete(p.u.progress, p.id)
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
		replaceScroll(p.scroll, vbox(pageHead(u.back, ""), muted(msg)))
		return
	}
	busy := p.busy != ""
	hasRemote := u.settings.Remote != ""

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
	actions := hbox()
	switch {
	case g.Status.Running:
		b := primary(lang.L("● Running"), nil)
		b.Disable()
		actions.Add(b)
	case g.Ready && g.Device.LaunchMode != "steamwrap":
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
	syncBtn := widget.NewButtonWithIcon(syncLabel, theme.ViewRefreshIcon(), func() { p.doSync("") })
	setEnabled(syncBtn, !busy && g.Ready && hasRemote && !g.Status.Running)
	actions.Add(syncBtn)
	actions.Add(widget.NewButtonWithIcon(lang.L("Edit"), theme.DocumentCreateIcon(), func() { u.navigate("/edit/" + p.id) }))
	info.Add(actions)
	if g.Device.LaunchMode == "steamwrap" && g.Ready {
		info.Add(muted(lang.L("This game is launched from Steam on this device (Proton). Use “Add to Steam” below, then start it from Steam.")))
	}

	st, hasSt := u.status[p.id]
	ev := u.progress[p.id]
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
			box.Add(hbox(p.cancelButton(label, func() bool { return u.sess.Cancel(p.id) })))
		}
		info.Add(fixedWidth(480, card("", box)))
	} else if p.busy == "sync" || p.busy == "restore" {
		p.prog, p.progOf = newProgress(), "busy"
		p.updateProgress()
		info.Add(fixedWidth(480, card("", vbox(p.prog, hbox(p.cancelButton(lang.L("Cancel"), func() bool { return u.sync.Cancel(p.id) }))))))
	}

	cover := coverView(coverSource{path: g.CoverPath, version: g.CoverVersion, name: g.Name}, fyne.NewSize(170, 255))
	hero := container.NewBorder(nil, nil, container.NewVBox(cover), nil, container.NewPadded(info))

	left := vbox(p.savesCard(g, busy, hasRemote), p.backupsCard(g, busy, hasRemote))
	right := vbox(p.steamCard(g, busy))
	if len(g.OtherDevices) > 0 {
		right.Add(otherDevicesCard(g))
	}
	right.Add(p.deleteCard(g))

	replaceScroll(p.scroll, vbox(hero, newCols(420, []float32{3, 2}, left, right)))
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
		plan, err = p.u.sync.Plan(p.id)
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
		action, err = u.sync.Sync(p.id, resolution)
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
	c := vbox(cardHead(lang.L("Backups"), btn),
		muted(lang.N("A backup is taken in the cloud whenever saves are about to be overwritten (keeping the last {{.Count}} backup).", p.u.settings.BackupRetention,
			map[string]any{"Count": p.u.settings.BackupRetention})))
	if p.backupsLoaded {
		if len(p.backups) == 0 {
			c.Add(muted(lang.L("No backups yet.")))
		}
		for _, b := range p.backups {
			c.Add(backupRow(b, p.busy == "restore", !busy && g.Ready, func() { p.restore(b) }))
		}
	}
	return card("", c)
}

func (p *gameDetail) loadBackups() {
	var list []syncer.Backup
	p.run("backups", func() error {
		var err error
		list, err = p.u.sync.Backups(p.id)
		return err
	}, func(error) {
		p.backups, p.backupsLoaded = list, true
	})
}

func (p *gameDetail) restore(b syncer.Backup) {
	p.run("restore", func() error { return p.u.sync.RestoreBackup(p.id, b.Name) }, func(err error) {
		if err == nil {
			p.u.toast(lang.L("Backup restored and uploaded"), toastOK)
			p.loadBackups()
		}
	})
}

func (p *gameDetail) steamCard(g services.GameView, busy bool) fyne.CanvasObject {
	u := p.u
	c := vbox(newSteamForm(steamFormOpts{
		Game: g, Info: p.steam, User: p.steamUser, Mode: p.steamMode, Busy: busy,
		OnUser: func(v string) { p.steamUser = v },
		OnMode: func(v string) { p.steamMode = v; p.render() },
		OnAdd: func() {
			var res core.AddToSteamResult
			p.run("steam", func() error {
				var err error
				res, err = u.steam.AddToSteam(p.id, p.steamUser, p.steamMode)
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
				n, err = u.steam.RemoveFromSteam(p.id, p.steamUser)
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
		return card("", hbox(dangerButton(lang.L("Delete from library…"), func() { p.confirmDelete = true; p.render() })))
	}
	return card("",
		colored(lang.L("Remove {{.Game}} from the library on all devices? Cloud saves and backups are kept.", map[string]any{"Game": g.Name}), widget.MediumImportance),
		hbox(widget.NewButton(lang.L("Cancel"), func() { p.confirmDelete = false; p.render() }),
			dangerButton(lang.L("Delete"), func() {
				p.run("delete", func() error { return p.u.lib.Delete(p.id) }, func(err error) {
					if err == nil {
						p.u.navigate("/")
					}
				})
			})))
}
