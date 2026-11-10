package ui

import (
	"fmt"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/akfaiz/playrelay/internal/core"
	"github.com/akfaiz/playrelay/internal/syncer"
	"github.com/akfaiz/playrelay/services"
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
	wrapOpen      bool
	wrapItem      *widget.AccordionItem // to keep its open state across renders

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
			msg = "Syncing…"
			if p.busy == "restore" {
				msg = "Restoring backup…"
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
		if err != nil {
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
	if p.wrapItem != nil {
		p.wrapOpen = p.wrapItem.Open
	}
	g, ok := u.game(p.id)
	if !ok {
		msg := "Loading…"
		if u.loaded {
			msg = "Game not found."
		}
		replaceScroll(p.scroll, vbox(pageHead(u.back, ""), muted(msg)))
		return
	}
	busy := p.busy != ""
	hasRemote := u.settings.Remote != ""

	// Hero.
	info := vbox(
		hbox(lowButton("← Library", func() { u.navigate("/") })),
		wrapTitle(g.Name),
		hbox(statusBadge(g), small("Last sync: "+timeAgo(g.Status.LastSyncAt))),
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
		b := primary("● Running", nil)
		b.Disable()
		actions.Add(b)
	case g.Ready && g.Device.LaunchMode != "steamwrap":
		b := primary("▶ Play", func() { p.run("play", func() error { return u.sess.Play(p.id) }, nil) })
		setEnabled(b, !busy)
		actions.Add(b)
	case !g.Ready:
		actions.Add(primary("Set up for this device", func() { u.navigate("/edit/" + p.id) }))
	}
	syncLabel := "Sync now"
	if p.busy == "sync" {
		syncLabel = "Syncing…"
	}
	syncBtn := widget.NewButtonWithIcon(syncLabel, theme.ViewRefreshIcon(), func() { p.doSync("") })
	setEnabled(syncBtn, !busy && g.Ready && hasRemote && !g.Status.Running)
	actions.Add(syncBtn)
	actions.Add(widget.NewButtonWithIcon("Edit", theme.DocumentCreateIcon(), func() { u.navigate("/edit/" + p.id) }))
	info.Add(actions)
	if g.Device.LaunchMode == "steamwrap" && g.Ready {
		info.Add(muted("This game is launched from Steam on this device (Proton). Use “Add to Steam” below, then start it from Steam."))
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
		info.Add(fixedWidth(480, card("", box)))
	} else if p.busy == "sync" || p.busy == "restore" {
		p.prog, p.progOf = newProgress(), "busy"
		p.updateProgress()
		info.Add(fixedWidth(480, card("", p.prog)))
	}

	cover := coverView(coverSource{path: g.CoverPath, version: g.CoverVersion, name: g.Name}, fyne.NewSize(170, 255))
	hero := container.NewBorder(nil, nil, container.NewVBox(cover), nil, container.NewPadded(info))

	left := vbox(p.savesCard(g, busy, hasRemote), p.backupsCard(g, busy, hasRemote))
	right := vbox(p.steamCard(g, busy))
	if len(g.OtherDevices) > 0 {
		devs := vbox()
		for _, d := range g.OtherDevices {
			devs.Add(widget.NewLabelWithStyle(d.DeviceName, fyne.TextAlignLeading, fyne.TextStyle{Bold: true}))
			for _, s := range g.Slots {
				if v := d.Paths[s.Key]; v != "" {
					l := mono(labelOf(s.Label, s.Key) + ": " + v)
					l.Importance = widget.LowImportance
					l.SizeName = theme.SizeNameCaptionText
					devs.Add(l)
				}
			}
		}
		right.Add(card("Other devices", devs))
	}
	right.Add(p.deleteCard(g))

	replaceScroll(p.scroll, vbox(hero, newCols(420, []float32{3, 2}, left, right)))
}

func labelOf(label, key string) string {
	if label != "" {
		return label
	}
	return key
}

var planText = map[string]string{
	"none":     "Local and cloud saves are identical.",
	"push":     "Local saves are newer — they will be uploaded.",
	"pull":     "Cloud saves are newer — they will be downloaded.",
	"conflict": "Both changed since the last sync. Choose which to keep.",
}

func (p *gameDetail) savesCard(g services.GameView, busy, hasRemote bool) fyne.CanvasObject {
	u := p.u
	cmpLabel := "Compare with cloud"
	if p.busy == "plan" {
		cmpLabel = "Checking…"
	}
	cmp := widget.NewButton(cmpLabel, p.check)
	setEnabled(cmp, !busy && g.Ready && hasRemote)
	rows := container.NewVBox()
	for _, s := range g.Slots {
		name := widget.NewLabelWithStyle(labelOf(s.Label, s.Key), fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
		var val fyne.CanvasObject
		switch {
		case g.PathErrors[s.Key] != "":
			val = colored(g.PathErrors[s.Key], widget.DangerImportance)
		case g.ResolvedPaths[s.Key] != "":
			path := g.ResolvedPaths[s.Key]
			l := mono(path)
			l.Importance = widget.HighImportance
			val = newTappable(container.NewBorder(nil, nil, widget.NewIcon(theme.FolderOpenIcon()), nil, l), func() { u.openFolder(path) })
		default:
			val = colored("not set on this device", widget.WarningImportance)
		}
		rows.Add(container.NewBorder(nil, nil, fixedWidth(110, name), nil, val))
	}
	c := vbox(cardHead("Saves", cmp), rows)
	if p.plan != nil {
		pl := p.plan
		c.Add(widget.NewSeparator())
		c.Add(widget.NewLabelWithStyle(planText[pl.Action], fyne.TextAlignLeading, fyne.TextStyle{Bold: true}))
		local := vbox(muted("This device"),
			widget.NewLabel(fmt.Sprintf("%d files · %s\nmodified %s", pl.Local.Files, formatBytes(pl.Local.Bytes), fmtDate(pl.Local.Newest))))
		cloudTitle := "Cloud"
		if pl.Remote.DeviceName != "" {
			cloudTitle += " (from " + pl.Remote.DeviceName + ")"
		}
		cloudText := "no PlayRelay saves for this game yet"
		if pl.HasRemote {
			cloudText = fmt.Sprintf("%d files · %s\nuploaded %s", pl.Remote.Files, formatBytes(pl.Remote.Bytes), fmtDate(pl.Remote.Time))
		}
		c.Add(container.NewGridWithColumns(2, local, vbox(muted(cloudTitle), widget.NewLabel(cloudText))))
		if pl.Action == "conflict" || g.Status.Conflict {
			keepCloud := widget.NewButton("Keep cloud saves", func() { p.doSync("cloud") })
			setEnabled(keepCloud, pl.HasRemote)
			c.Add(hbox(widget.NewButton("Keep this device's saves", func() { p.doSync("local") }), keepCloud))
		}
	} else if g.Status.Conflict {
		c.Add(widget.NewSeparator())
		c.Add(colored("Unresolved save conflict. Compare with cloud to choose a version.", widget.WarningImportance))
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
			u.toast("Save conflict: choose which version to keep below", toastErr)
		case "none":
			u.toast("Already up to date", toastOK)
		case "push":
			u.toast("Uploaded saves", toastOK)
		default:
			u.toast("Downloaded saves", toastOK)
		}
		p.plan = nil
		if p.backupsLoaded {
			p.loadBackups()
		}
	})
}

func (p *gameDetail) backupsCard(g services.GameView, busy, hasRemote bool) fyne.CanvasObject {
	label := "Show backups"
	if p.backupsLoaded {
		label = "Refresh"
	}
	btn := widget.NewButton(label, p.loadBackups)
	setEnabled(btn, !busy && hasRemote)
	c := vbox(cardHead("Backups", btn),
		muted(fmt.Sprintf("A backup is taken in the cloud whenever saves are about to be overwritten (keeping the last %d).", p.u.settings.BackupRetention)))
	if p.backupsLoaded {
		if len(p.backups) == 0 {
			c.Add(muted("No backups yet."))
		}
		for _, b := range p.backups {
			desc := b.DeviceName + "'s local saves"
			if b.Kind != "local" {
				desc = "cloud saves (replaced by " + b.DeviceName + ")"
			}
			rl := "Restore"
			if p.busy == "restore" {
				rl = "…"
			}
			restore := widget.NewButton(rl, func() { p.restore(b) })
			setEnabled(restore, !busy && g.Ready)
			c.Add(container.NewBorder(nil, nil, fixedWidth(140, widget.NewLabel(fmtDate(b.Time))), restore, muted(desc)))
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
			p.u.toast("Backup restored and uploaded", toastOK)
			p.loadBackups()
		}
	})
}

