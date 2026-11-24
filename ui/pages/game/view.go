package game

import (
	"errors"
	"runtime"
	"slices"
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/akfaiz/carryover/internal/cloudsync/syncer"
	"github.com/akfaiz/carryover/services"
	"github.com/akfaiz/carryover/ui/components"
	"github.com/akfaiz/carryover/ui/env"
	"github.com/akfaiz/carryover/ui/flows"
	"github.com/akfaiz/carryover/ui/i18n"
	"github.com/akfaiz/carryover/ui/kit"
	"github.com/akfaiz/carryover/ui/store"
	"github.com/akfaiz/carryover/ui/textutil"
)

// Page is a game: its saves and how they compare with the cloud, backups,
// the Steam shortcut and the play session.
type Page struct {
	u      env.Env
	vm     *ViewModel
	scroll ui.ScrollState
}

var (
	_ env.Syncable   = (*Page)(nil)
	_ env.Stateful   = (*Page)(nil)
	_ env.PadFocuser = (*Page)(nil)
)

// New returns the page of the game with that id.
func New(u env.Env, id string) *Page {
	p := &Page{u: u, vm: newViewModel(id, serviceBackend{u})}
	if g, ok := u.Store().Game(id); ok {
		p.vm.initFrom(g)
	}
	kit.Async(func() (services.SteamInfo, error) { return p.vm.be.SteamInfo(), nil }, func(info services.SteamInfo, _ error) {
		p.vm.steam = &info
		if len(info.Users) > 0 {
			p.vm.steamUser = info.Users[0].ID
		}
	})
	kit.Async(func() (string, error) { return p.vm.be.WrapLaunchOptions(p.vm.id) }, func(s string, err error) {
		if err == nil {
			p.vm.wrapOpts = s
		}
	})
	return p
}

// GameID is the game the page shows.
func (p *Page) GameID() string { return p.vm.id }

// Scroll is the page's scroll container, which the pad's right stick scrolls.
func (p *Page) Scroll() *ui.ScrollState { return &p.scroll }

// SelectedProfile is the id of the profile whose saves are shown ("" is the
// game's own saves), for the game as the library lists it.
func (p *Page) SelectedProfile(g services.GameView) string {
	return p.vm.selectedProfile(p.vm.profiles(g)).id
}

// DefaultFocus is where a gamepad starts: A there only goes back, so an A
// pressed once too often (the one that opened the game) never starts it.
func (p *Page) DefaultFocus() *ui.Handle { return p.u.Navigator().Handle("game-back") }

// SyncNow is F5: sync every profile that can be synced here.
func (p *Page) SyncNow() {
	g, ok := p.u.Store().Game(p.vm.id)
	if !ok || p.vm.busy != "" || p.u.Store().Settings.Remote == "" || g.Overall.Running {
		return
	}
	p.syncAll(p.vm.profiles(g))
}

// StateChanged: the page reads the store as it draws; it only has to notice
// that the session's sync phase ended.
func (p *Page) StateChanged(what store.Topic) {
	if what == store.Status && p.u.Store().Status[p.vm.id].Phase != "sync" && p.vm.busy == "" {
		p.vm.cancelling = false
	}
}

// sessionKey is the sync key of the play session the page shows: the profile
// that was started last.
func (p *Page) sessionKey() string {
	if st, ok := p.u.Store().Status[p.vm.id]; ok && st.Key != "" {
		return st.Key
	}
	return p.vm.id
}

// button is a button that registers with the gamepad. key tells it from the
// others on the page.
func (p *Page) button(c *ui.Context, key any, label string, primary, disabled bool, action func()) {
	nav := p.u.Navigator()
	k := [2]any{"game", key}
	var b ui.Element
	if primary {
		b = ui.PrimaryButton(c.Key(k), label)
	} else {
		b = ui.Button(c.Key(k), label)
	}
	if nav.Item(nav.Handle(k), b.Disabled(disabled), action).Clicked() && !disabled {
		action()
	}
}

// View draws the page.
func (p *Page) View(c *ui.Context) {
	u, nav := p.u, p.u.Navigator()
	ui.Scroll(c).Grow(1).TrackScroll(&p.scroll).Children(func() {
		ui.Column(c).Gap(10).Padding(8, 20, 20, 20).Children(func() {
			orig, ok := u.Store().Game(p.vm.id)
			if !ok {
				msg := i18n.L("Loading…")
				if u.Store().Loaded {
					msg = i18n.L("Game not found.")
				}
				kit.PageHead(c, nav, nav.Handle("game-back"), u.Back, "", nil)
				kit.Muted(c, msg)
				return
			}
			p.game(c, orig)
		})
	})
}

