package game

import (
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/akfaiz/carryover/services"
	"github.com/akfaiz/carryover/ui/components"
	"github.com/akfaiz/carryover/ui/i18n"
	"github.com/akfaiz/carryover/ui/kit"
	"github.com/akfaiz/carryover/ui/textutil"
)

// saveLocationRow shows one save location of a game on this device: its
// label and either the resolved folder (press to open), an error or a hint,
// and under it which files it syncs when its filters leave some out.
func (p *Page) saveLocationRow(c *ui.Context, sl services.SaveSlot, resolved, pathErr string) {
	th := c.Theme()
	filters := filterLine(sl.Include, sl.Exclude)
	row := ui.Row(c).Gap(kit.S3).AlignItems(ui.Center)
	if filters != "" {
		row.AlignItems(ui.Start) // the label stays level with the folder, not the two lines
	}
	row.Children(func() {
		name := ui.Text(c, textutil.LabelOf(sl.Label, sl.Key)).FontWeight(600).Width(110)
		if filters != "" {
			name.Padding(6, 0, 0, 0)
		}
		ui.Column(c).Grow(1).Shrink(1).Gap(kit.S1).Children(func() {
			switch {
			case pathErr != "":
				kit.Danger(c, pathErr)
			case resolved != "":
				// One line, cut at the end: the full path wrapped over three lines
				// and outshouted the page.
				kit.PathButton(c, p.u.Navigator(), [2]any{"folder", resolved}, resolved, func() { p.u.OpenFolder(resolved) })
			default:
				kit.Colored(c, i18n.L("not set on this device"), th.Warning)
			}
			if filters != "" {
				kit.Small(c, filters).Wrap()
			}
		})
	})
}

// filterLine says which files of a save location sync, from its include and
// exclude patterns: "" when it has none, so every file syncs.
func filterLine(include, exclude []string) string {
	var parts []string
	if len(include) > 0 {
		parts = append(parts, i18n.L("Only {{.Patterns}}", map[string]any{"Patterns": patternList(include)}))
	}
	if len(exclude) > 0 {
		parts = append(parts, i18n.L("Skips {{.Patterns}}", map[string]any{"Patterns": patternList(exclude)}))
	}
	return strings.Join(parts, " · ")
}

// patternList lists the first few patterns and says how many more there are,
// so a long filter doesn't take over the card (the editor shows them all).
func patternList(ps []string) string {
	const shown = 3
	if len(ps) <= shown {
		return strings.Join(ps, ", ")
	}
	more := len(ps) - shown
	return strings.Join(ps[:shown], ", ") + " " + i18n.N("+{{.Count}} more", more, map[string]any{"Count": more})
}

// savesCard shows the save locations of one profile and compares them with the
// cloud. With several profiles it also says how this one is doing, unless
// that reads exactly like the page header's line for the whole game.
func (p *Page) savesCard(c *ui.Context, pe profileEntry, busy, hasRemote, several bool, whole services.GameView) {
	g := pe.view
	st := p.vm.state(pe.id)
	th := c.Theme()
	kit.Card(c, "", func() {
		kit.CardHead(c, i18n.L("Saves"), func() {
			if !g.Ready {
				return // nothing to compare yet: the card offers the setup instead
			}
			label := i18n.L("Compare with cloud")
			if p.vm.busyFor("plan", pe.id) {
				label = i18n.L("Checking…")
			}
			p.btn(c, "compare:"+pe.id, kit.BtnSecondary, nil, label, busy || !hasRemote, func() { p.compare(pe.id) })
		})
		if several {
			// Only what differs from the header's line for the whole game: the
			// same hint twice reads like two problems.
			text, _ := components.StatusOf(g)
			wholeText, _ := components.StatusOf(whole)
			if text != wholeText || components.TimeAgo(g.Status.LastSyncAt) != components.TimeAgo(whole.Status.LastSyncAt) {
				ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
					components.StatusBadge(c, g)
					kit.Small(c, i18n.L("Last sync: {{.When}}", map[string]any{"When": components.TimeAgo(g.Status.LastSyncAt)}))
				})
			}
			if !g.Ready && (whole.Ready || g.SetupHint != whole.SetupHint) {
				kit.Callout(c, kit.BadgeWarn, kit.IconAlert, i18n.Text(g.SetupHint, g.SetupHintMsg), nil)
			}
			if g.Status.LastError != "" && !g.Status.Running && g.Status.LastError != whole.Status.LastError {
				kit.Callout(c, kit.BadgeErr, kit.IconAlert, i18n.Text(g.Status.LastError, g.Status.LastErrorMsg), nil)
			}
			if g.Status.LocalChanges && !g.Status.Running { // says which of the profiles changed
				kit.Callout(c, kit.BadgeWarn, kit.IconUpload, i18n.L("These saves changed on this device after the last sync. Use “Sync now” to upload them."), nil)
			}
		}
		for _, sl := range g.Slots {
			p.saveLocationRow(c, sl, g.ResolvedPaths[sl.Key], pathError(g, sl.Key))
		}
		if !g.Ready {
			p.btn(c, "setup:"+pe.id, kit.BtnPrimary, kit.IconPencil, i18n.L("Set up saves for this device"), false,
				func() { p.u.Navigate("/edit/" + p.vm.id) }).AlignSelf(ui.Start)
		}
		if p.vm.busyFor("plan", pe.id) {
			kit.BusyLine(c, i18n.L("Comparing with the cloud…"))
		}
		if pl := st.plan; pl != nil {
			ui.Divider(c)
			p.plan(c, pe.id, pl, pl.Action == "conflict" || g.Status.Conflict)
		} else if g.Status.Conflict {
			ui.Divider(c)
			kit.Colored(c, i18n.L("Unresolved save conflict. Compare with cloud to choose a version."), th.Warning)
		}
	})
}