func (p *gameDetail) steamCard(g services.GameView, busy bool) fyne.CanvasObject {
	u := p.u
	c := vbox()
	switch {
	case p.steam == nil:
		c.Add(muted("Detecting Steam…"))
	case !p.steam.Found:
		c.Add(muted("Steam was not found on this device."))
	default:
		c.Add(muted("Add " + g.Name + " to Steam as a non-Steam game. Steam launches it through PlayRelay, so saves sync " +
			"automatically before and after every session — also in Steam Deck Game Mode."))
		if len(p.steam.Users) > 1 {
			users := &choice{}
			for _, us := range p.steam.Users {
				users.add(us.ID, us.Name)
			}
			c.Add(field("Steam account", selectOf(users, p.steamUser, func(v string) { p.steamUser = v })))
		}
		modes := &choice{}
		modes.add("direct", "Run PlayRelay (it starts the game)")
		modes.add("steamwrap", "Run the game exe, wrapped by PlayRelay (pick Proton in Steam)")
		c.Add(field("Shortcut type", selectOf(modes, p.steamMode, func(v string) { p.steamMode = v; p.render() })))
		if p.steamMode == "steamwrap" {
			t := "After adding, open the shortcut's Properties → Compatibility in Steam and force a Proton version. " +
				"Its save prefix is available as {proton:shortcut}"
			if g.Device.SteamAppID != "" {
				t += " (app id " + g.Device.SteamAppID + ")"
			}
			c.Add(muted(t + "."))
		}
		addLabel := "Add to Steam"
		if g.Device.SteamAppID != "" {
			addLabel = "Update Steam shortcut"
		}
		add := primary(addLabel, func() {
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
					u.toast("Added to Steam. Restart Steam to see it.", toastOK)
				} else {
					u.toast("Added to Steam", toastOK)
				}
			})
		})
		setEnabled(add, !busy && p.steamUser != "")
		row := hbox(add)
		if g.Device.SteamAppID != "" {
			rm := widget.NewButton("Remove", func() {
				var n int
				p.run("steam", func() error {
					var err error
					n, err = u.steam.RemoveFromSteam(p.id, p.steamUser)
					return err
				}, func(err error) {
					if err == nil && n > 0 {
						u.toast("Removed from Steam", toastOK)
					} else if err == nil {
						u.toast("No shortcut found", toastOK)
					}
				})
			})
			setEnabled(rm, !busy)
			row.Add(rm)
		}
		c.Add(row)
		if p.steam.Running {
			c.Add(colored("Steam is running — restart it after adding so it picks up the change.", widget.WarningImportance))
		}
	}
	if p.wrapOpts != "" {
		e := widget.NewEntry()
		e.SetText(p.wrapOpts)
		e.TextStyle.Monospace = true
		e.OnChanged = func(string) { e.SetText(p.wrapOpts) } // read-only but selectable
		copyBtn := widget.NewButtonWithIcon("Copy", theme.ContentCopyIcon(), func() {
			u.app.Clipboard().SetContent(p.wrapOpts)
			u.toast("Copied", toastOK)
		})
		item := widget.NewAccordionItem("Already own it on Steam?", vbox(
			muted("Paste this into the game's Steam launch options to sync around Steam's own launch:"),
			rowFill(e, copyBtn)))
		item.Open = p.wrapOpen
		p.wrapItem = item
		c.Add(widget.NewAccordion(item))
	}
	return card("Steam", c)
}

func (p *gameDetail) deleteCard(g services.GameView) fyne.CanvasObject {
	if !p.confirmDelete {
		return card("", hbox(dangerButton("Delete from library…", func() { p.confirmDelete = true; p.render() })))
	}
	return card("",
		colored("Remove "+g.Name+" from the library on all devices? Cloud saves and backups are kept.", widget.MediumImportance),
		hbox(widget.NewButton("Cancel", func() { p.confirmDelete = false; p.render() }),
			dangerButton("Delete", func() {
				p.run("delete", func() error { return p.u.lib.Delete(p.id) }, func(err error) {
					if err == nil {
						p.u.navigate("/")
					}
				})
			})))
}