func (p *Page) game(c *ui.Context, orig services.GameView) {
	u, nav := p.u, p.u.Navigator()
	g := orig
	g.Status = g.Overall // the game as a whole: its own saves and every emulator
	profs := p.vm.profiles(orig)
	busy := p.vm.busy != ""
	hasRemote := u.Store().Settings.Remote != ""

	// Back on its own row at the top left, where other pages have it.
	kit.PageHead(c, nav, nav.Handle("game-back"), u.Back, "", nil)

	ui.Row(c).Gap(16).AlignItems(ui.Start).Children(func() {
		// A smaller cover than the library's leaves room for the saves on a Steam
		// Deck's 800 px high screen.
		kit.CoverView(c, kit.CoverSource{Path: g.CoverPath, Version: g.CoverVersion, Name: g.Name}, 120, 180)
		ui.Column(c).Grow(1).Gap(8).Children(func() { p.hero(c, orig, g, profs, busy, hasRemote) })
	})

	// The saves and backups of one profile; a game with emulators switches
	// between them above both columns, so the columns start level.
	sel := p.vm.selectedProfile(profs)
	if len(profs) > 1 {
		p.profileSwitcher(c, profs, sel)
	}
	ui.Row(c).Gap(16).Wrap().AlignItems(ui.Start).Children(func() {
		ui.Column(c).Grow(3).MinWidth(420).Gap(10).Children(func() {
			p.savesCard(c, sel, busy, hasRemote, len(profs) > 1, g)
			p.backupsCard(c, sel, busy, hasRemote)
		})
		ui.Column(c).Grow(2).MinWidth(300).Gap(10).Children(func() {
			p.steamCard(c, orig, busy)
			p.otherDevices(c, g)
		})
	})
}

// hero is the name, status and the buttons at the top.
func (p *Page) hero(c *ui.Context, orig, g services.GameView, profs []profileEntry, busy, hasRemote bool) {
	u := p.u
	kit.Title(c, g.Name).Wrap()
	ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
		components.StatusBadge(c, g)
		kit.Small(c, i18n.L("Last sync: {{.When}}", map[string]any{"When": components.TimeAgo(g.Status.LastSyncAt)}))
	})
	hasPaths := slices.ContainsFunc(g.OtherDevices, func(d services.DevicePaths) bool {
		return slices.ContainsFunc(g.Slots, func(s services.SaveSlot) bool { return d.Paths[s.Key] != "" })
	})
	if !hasPaths && len(g.OtherDevices) > 0 {
		names := make([]string, 0, len(g.OtherDevices))
		for _, d := range g.OtherDevices {
			names = append(names, d.DeviceName)
		}
		kit.Small(c, i18n.L("Also on {{.Devices}}", map[string]any{"Devices": strings.Join(names, ", ")}))
	}
	th := c.Theme()
	if !g.Ready {
		kit.Colored(c, g.SetupHint, th.Warning)
	}
	if g.Status.LastError != "" && !g.Status.Running {
		kit.Danger(c, g.Status.LastError)
	}
	if g.Status.LocalChanges && !g.Status.Running {
		kit.Colored(c, i18n.L("The saves on this device changed after the last sync, so the game was probably played without Carryover. Use “Sync now” to upload them."), th.Warning)
	}
	steamLaunched := g.Device.LaunchMode == "steamwrap"
	ui.Row(c).Gap(8).Wrap().Children(func() {
		switch {
		case g.Status.Running:
			p.button(c, "play", i18n.L("● Running"), true, true, func() {})
		case g.Ready && steamLaunched:
			p.button(c, "play", i18n.L("Play in Steam"), true, services.SteamRunURL(orig) == "", func() { flows.OpenSteam(u, orig) })
		case g.Ready:
			// A game that can be played in several ways asks which one first.
			p.button(c, "play", i18n.L("Play"), true, busy, func() { p.run("play", func() error { return p.vm.be.Play(p.vm.id) }, nil) })
		default:
			p.button(c, "play", i18n.L("Set up for this device"), true, false, func() { u.Navigate("/edit/" + p.vm.id) })
		}
		syncLabel := i18n.L("Sync now")
		if p.vm.busy == "sync" {
			syncLabel = i18n.L("Syncing…")
		}
		p.button(c, "sync", syncLabel, false, busy || !canSync(profs) || !hasRemote || g.Status.Running, func() { p.syncAll(profs) })
		p.button(c, "edit", i18n.L("Edit"), false, false, func() { u.Navigate("/edit/" + p.vm.id) })
		p.button(c, "more", "⋯", false, false, func() { p.showMore(orig, busy) })
	})
	if steamLaunched && g.Ready && services.SteamRunURL(orig) == "" {
		kit.Muted(c, i18n.L("This game is launched from Steam on this device (Proton). Use “Add to Steam” below, then start it from Steam."))
	}
	if p.vm.busy == "delete" {
		kit.BusyLine(c, i18n.L("Deleting…"))
	}
	p.session(c, g)
}

