package game

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/akfaiz/carryover/ui/textutil"

	"github.com/akfaiz/carryover/ui/i18n"

	"github.com/akfaiz/carryover/services"
	"github.com/akfaiz/carryover/ui/components"
	"github.com/akfaiz/carryover/ui/env"
	"github.com/akfaiz/carryover/ui/flows"
	"github.com/akfaiz/carryover/ui/kit"
	"github.com/akfaiz/carryover/ui/store"
)

// Page is a game: its saves and how they compare with the cloud, backups,
// the Steam shortcut and the play session.
type Page struct {
	u  env.Env
	vm *ViewModel

	wrapSection *kit.AccordionSection // to keep its open state across renders
	scroll      *container.Scroll
	prog        *kit.ProgressBar // current progress widget, updated in place
	progOf      string           // "session" or "busy"
	// backBtn is where a gamepad starts: A there only goes back, so an A
	// pressed once too often (the one that opened the game) never starts it.
	backBtn *widget.Button
}

var (
	_ env.Syncable   = (*Page)(nil)
	_ env.Stateful   = (*Page)(nil)
	_ env.PadFocuser = (*Page)(nil)
	_ env.Redrawable = (*Page)(nil)
)

// GameID is the game the page shows.
func (p *Page) GameID() string { return p.vm.id }

// MoreMenu is the menu behind the page's "⋯" button.
func (p *Page) MoreMenu(g services.GameView, busy bool) *fyne.Menu { return p.moreMenu(g, busy) }

// SelectedProfile is the id of the profile whose saves are shown ("" is the
// game's own saves), for the game as the library lists it.
func (p *Page) SelectedProfile(g services.GameView) string {
	return p.vm.selectedProfile(p.vm.profiles(g)).id
}

func (p *Page) DefaultFocus() fyne.Focusable {
	if p.backBtn == nil {
		return nil
	}
	return p.backBtn
}

// New returns the page of the game with that id.
func New(u env.Env, id string) *Page {
	return &Page{u: u, vm: newViewModel(id, serviceBackend{u})}
}

func (p *Page) View() fyne.CanvasObject {
	p.scroll = kit.ScrollPage(widget.NewLabel(""))
	if g, ok := p.u.Store().Game(p.vm.id); ok {
		p.vm.initFrom(g)
	}
	kit.Async(func() (services.SteamInfo, error) { return p.vm.be.SteamInfo(), nil }, func(info services.SteamInfo, _ error) {
		p.vm.steam = &info
		if len(info.Users) > 0 {
			p.vm.steamUser = info.Users[0].ID
		}
		p.render()
	})
	kit.Async(func() (string, error) { return p.vm.be.WrapLaunchOptions(p.vm.id) }, func(s string, err error) {
		if err == nil {
			p.vm.wrapOpts = s
			p.render()
		}
	})
	p.render()
	return p.scroll
}

// syncNow is F5: sync every profile that can be synced here.
func (p *Page) SyncNow() {
	g, ok := p.u.Store().Game(p.vm.id)
	if !ok || p.vm.busy != "" || p.u.Store().Settings.Remote == "" || g.Overall.Running {
		return
	}
	p.syncAll(p.vm.profiles(g))
}

