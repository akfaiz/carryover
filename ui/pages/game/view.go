package game

import (
	"errors"

	"github.com/egoist/mygo/ui"

	"github.com/akfaiz/carryover/services"
	"github.com/akfaiz/carryover/ui/env"
	"github.com/akfaiz/carryover/ui/i18n"
	"github.com/akfaiz/carryover/ui/kit"
	"github.com/akfaiz/carryover/ui/store"
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
func (p *Page) DefaultFocus() *ui.Handle { return kit.BtnHandle(p.u.Navigator(), kit.PageBackKey) }

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

// btn is a button (icon may be nil) that registers with the gamepad. key
// tells it from the others on the page.
func (p *Page) btn(c *ui.Context, key any, kind kit.BtnKind, icon *ui.SVG, label string, disabled bool, action func()) ui.Element {
	return kit.Btn(c, p.u.Navigator(), [2]any{"game", key}, kind, icon, label, action, kit.BtnDisabled(disabled))
}

// iconBtn is a square button with only an icon; label names it.
func (p *Page) iconBtn(c *ui.Context, key any, kind kit.BtnKind, icon *ui.SVG, label string, disabled bool, action func()) ui.Element {
	return kit.IconBtn(c, p.u.Navigator(), [2]any{"game", key}, kind, icon, label, action, kit.BtnDisabled(disabled))
}

// View draws the page.
func (p *Page) View(c *ui.Context) {
	u, nav := p.u, p.u.Navigator()
	orig, ok := u.Store().Game(p.vm.id)
	// The game's hero artwork, else its wide one, behind the header.
	art := orig.Art[string(services.ArtHero)]
	if art.Path == "" {
		art = orig.Art[string(services.ArtWide)]
	}
	kit.ArtPage(c, kit.PageWide, &p.scroll, art.Path, art.Version, func() {
		if !ok {
			msg := i18n.L("Loading…")
			if u.Store().Loaded {
				msg = i18n.L("Game not found.")
			}
			kit.PageHead(c, nav, u.Back, i18n.L("Back"), "", nil)
			kit.Muted(c, msg)
			return
		}
		p.game(c, orig)
	})
}

func (p *Page) game(c *ui.Context, orig services.GameView) {
	u, nav := p.u, p.u.Navigator()
	g := orig
	g.Status = g.Overall // the game as a whole: its own saves and every emulator
	profs := p.vm.profiles(orig)
	busy := p.vm.busy != ""
	hasRemote := u.Store().Settings.Remote != ""

	// The header (over the page's artwork), with Back on its own row at the
	// top left, where other pages have it.
	kit.PageHead(c, nav, u.Back, i18n.L("Back"), "", nil)
	ui.Row(c).Gap(kit.S6).AlignItems(ui.Start).Children(func() {
		// A smaller cover than the library's leaves room for the saves on a
		// Steam Deck's 800 px high screen.
		kit.CoverView(c, kit.CoverSource{Path: g.CoverPath, Version: g.CoverVersion, Name: g.Name}, 140, 210).
			Shadow(0, 8, 24, 0, ui.RGBA(0, 0, 0, 0.35))
		ui.Column(c).Grow(1).Shrink(1).Gap(kit.S2).Children(func() { p.hero(c, orig, g, profs, busy, hasRemote) })
	})

	// The saves and backups of one profile; a game with emulators switches
	// between them above both columns, so the columns start level.
	sel := p.vm.selectedProfile(profs)
	if len(profs) > 1 {
		p.profileSwitcher(c, profs, sel)
	}
	ui.Row(c).Gap(kit.S4).Wrap().AlignItems(ui.Start).Children(func() {
		ui.Column(c).Grow(3).MinWidth(420).Gap(kit.S4).Children(func() {
			p.savesCard(c, sel, busy, hasRemote, len(profs) > 1, g)
			p.backupsCard(c, sel, busy, hasRemote)
		})
		ui.Column(c).Grow(2).MinWidth(300).Gap(kit.S4).Children(func() {
			p.steamCard(c, orig, busy)
			p.otherDevices(c, sel.view) // for the profile shown, as the Saves card
		})
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
		p.btn(c, "cancel", kit.BtnSecondary, nil, i18n.L("Cancelling…"), true, func() {})
		return
	}
	p.btn(c, "cancel", kit.BtnSecondary, nil, label, false, func() {
		if cancel() {
			p.vm.cancelling = true
		}
	})

}