// session shows the running play session, or the transfer of a sync started
// here, with its Cancel button.
func (p *Page) session(c *ui.Context, g services.GameView) {
	sessKey := p.sessionKey()
	st, hasSt := p.u.Store().Status[p.vm.id]
	ev := p.u.Store().Progress[sessKey]
	switch {
	case hasSt && (st.Phase != "done" || g.Status.Running) && st.Phase != "error":
		kit.Card(c, "", func() {
			ui.Text(c, st.Message).Bold().Wrap()
			if st.Phase != "playing" && st.Phase != "choose" {
				kit.ProgressLine(c, "", ev.Bytes, ev.TotalBytes)
			}
			if st.Phase == "sync" {
				// Before the game starts this cancels the launch; afterwards the upload.
				label := i18n.L("Cancel upload")
				if st.BeforePlay {
					label = i18n.L("Cancel and don't play")
				}
				p.cancelButton(c, label, func() bool { return p.vm.be.CancelPlay(sessKey) })
			}
		}).MaxWidth(480)
	case p.vm.busy == "sync":
		p.transferCard(c).MaxWidth(480)
	}
}

// showMore is the menu behind the "⋯" button: open the save folders, and
// delete the game.
func (p *Page) showMore(g services.GameView, busy bool) {
	var d *kit.Dialog
	d = p.u.Dialogs().Show(&kit.Dialog{Title: i18n.L("More"), Width: 380, Body: func(c *ui.Context, nav *kit.Navigator) {
		item := func(key any, label string, danger, disabled bool, action func()) {
			k := [2]any{"game-more", key}
			b := ui.Button(c.Key(k), label).Disabled(disabled).AlignSelf(ui.Stretch)
			if danger {
				b = b.TextColor(c.Theme().Danger)
			}
			do := func() { d.Close(); action() }
			if nav.Item(nav.Handle(k), b, do).Clicked() && !disabled {
				do()
			}
		}
		for _, sl := range g.Slots {
			if dir := g.ResolvedPaths[sl.Key]; dir != "" && g.PathErrors[sl.Key] == "" {
				label := i18n.L("Open save folder")
				if len(g.Slots) > 1 {
					label = i18n.L("Open save folder: {{.Label}}", map[string]any{"Label": textutil.LabelOf(sl.Label, sl.Key)})
				}
				item(sl.Key, label, false, false, func() { p.u.OpenFolder(dir) })
			}
		}
		item("delete", i18n.L("Delete from library…"), true, busy || g.Overall.Running, p.askDelete)
	}})
}

// askDelete asks whether to move the game to the trash or delete it with its
// cloud saves, then does it.
func (p *Page) askDelete() {
	g, ok := p.u.Store().Game(p.vm.id)
	if !ok {
		return
	}
	components.ConfirmDanger(p.u, components.ConfirmOpts{
		Title:    i18n.L("Delete game"),
		Message:  i18n.L("Move {{.Game}} to the trash on all devices? You can restore it from the Trash page. Cloud saves and backups are kept.", map[string]any{"Game": g.Name}),
		Check:    i18n.L("Also delete its cloud saves and backups"),
		Checked:  i18n.L("Delete {{.Game}} permanently on all devices, with its cloud saves and backups (for every emulator too)? It can't be restored.", map[string]any{"Game": g.Name}),
		KeptHint: i18n.L("The cloud saves stay until you delete them. Once it is gone for good, they are listed under Trash → Leftover cloud saves."),
		OnConfirm: func(saves bool) {
			p.run("delete", func() error { return p.vm.be.Delete(p.vm.id, saves) }, func(err error) {
				if err == nil {
					p.u.Replace("/") // the game's page is gone: Back shouldn't return to it
				}
			})
		},
	})
}

// tagOf names a profile on the switcher: the emulator, or the game's own saves.
func tagOf(pe profileEntry) string {
	if pe.noEmu {
		return i18n.L("Without an emulator")
	}
	return pe.tag
}

