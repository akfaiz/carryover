package game

import (
	"github.com/egoist/mygo/ui"

	"github.com/akfaiz/carryover/services"
	"github.com/akfaiz/carryover/ui/components"
	"github.com/akfaiz/carryover/ui/flows"
	"github.com/akfaiz/carryover/ui/i18n"
	"github.com/akfaiz/carryover/ui/kit"
	"github.com/akfaiz/carryover/ui/textutil"
)

// hero is the name, status and the buttons at the top.
func (p *Page) hero(c *ui.Context, orig, g services.GameView, profs []profileEntry, busy, hasRemote bool) {
	u := p.u
	kit.HeroTitle(c, g.Name).Wrap()
	ui.Row(c).Gap(kit.S2).AlignItems(ui.Center).Children(func() {
		components.StatusBadge(c, g)
		kit.Small(c, i18n.L("Last sync: {{.When}}", map[string]any{"When": components.TimeAgo(g.Status.LastSyncAt)}))
	})
	// Other devices are listed in their own card, for the profile shown.
	// What needs the user's eye, each in a callout. None repeats a button: the
	// buttons under them do what they ask.
	if !g.Ready {
		kit.Callout(c, kit.BadgeWarn, kit.IconAlert, i18n.Text(g.SetupHint, g.SetupHintMsg), nil)
	}
	if g.Status.LastError != "" && !g.Status.Running {
		kit.Callout(c, kit.BadgeErr, kit.IconAlert, i18n.Text(g.Status.LastError, g.Status.LastErrorMsg), nil)
	}
	if g.Status.LocalChanges && !g.Status.Running {
		kit.Callout(c, kit.BadgeWarn, kit.IconUpload, i18n.L("The saves on this device changed after the last sync, so the game was probably played without Carryover. Use “Sync now” to upload them."), nil)
	}
	steamLaunched := g.Device.LaunchMode == "steamwrap"
	ui.Row(c).Gap(kit.S2).Wrap().AlignItems(ui.Center).Padding(kit.S1, 0, 0, 0).Children(func() {
		large := kit.BtnLarge()
		switch {
		case g.Status.Running:
			kit.Btn(c, u.Navigator(), [2]any{"game", "play"}, kit.BtnSecondary, kit.IconDot, i18n.L("Running"), func() {}, kit.BtnDisabled(true), large)
		case g.Ready && steamLaunched:
			kit.Btn(c, u.Navigator(), [2]any{"game", "play"}, kit.BtnPrimary, kit.IconPlay, i18n.L("Play in Steam"), func() { flows.OpenSteam(u, orig) },
				kit.BtnDisabled(services.SteamRunURL(orig) == ""), large)
		case g.Ready:
			// A game that can be played in several ways asks which one first.
			kit.Btn(c, u.Navigator(), [2]any{"game", "play"}, kit.BtnPrimary, kit.IconPlay, i18n.L("Play"),
				func() { p.run("play", func() error { return p.vm.be.Play(p.vm.id) }, nil) }, kit.BtnDisabled(busy), large)
		default:
			kit.Btn(c, u.Navigator(), [2]any{"game", "play"}, kit.BtnPrimary, kit.IconPencil, i18n.L("Set up for this device"),
				func() { u.Navigate("/edit/" + p.vm.id) }, large)
		}
		syncLabel := i18n.L("Sync now")
		if p.vm.busy == "sync" {
			syncLabel = i18n.L("Syncing…")
		}
		p.btn(c, "sync", kit.BtnSecondary, kit.IconRefresh, syncLabel, busy || !canSync(profs) || !hasRemote || g.Status.Running, func() { p.syncAll(profs) })
		p.btn(c, "edit", kit.BtnSecondary, kit.IconPencil, i18n.L("Edit"), false, func() { u.Navigate("/edit/" + p.vm.id) })
		kit.MenuButton(c, u.Navigator(), [2]any{"game", "more"}, kit.BtnSecondary, kit.IconMore, i18n.L("More"), p.moreMenu(orig, busy))
	})
	if steamLaunched && g.Ready && services.SteamRunURL(orig) == "" {
		kit.Callout(c, kit.BadgeInfo, kit.IconInfo, i18n.L("This game is launched from Steam on this device (Proton). Use “Add to Steam” below, then start it from Steam."), nil)
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
			ui.Text(c, i18n.Text(st.Message, st.Msg)).Bold().Wrap()
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

// moreMenu is the menu behind the "⋯" button: open the save folders, and
// delete the game.
func (p *Page) moreMenu(g services.GameView, busy bool) []kit.MenuItem {
	var items []kit.MenuItem
	for _, sl := range g.Slots {
		if dir := g.ResolvedPaths[sl.Key]; dir != "" && g.PathErrors[sl.Key] == "" {
			label := i18n.L("Open save folder")
			if len(g.Slots) > 1 {
				label = i18n.L("Open save folder: {{.Label}}", map[string]any{"Label": textutil.LabelOf(sl.Label, sl.Key)})
			}
			items = append(items, kit.MenuItem{Key: [2]any{"slot", sl.Key}, Label: label, Icon: kit.IconFolderOpen, Action: func() { p.u.OpenFolder(dir) }})
		}
	}
	return append(items, kit.MenuItem{
		Key: "delete", Label: i18n.L("Delete from library…"), Icon: kit.IconTrash,
		Danger: true, Disabled: busy || g.Overall.Running, Action: p.askDelete,
	})
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
		var icon *ui.SVG
		switch {
		case pe.view.Status.Running:
			icon = kit.IconPlay
		case kind == kit.BadgeOK:
			icon = kit.IconCheck
		case kind == kit.BadgeWarn:
			icon = kit.IconAlert
		case kind == kit.BadgeErr:
			icon = kit.IconX
		}
		opts = append(opts, kit.ChipOption{Value: pe.id, Label: tagOf(pe), Icon: icon})
	}
	ui.Column(c).Gap(kit.S1).Children(func() {
		kit.Small(c, i18n.L("Saves of"))
		kit.Chips(c, p.u.Navigator(), "profile", opts, sel.id, func(id string) { p.vm.selected, p.vm.chosen = id, true })
	})
}
