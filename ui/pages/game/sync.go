package game

import (
	"errors"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/lang"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/akfaiz/carryover/services"
	"github.com/akfaiz/carryover/ui/components"
	"github.com/akfaiz/carryover/ui/kit"
)

func (p *Page) updateProgress() {
	switch p.progOf {
	case "session":
		st := p.u.Store().Status[p.vm.id]
		if st.Phase == "playing" || st.Phase == "choose" {
			return
		}
		ev := p.u.Store().Progress[p.sessionKey()]
		p.prog.Set(st.Message, ev.Bytes, ev.TotalBytes)
	case "busy":
		ev := p.u.Store().Progress[p.vm.key()]
		msg := ev.Message
		if msg == "" {
			msg = lang.L("Syncing…")
			switch p.vm.busy {
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
	p.vm.busy = label
	if label == "sync" || label == "restore" || label == "backup" {
		p.u.Store().ClearProgress(p.vm.key())
	}
	p.render()
	kit.Async0(work, func(err error) {
		p.vm.busy = ""
		p.vm.cancelling = false
		switch {
		case errors.Is(err, services.ErrSyncCancelled):
			p.u.Toast(lang.L("Sync cancelled. Files already transferred are kept; the next sync continues."), kit.ToastInfo)
			err = nil
			for _, st := range p.vm.states {
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
	return kit.Card("", kit.VBox(p.prog, kit.HBox(p.cancelButton(lang.L("Cancel"), func() bool { return p.vm.be.CancelSync(p.vm.key()) }))))
}

// cancelButton stops a running sync; it shows "Cancelling…" until it stops.
func (p *Page) cancelButton(label string, cancel func() bool) *widget.Button {
	if p.vm.cancelling {
		b := widget.NewButton(lang.L("Cancelling…"), nil)
		b.Disable()
		return b
	}
	return widget.NewButtonWithIcon(label, theme.CancelIcon(), func() {
		if cancel() {
			p.vm.cancelling = true
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
	st := p.vm.state(pe.id)
	cmpLabel := lang.L("Compare with cloud")
	if p.vm.busyFor("plan", pe.id) {
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
	if p.vm.busyFor("plan", pe.id) {
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
	var plan services.PlanView
	p.vm.setActive(prof)
	p.run("plan", func() error {
		var err error
		plan, err = p.vm.Plan(prof)
		return err
	}, func(err error) {
		if err == nil {
			p.vm.state(prof).plan = &plan
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
	p.vm.setActive(ids[0])
	var done, actions []string
	p.run("sync", func() error {
		var err error
		done, actions, err = p.vm.SyncProfiles(ids)
		return err
	}, func(err error) {
		if err == nil {
			p.synced(done, actions)
		}
	})
}

// doSync syncs one profile; resolution settles a conflict ("local" or "cloud").
func (p *Page) doSync(prof, resolution string) {
	var action string
	p.vm.setActive(prof)
	p.run("sync", func() error {
		var err error
		action, err = p.vm.SyncOne(prof, resolution)
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
		p.vm.state(prof).plan = nil
	}
	p.loadBackups(profs...)
	if conflict >= 0 {
		prof := profs[conflict]
		p.vm.selected, p.vm.chosen = prof, true
		p.check(prof)
	}
}