// profileSwitcher is a row of buttons, one per way to play the game, each
// marked with its sync state; the selected one's saves are shown below.
func (p *Page) profileSwitcher(c *ui.Context, profs []profileEntry, sel profileEntry) {
	var opts []kit.ChipOption
	for _, pe := range profs {
		_, kind := components.StatusOf(pe.view)
		mark := ""
		switch {
		case pe.view.Status.Running:
			mark = "▶ "
		case kind == kit.BadgeOK:
			mark = "✓ "
		case kind == kit.BadgeWarn:
			mark = "⚠ "
		case kind == kit.BadgeErr:
			mark = "✕ "
		}
		opts = append(opts, kit.ChipOption{Value: pe.id, Label: mark + tagOf(pe)})
	}
	ui.Column(c).Gap(4).Children(func() {
		kit.Small(c, i18n.L("Saves of"))
		kit.Chips(c, p.u.Navigator(), "profile", opts, sel.id, func(id string) { p.vm.selected, p.vm.chosen = id, true })
	})
}

func (p *Page) updateProgressText(label string) string {
	ev := p.u.Store().Progress[p.vm.key()]
	if ev.Message != "" {
		return ev.Message
	}
	switch p.vm.busy {
	case "restore":
		return i18n.L("Restoring backup…")
	case "backup":
		return i18n.L("Backing up…")
	}
	return label
}

// run marks the page busy while work runs.
func (p *Page) run(label string, work func() error, done func(error)) {
	p.vm.busy = label
	if label == "sync" || label == "restore" || label == "backup" {
		p.u.Store().ClearProgress(p.vm.key())
	}
	kit.Async0(work, func(err error) {
		p.vm.busy = ""
		p.vm.cancelling = false
		switch {
		case errors.Is(err, services.ErrSyncCancelled):
			p.u.Toast(i18n.L("Sync cancelled. Files already transferred are kept; the next sync continues."), kit.ToastInfo)
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
	})
	kit.Redraw()
}

// transferCard is the progress of a sync, restore or backup with its Cancel
// button; it goes where the action was started.
func (p *Page) transferCard(c *ui.Context) ui.Element {
	return kit.Card(c, "", func() {
		ev := p.u.Store().Progress[p.vm.key()]
		kit.ProgressLine(c, p.updateProgressText(i18n.L("Syncing…")), ev.Bytes, ev.TotalBytes)
		p.cancelButton(c, i18n.L("Cancel"), func() bool { return p.vm.be.CancelSync(p.vm.key()) })
	})
}

// cancelButton stops a running sync; it shows "Cancelling…" until it stops.
func (p *Page) cancelButton(c *ui.Context, label string, cancel func() bool) {
	if p.vm.cancelling {
		p.button(c, "cancel", i18n.L("Cancelling…"), false, true, func() {})
		return
	}
	p.button(c, "cancel", label, false, false, func() {
		if cancel() {
			p.vm.cancelling = true
		}
	})
}

// saveLocationRow shows one save location of a game on this device: its
// label and either the resolved folder (press to open), an error or a hint.
func (p *Page) saveLocationRow(c *ui.Context, label, resolved, pathErr string) {
	nav := p.u.Navigator()
	th := c.Theme()
	ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
		ui.Text(c, label).Bold().Width(110)
		switch {
		case pathErr != "":
			kit.Danger(c, pathErr)
		case resolved != "":
			// One line, cut at the end: the full path wrapped over three lines and
			// outshouted the page.
			k := [2]any{"folder", resolved}
			open := func() { p.u.OpenFolder(resolved) }
			b := ui.Button(c.Key(k), "📁 "+resolved).Label(resolved).Grow(1).Ellipsis("…").SingleLine().Font("monospace").FontSize(12)
			if nav.Item(nav.Handle(k), b, open).Clicked() {
				open()
			}
		default:
			kit.Colored(c, i18n.L("not set on this device"), th.Warning)
		}
	})
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
			label := i18n.L("Compare with cloud")
			if p.vm.busyFor("plan", pe.id) {
				label = i18n.L("Checking…")
			}
			p.button(c, "compare:"+pe.id, label, false, busy || !g.Ready || !hasRemote, func() { p.check(pe.id) })
		})
		if several {
			text, _ := components.StatusOf(g)
			wholeText, _ := components.StatusOf(whole)
			if text != wholeText || components.TimeAgo(g.Status.LastSyncAt) != components.TimeAgo(whole.Status.LastSyncAt) {
				ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
					components.StatusBadge(c, g)
					kit.Small(c, i18n.L("Last sync: {{.When}}", map[string]any{"When": components.TimeAgo(g.Status.LastSyncAt)}))
				})
			}
			if !g.Ready {
				kit.Colored(c, g.SetupHint, th.Warning)
			}
			if g.Status.LastError != "" && !g.Status.Running {
				kit.Danger(c, g.Status.LastError)
			}
			if g.Status.LocalChanges && !g.Status.Running {
				kit.Colored(c, i18n.L("These saves changed on this device after the last sync. Use “Sync now” to upload them."), th.Warning)
			}
		}
		for _, sl := range g.Slots {
			p.saveLocationRow(c, textutil.LabelOf(sl.Label, sl.Key), g.ResolvedPaths[sl.Key], g.PathErrors[sl.Key])
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
		p.check(prof)
	}
}