func (p *Page) StateChanged(what store.Topic) {
	if what == store.Status && p.u.Store().Status[p.vm.id].Phase != "sync" && p.vm.busy == "" {
		p.vm.cancelling = false // the session's sync phase ended
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
	if st, ok := p.u.Store().Status[p.vm.id]; ok && st.Key != "" {
		return st.Key
	}
	return p.vm.id
}

func (p *Page) render() {
	u := p.u
	p.prog, p.progOf = nil, ""
	if p.wrapSection != nil {
		p.vm.wrapOpen = p.wrapSection.Open()
	}
	orig, ok := u.Store().Game(p.vm.id)
	if !ok {
		msg := i18n.L("Loading…")
		if u.Store().Loaded {
			msg = i18n.L("Game not found.")
		}
		kit.ReplaceScroll(p.scroll, kit.Stack(kit.PageHead(u.Back, ""), kit.Muted(msg)))
		return
	}
	g := orig
	g.Status = g.Overall // the game as a whole: its own saves and every emulator
	profs := p.vm.profiles(orig)
	busy := p.vm.busy != ""
	hasRemote := u.Store().Settings.Remote != ""
	canSync := canSync(profs)

	// Hero.
	p.backBtn = widget.NewButtonWithIcon(i18n.L("Back"), theme.NavigateBackIcon(), u.Back)
	p.backBtn.Importance = widget.LowImportance
	info := kit.VBox(
		kit.WrapTitle(g.Name),
		kit.HBox(components.StatusBadge(g), kit.Small(i18n.L("Last sync: {{.When}}", map[string]any{"When": components.TimeAgo(g.Status.LastSyncAt)}))),
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
		info.Add(kit.Colored(i18n.L("The saves on this device changed after the last sync, so the game was probably played without Carryover. Use “Sync now” to upload them."), widget.WarningImportance))
	}
	actions := kit.HBox()
	steamLaunched := g.Device.LaunchMode == "steamwrap"
	switch {
	case g.Status.Running:
		b := kit.Primary(i18n.L("● Running"), nil)
		b.Disable()
		actions.Add(b)
	case g.Ready && steamLaunched:
		b := kit.Primary(i18n.L("Play in Steam"), func() { flows.OpenSteam(u, orig) })
		kit.SetEnabled(b, services.SteamRunURL(orig) != "")
		actions.Add(b)
	case g.Ready:
		// A game that can be played in several ways asks which one first.
		b := widget.NewButtonWithIcon(i18n.L("Play"), theme.MediaPlayIcon(), func() { p.run("play", func() error { return p.vm.be.Play(p.vm.id) }, nil) })
		b.Importance = widget.HighImportance
		kit.SetEnabled(b, !busy)
		actions.Add(b)
	default:
		actions.Add(kit.Primary(i18n.L("Set up for this device"), func() { u.Navigate("/edit/" + p.vm.id) }))
	}
	syncLabel := i18n.L("Sync now")
	if p.vm.busy == "sync" {
		syncLabel = i18n.L("Syncing…")
	}
	syncBtn := widget.NewButtonWithIcon(syncLabel, theme.ViewRefreshIcon(), func() { p.syncAll(profs) })
	kit.SetEnabled(syncBtn, !busy && canSync && hasRemote && !g.Status.Running)
	actions.Add(syncBtn)
	actions.Add(widget.NewButtonWithIcon(i18n.L("Edit"), theme.DocumentCreateIcon(), func() { u.Navigate("/edit/" + p.vm.id) }))
	actions.Add(p.moreButton(orig, busy))
	info.Add(actions)
	if steamLaunched && g.Ready && services.SteamRunURL(orig) == "" {
		info.Add(kit.Muted(i18n.L("This game is launched from Steam on this device (Proton). Use “Add to Steam” below, then start it from Steam.")))
	}
	if p.vm.busy == "delete" {
		info.Add(kit.FixedWidth(480, kit.BusyLine(i18n.L("Deleting…"))))
	}

	sessKey := p.sessionKey()
	st, hasSt := u.Store().Status[p.vm.id]
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
			label := i18n.L("Cancel upload")
			if st.BeforePlay {
				label = i18n.L("Cancel and don't play")
			}
			box.Add(kit.HBox(p.cancelButton(label, func() bool { return p.vm.be.CancelPlay(sessKey) })))
		}
		info.Add(kit.FixedWidth(480, kit.Card("", box)))
	} else if p.vm.busy == "sync" {
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
	sel := p.vm.selectedProfile(profs)
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
	return kit.VBox(kit.Small(i18n.L("Saves of")), kit.Chips(opts, sel.id, func(id string) {
		p.vm.selected, p.vm.chosen = id, true
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
			label := i18n.L("Open save folder")
			if len(g.Slots) > 1 {
				label = i18n.L("Open save folder: {{.Label}}", map[string]any{"Label": textutil.LabelOf(sl.Label, sl.Key)})
			}
			items = append(items, fyne.NewMenuItem(label, func() { p.u.OpenFolder(dir) }))
		}
	}
	if len(items) > 0 {
		items = append(items, fyne.NewMenuItemSeparator())
	}
	del := fyne.NewMenuItem(i18n.L("Delete from library…"), p.askDelete)
	del.Icon = theme.DeleteIcon()
	del.Disabled = busy || g.Overall.Running
	return fyne.NewMenu("", append(items, del)...)
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

// Redraw draws the page again from its state (env.Redrawable).
func (p *Page) Redraw() { p.render() }

// tagOf names a profile on the switcher: the emulator, or the game's own saves.
func tagOf(pe profileEntry) string {
	if pe.noEmu {
		return i18n.L("Without an emulator")
	}
	return pe.tag
}