// planText describes a sync plan action in words.
func planText(action string) string {
	switch action {
	case "none":
		return i18n.L("Local and cloud saves are identical.")
	case "push":
		return i18n.L("Local saves are newer — they will be uploaded.")
	case "pull":
		return i18n.L("Cloud saves are newer — they will be downloaded.")
	case "conflict":
		return i18n.L("Both changed since the last sync. Choose which to keep.")
	}
	return ""
}

// plan compares this device's saves with the cloud and, on a conflict, offers
// to keep either side.
func (p *Page) plan(c *ui.Context, prof string, pl *services.PlanView, resolvable bool) {
	var onKeep func(string)
	if resolvable {
		onKeep = func(resolution string) { p.doSync(prof, resolution) }
	}
	ui.Column(c).Gap(8).Children(func() {
		ui.Text(c, planText(pl.Action)).Bold()
		components.SaveSides(c, p.u.Navigator(), "plan:"+prof, pl.Local, pl.Remote, pl.HasRemote, onKeep)
	})
}

// compare compares a profile's saves with the cloud.
func (p *Page) compare(prof string) {
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
		p.u.Toast(i18n.L("Save conflict: choose which version to keep below"), kit.ToastErr)
	case syncBoth:
		p.u.Toast(i18n.L("Synced saves"), kit.ToastOK)
	case syncPushed:
		p.u.Toast(i18n.L("Uploaded saves"), kit.ToastOK)
	case syncPulled:
		p.u.Toast(i18n.L("Downloaded saves"), kit.ToastOK)
	default:
		p.u.Toast(i18n.L("Already up to date"), kit.ToastOK)
	}
	for _, prof := range profs {
		p.vm.state(prof).plan = nil
	}
	p.loadBackups(profs...)
	if conflict >= 0 {
		prof := profs[conflict]
		p.vm.selected, p.vm.chosen = prof, true
		p.compare(prof)
	}
}

// otherDevices lists the other devices that have the game, for the profile g
// is (the game's own saves or one emulator's): each with its save paths there,
// or a line saying it has none set. Nothing when no other device has the game.
func (p *Page) otherDevices(c *ui.Context, g services.GameView) {
	if len(g.OtherDevices) == 0 {
		return
	}
	kit.Card(c, i18n.L("Other devices"), func() {
		for i, d := range g.OtherDevices {
			// By its place too: two devices may have one name.
			kit.ListRow(c, [2]any{d.DeviceName, i}, i == 0, d.DeviceName, func() {
				set := false
				for _, s := range g.Slots {
					if v := d.Paths[s.Key]; v != "" {
						// Breakable: a folder named by a long id has no other place to wrap.
						kit.Small(c, textutil.LabelOf(s.Label, s.Key)+": "+textutil.BreakablePath(v)).Font("monospace").Wrap()
						set = true
					}
				}
				if !set {
					kit.Small(c, i18n.L("No save location set there"))
				}
			}, nil)
		}
	})
}

// pathError is why the save folder of a slot can't be resolved, "" if it can.
func pathError(g services.GameView, slot string) string {
	if m, ok := g.PathErrorMsgs[slot]; ok {
		return i18n.Msg(m)
	}
	return g.PathErrors[slot]
}