// otherDevices lists the save paths other devices use for the game; nothing
// when they have none to show.
func (p *Page) otherDevices(c *ui.Context, g services.GameView) {
	if !slices.ContainsFunc(g.OtherDevices, func(d services.DevicePaths) bool {
		return slices.ContainsFunc(g.Slots, func(s services.SaveSlot) bool { return d.Paths[s.Key] != "" })
	}) {
		return
	}
	kit.Card(c, i18n.L("Other devices"), func() {
		for _, d := range g.OtherDevices {
			ui.Text(c, d.DeviceName).Bold()
			for _, s := range g.Slots {
				if v := d.Paths[s.Key]; v != "" {
					kit.Small(c, textutil.LabelOf(s.Label, s.Key)+": "+v).Font("monospace").TextColor(c.Theme().TextMuted)
				}
			}
		}
	})
}

// backupRow is one backup with Pin, Delete and Restore buttons (no Pin for a
// safety copy kept on this device).
func (p *Page) backupRow(c *ui.Context, prof string, b syncer.Backup, restoring, canRestore, canPin bool) {
	var desc string
	switch {
	case b.Local:
		desc = i18n.L("{{.Device}}'s saves (copy kept on this device)", map[string]any{"Device": b.DeviceName})
	case b.Kind == "local":
		desc = i18n.L("{{.Device}}'s local saves", map[string]any{"Device": b.DeviceName})
	case b.Kind == "manual":
		desc = i18n.L("{{.Device}}'s saves (manual backup)", map[string]any{"Device": b.DeviceName})
	default:
		desc = i18n.L("cloud saves (replaced by {{.Device}})", map[string]any{"Device": b.DeviceName})
	}
	pinLabel := i18n.L("Pin")
	if b.Pinned {
		desc += " · " + i18n.L("pinned")
		pinLabel = i18n.L("Unpin")
	}
	label := i18n.L("Restore")
	if restoring {
		label = "…"
	}
	key := prof + "/" + b.Name
	ui.Row(c).Gap(8).AlignItems(ui.Center).Wrap().Children(func() {
		ui.Text(c, components.FmtDate(b.Time)).Width(140)
		kit.Muted(c, desc).Grow(1)
		if !b.Local {
			p.button(c, "pin:"+key, pinLabel, false, !canPin, func() { p.setPinned(prof, b) })
		}
		p.button(c, "del:"+key, "🗑", false, !canPin, func() { p.deleteBackup(prof, b) })
		p.button(c, "restore:"+key, label, false, !canRestore, func() { p.restore(prof, b) })
	})
}

// backupsCard lists and takes the backups of one profile's cloud save. The
// list loads by itself the first time the card is shown.
func (p *Page) backupsCard(c *ui.Context, pe profileEntry, busy, hasRemote bool) {
	g := pe.view
	st := p.vm.state(pe.id)
	if hasRemote && !st.backupsLoaded && !st.backupsLoading {
		p.loadBackups(pe.id)
	}
	summary := i18n.L("Cloud backups are off. A copy of this device's saves is kept here before a download replaces them.")
	details := []string{i18n.L("Cloud backups are off. Before a download replaces this device's saves, a copy is kept on this device (the last {{.Count}}). Turn on cloud backups in Settings to keep backups in the cloud.",
		map[string]any{"Count": syncer.SafetyKeep})}
	if n := p.u.Store().Settings.BackupRetention; n > 0 {
		summary = i18n.N("Cloud backups are on: the last {{.Count}} backup is kept.", n, map[string]any{"Count": n})
		details = []string{i18n.N("A backup is taken in the cloud whenever saves are about to be overwritten (keeping the last {{.Count}} backup).", n,
			map[string]any{"Count": n})}
	}
	details = append(details, i18n.L("Backups you make with “Back up now” are pinned. Pinned backups are never deleted automatically."))
	kit.Card(c, "", func() {
		kit.CardHead(c, i18n.L("Backups"), func() {
			label := i18n.L("Back up now")
			if p.vm.busyFor("backup", pe.id) {
				label = i18n.L("Backing up…")
			}
			p.button(c, "backup-now:"+pe.id, label, false, busy || !hasRemote || !g.Ready || g.Status.Running, func() { p.backupNow(pe.id) })
			p.button(c, "backups-refresh:"+pe.id, "⟳", false, !hasRemote || st.backupsLoading, func() { p.loadBackups(pe.id) })
		})
		components.Help(c, p.u, summary, details...)
		if g.SafetyDir != "" {
			dir := g.SafetyDir
			p.button(c, "safety:"+pe.id, i18n.L("Open copies on this device"), false, false, func() { p.u.OpenFolder(dir) })
		}
		if (p.vm.busy == "restore" || p.vm.busy == "backup") && p.vm.activeProfile() == pe.id {
			p.transferCard(c)
		}
		if p.vm.busyFor("pin", pe.id) {
			kit.BusyLine(c, i18n.L("Updating backups…"))
		}
		switch {
		case !hasRemote:
			kit.Muted(c, i18n.L("Set up cloud storage to keep backups in the cloud."))
		case st.backupsLoading && !st.backupsLoaded:
			kit.BusyLine(c, i18n.L("Loading backups…"))
		case st.backupsErr != "":
			kit.Danger(c, st.backupsErr)
		case len(st.backups) == 0:
			kit.Muted(c, i18n.L("No backups yet."))
		}
		for _, b := range st.backups {
			p.backupRow(c, pe.id, b, p.vm.busyFor("restore", pe.id), !busy && g.Ready, !busy)
		}
	})
}

// loadBackups reads the backup lists of the profiles in the background: it
// doesn't make the page busy, so Play and Sync stay available meanwhile.
func (p *Page) loadBackups(profs ...string) {
	if p.u.Store().Settings.Remote == "" {
		return
	}
	for _, prof := range profs {
		st := p.vm.state(prof)
		if st.backupsLoading {
			continue
		}
		st.backupsLoading = true
		kit.Async(func() ([]syncer.Backup, error) { return p.vm.LoadBackups(prof) }, func(list []syncer.Backup, err error) {
			st.backupsLoading, st.backupsLoaded, st.backupsErr = false, true, ""
			if err != nil {
				st.backupsErr = err.Error()
			} else {
				st.backups = list
			}
		})
	}
}

// backupNow stores a pinned backup of this device's saves.
func (p *Page) backupNow(prof string) {
	p.vm.setActive(prof)
	p.run("backup", func() error { return p.vm.BackupNow(prof) }, func(err error) {
		if err == nil {
			p.u.Toast(i18n.L("Backup saved and pinned"), kit.ToastOK)
			p.loadBackups(prof)
		}
	})
}

// deleteBackup asks, then permanently deletes a backup for every device.
func (p *Page) deleteBackup(prof string, b syncer.Backup) {
	msg := i18n.L("Delete the backup from {{.When}}? It is shared with all your devices and can't be recovered.", map[string]any{"When": components.FmtDate(b.Time)})
	if b.Pinned {
		msg = i18n.L("This backup is pinned. Delete the backup from {{.When}} anyway? It is shared with all your devices and can't be recovered.", map[string]any{"When": components.FmtDate(b.Time)})
	}
	components.ConfirmDanger(p.u, components.ConfirmOpts{Title: i18n.L("Delete backup"), Message: msg, OnConfirm: func(bool) {
		p.vm.setActive(prof)
		p.run("pin", func() error { return p.vm.DeleteBackup(prof, b) }, func(err error) {
			if err == nil {
				p.u.Toast(i18n.L("Backup deleted"), kit.ToastOK)
				p.loadBackups(prof)
				p.u.Store().Refresh() // the last copy on this device may be gone
			}
		})
	}})
}

func (p *Page) setPinned(prof string, b syncer.Backup) {
	p.vm.setActive(prof)
	p.run("pin", func() error { return p.vm.TogglePinned(prof, b) }, func(err error) {
		if err == nil {
			p.loadBackups(prof)
		}
	})
}

// restore asks, then puts a backup back on this device and in the cloud.
func (p *Page) restore(prof string, b syncer.Backup) {
	components.ConfirmDanger(p.u, components.ConfirmOpts{
		Title:   i18n.L("Restore backup"),
		Message: i18n.L("Replace the saves on this device and in the cloud with the backup from {{.When}}? The saves on this device are backed up first.", map[string]any{"When": components.FmtDate(b.Time)}),
		Action:  i18n.L("Restore"),
		OnConfirm: func(bool) {
			p.vm.setActive(prof)
			p.run("restore", func() error { return p.vm.RestoreBackup(prof, b) }, func(err error) {
				if err == nil {
					p.u.Toast(i18n.L("Backup restored and uploaded"), kit.ToastOK)
					p.loadBackups(prof)
				}
			})
		},
	})
}

// steamCard is the Steam shortcut form. A game with emulators gets one
// shortcut for the whole game by default; it asks which emulator to use when it
// starts. The shortcut of one emulator is the other choice.
func (p *Page) steamCard(c *ui.Context, orig services.GameView, busy bool) {
	u := p.u
	sc := p.vm.steamChoice(orig)
	key, form, target, asks, hideMode := sc.key, sc.form, sc.target, sc.asks, sc.hideMode
	kit.Card(c, i18n.L("Steam"), func() {
		p.steamForm(c, steamFormOpts{
			Game: form, Info: p.vm.steam, User: p.vm.steamUser, Mode: p.vm.steamMode, Busy: busy,
			Profiles: orig.Emulators, Target: target, AsksAtLaunch: asks, HideMode: hideMode,
			Expanded: p.vm.steamExpanded,
			OnExpand: func() { p.vm.steamExpanded = true },
			OnTarget: func(v string) { p.vm.steamTarget = v },
			OnUser:   func(v string) { p.vm.steamUser = v },
			OnMode:   func(v string) { p.vm.steamMode = v },
			OnAdd: func() {
				var res services.AddToSteamResult
				p.run("steam", func() error {
					var err error
					res, err = p.vm.AddToSteam(key, hideMode)
					return err
				}, func(err error) {
					if err != nil {
						return
					}
					p.vm.steamExpanded = false
					if res.SteamRunning {
						u.Toast(i18n.L("Added to Steam. Restart Steam to see it."), kit.ToastOK)
					} else {
						u.Toast(i18n.L("Added to Steam"), kit.ToastOK)
					}
				})
			},
			OnRemove: func() {
				var n int
				p.run("steam", func() error {
					var err error
					n, err = p.vm.RemoveFromSteam(key)
					return err
				}, func(err error) {
					if err == nil && n > 0 {
						u.Toast(i18n.L("Removed from Steam"), kit.ToastOK)
					} else if err == nil {
						u.Toast(i18n.L("No shortcut found"), kit.ToastOK)
					}
				})
			},
		})
		if p.vm.busy == "steam" {
			kit.BusyLine(c, i18n.L("Updating Steam…"))
		}
		if p.vm.wrapOpts != "" {
			ui.Collapsible(c.Key("wrap-opts"), i18n.L("Already own it on Steam?"), &p.vm.wrapOpen, func() {
				kit.Muted(c, i18n.L("Paste this into the game's Steam launch options to sync around Steam's own launch:"))
				components.CopyField(c, u, p.vm.wrapOpts)
			})
		}
	})
}

// steamFormOpts configures the Steam form. Info is nil while Steam is still
// being detected.
type steamFormOpts struct {
	Game       services.GameView
	Info       *services.SteamInfo
	User, Mode string // selected Steam account and shortcut type
	Busy       bool

	// For a game with emulators. Profiles are its emulators; Target is "game"
	// (one shortcut for the whole game, which asks which emulator to use when it
	// starts) or the ID of the emulator the shortcut is for. AsksAtLaunch is set
	// when the shortcut is the whole-game one. HideMode hides the shortcut type:
	// a game played only through emulators has just the direct one.
	Profiles     []services.ProfileView
	Target       string
	AsksAtLaunch bool
	HideMode     bool

	// Once the shortcut exists the form folds into one line with Change and
	// Remove; Expanded shows it in full again (OnExpand asks for that).
	Expanded bool
	OnExpand func()

	OnUser   func(string)
	OnTarget func(string)
	OnMode   func(string)
	OnAdd    func()
	OnRemove func()
}

// steamForm is the "add to Steam as a non-Steam game" form: account, shortcut
// type and the Add / Update / Remove buttons.
func (p *Page) steamForm(c *ui.Context, o steamFormOpts) {
	nav, th := p.u.Navigator(), c.Theme()
	g := o.Game
	targets := func() {
		if len(o.Profiles) == 0 {
			return
		}
		ts := &kit.Choice{}
		ts.Add("game", i18n.L("The whole game: asks which emulator to use when it starts"))
		for _, pv := range o.Profiles {
			ts.Add(pv.EmulatorID, i18n.L("Only {{.Emulator}}", map[string]any{"Emulator": pv.Name}))
		}
		kit.Labeled(c, i18n.L("Shortcut for"), func() {
			kit.SelectOf(c, nav, p.u.Dialogs(), "steam-target", i18n.L("Shortcut for"), ts, o.Target, o.OnTarget)
		})
	}
	remove := func() { p.button(c, "steam-remove", i18n.L("Remove"), false, o.Busy, o.OnRemove) }
	switch {
	case o.Info == nil:
		kit.Muted(c, i18n.L("Detecting Steam…"))
	case !o.Info.Found:
		kit.Muted(c, i18n.L("Steam was not found on this device."))
	case g.Device.SteamAppID != "" && !o.Expanded:
		kit.Colored(c, "✓ "+i18n.L("In Steam on this device"), th.Success)
		ui.Row(c).Gap(8).Children(func() {
			p.button(c, "steam-change", i18n.L("Change…"), false, o.Busy, o.OnExpand)
			remove()
		})
		targets()
	default:
		kit.Muted(c, i18n.L("Add {{.Game}} to Steam as a non-Steam game. Steam launches it through Carryover, so saves sync automatically before and after every session — also in Steam Deck Game Mode.",
			map[string]any{"Game": g.Name}))
		if len(o.Info.Users) > 1 {
			users := &kit.Choice{}
			for _, us := range o.Info.Users {
				users.Add(us.ID, us.Name)
			}
			kit.Labeled(c, i18n.L("Steam account"), func() {
				kit.SelectOf(c, nav, p.u.Dialogs(), "steam-user", i18n.L("Steam account"), users, o.User, o.OnUser)
			})
		}
		targets()
		if o.AsksAtLaunch {
			kit.Muted(c, i18n.L("Steam shows one game. When it starts, Carryover asks which emulator to use and, after a few seconds, starts the one you played last."))
		}
		switch {
		case o.HideMode: // only the direct shortcut exists here
		case g.Device.LaunchMode == "heroic":
			kit.Muted(c, i18n.L("Steam runs Carryover, which starts the game through Heroic Games Launcher, so it keeps Heroic's settings (and, on Linux, its Wine prefix). Don't force a Proton version on this shortcut."))
		case g.Device.LaunchMode == "lutris":
			kit.Muted(c, i18n.L("Steam runs Carryover, which starts the game through Lutris, so it keeps Lutris's runner and Wine prefix. Don't force a Proton version on this shortcut."))
		default:
			modes := &kit.Choice{}
			modes.Add("direct", i18n.L("Run Carryover (it starts the game)"))
			if runtime.GOOS == "darwin" { // no Proton on macOS
				modes.Add("steamwrap", i18n.L("Run the game, wrapped by Carryover"))
			} else {
				modes.Add("steamwrap", i18n.L("Run the game exe, wrapped by Carryover (pick Proton in Steam)"))
			}
			kit.Labeled(c, i18n.L("Shortcut type"), func() {
				kit.SelectOf(c, nav, p.u.Dialogs(), "steam-mode", i18n.L("Shortcut type"), modes, o.Mode, o.OnMode)
			})
		}
		if o.Mode == "steamwrap" && g.Device.LaunchMode != "heroic" && g.Device.LaunchMode != "lutris" && runtime.GOOS != "darwin" {
			t := i18n.L("After adding, open the shortcut's Properties → Compatibility in Steam and force a Proton version. Its save prefix is available as {{.Token}}.",
				map[string]any{"Token": "{proton:shortcut}"})
			if g.Device.SteamAppID != "" {
				t = i18n.L("After adding, open the shortcut's Properties → Compatibility in Steam and force a Proton version. Its save prefix is available as {{.Token}} (app id {{.AppID}}).",
					map[string]any{"Token": "{proton:shortcut}", "AppID": g.Device.SteamAppID})
			}
			kit.Muted(c, t)
		}
		addLabel := i18n.L("Add to Steam")
		if g.Device.SteamAppID != "" {
			addLabel = i18n.L("Update Steam shortcut")
		}
		ui.Row(c).Gap(8).Children(func() {
			p.button(c, "steam-add", addLabel, true, o.Busy || o.User == "", o.OnAdd)
			if g.Device.SteamAppID != "" {
				remove()
			}
		})
		if o.Info.Running {
			kit.Colored(c, i18n.L("Steam is running — restart it after adding so it picks up the change."), th.Warning)
		}
	}
}
