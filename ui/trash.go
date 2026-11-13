package ui

import (
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/lang"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/akfaiz/carryover/services"
)

// trashPage lists what was deleted: games and emulators that can still be
// restored, and cloud save folders no game uses any more.
type trashPage struct {
	u *UI

	games  []services.TrashGame
	emus   []services.TrashEmulator
	left   []services.LeftoverView
	loaded bool
	// leftLoaded: the cloud was asked for leftover folders (it is slower than
	// the rest, so it loads after the page is shown).
	leftLoaded  bool
	leftLoading bool
	leftErr     string

	confirm     string // "g:<id>", "e:<id>" or "l:<key>": the row whose deletion awaits confirmation
	deleteSaves bool   // that deletion also deletes the cloud saves
	busy        bool

	scroll *container.Scroll
}

func newTrashPage(u *UI) *trashPage { return &trashPage{u: u} }

func (p *trashPage) view() fyne.CanvasObject {
	p.scroll = scrollPage(muted(lang.L("Loading…")))
	p.reload()
	p.loadLeftovers()
	return p.scroll
}

func (p *trashPage) stateChanged(what change) {
	if what == changeLibrary && !p.busy {
		p.reload()
		if !p.leftLoaded {
			p.loadLeftovers() // the cloud may have been set up since the page opened
		}
	}
}

func (p *trashPage) reload() {
	type loaded struct {
		games []services.TrashGame
		emus  []services.TrashEmulator
	}
	async(func() (loaded, error) { return loaded{p.u.lib.Trash(), p.u.emus.Trash()}, nil }, func(l loaded, _ error) {
		p.games, p.emus, p.loaded = l.games, l.emus, true
		p.u.trashCount = len(l.games) + len(l.emus)
		p.u.refreshSidebar()
		p.render()
	})
}

func (p *trashPage) loadLeftovers() {
	if p.u.settings.Remote == "" || p.leftLoading {
		return
	}
	p.leftLoading = true
	async(p.u.lib.Leftovers, func(l []services.LeftoverView, err error) {
		p.left, p.leftLoaded, p.leftLoading, p.leftErr = l, true, false, ""
		if err != nil {
			p.leftErr = err.Error()
		} else {
			p.u.leftoverCount = len(l)
			p.u.refreshSidebar()
		}
		p.render()
	})
}

// run does a deletion or restore off the UI thread and then reloads.
func (p *trashPage) run(work func() error, done func()) {
	p.busy = true
	async0(work, func(err error) {
		p.busy, p.confirm, p.deleteSaves = false, "", false
		p.u.toastErr(err)
		if err == nil && done != nil {
			done()
		}
		p.reload()
		p.loadLeftovers()
	})
}

func (p *trashPage) render() {
	if !p.loaded {
		return
	}
	body := stack(pageHead(nil, lang.L("Trash")),
		muted(lang.L("Deleted games and emulators stay here for {{.Days}} days and can be restored with all their settings. Their cloud saves are not touched unless you delete them too.",
			map[string]any{"Days": services.TrashDays})))

	games := stack()
	for _, g := range p.games {
		key := "g:" + g.ID
		games.Add(p.row(key, g.Name, g.DeletedAt, g.ExpiresAt,
			func() {
				p.run(func() error { return p.u.lib.Restore(g.ID) }, func() { p.u.toast(lang.L("Restored"), toastOK) })
			},
			lang.L("Also delete its cloud saves and backups"),
			func(saves bool) error {
				if saves {
					return p.u.lib.DeleteWithSaves(g.ID)
				}
				return p.u.lib.DeleteForever(g.ID)
			}))
	}
	body.Add(p.section(lang.L("Games"), lang.L("No deleted games."), games, len(p.games)))

	emus := stack()
	for _, e := range p.emus {
		key := "e:" + e.ID
		emus.Add(p.row(key, e.Name, e.DeletedAt, e.ExpiresAt,
			func() {
				p.run(func() error { return p.u.emus.Restore(e.ID) }, func() { p.u.toast(lang.L("Restored"), toastOK) })
			},
			lang.L("Also delete the cloud saves made with it"),
			func(saves bool) error {
				if saves {
					return p.u.emus.DeleteWithSaves(e.ID)
				}
				return p.u.emus.DeleteForever(e.ID)
			}))
	}
	body.Add(p.section(lang.L("Emulators"), lang.L("No deleted emulators."), emus, len(p.emus)))
	body.Add(p.leftovers())
	replaceScroll(p.scroll, body)
}

// section is a card with a heading and either its rows or an empty message.
func (p *trashPage) section(title, empty string, rows *fyne.Container, n int) fyne.CanvasObject {
	if n == 0 {
		return card(title, muted(empty))
	}
	return card(title, rows)
}

// row is a restorable item: Restore, and Delete permanently with an optional
// deletion of its cloud saves.
func (p *trashPage) row(key, name string, deleted, expires time.Time, restore func(), checkLabel string, del func(saves bool) error) fyne.CanvasObject {
	sub := small(lang.L("Deleted {{.When}} · settings kept until {{.Until}}", map[string]any{"When": fmtDate(deleted), "Until": fmtDate(expires)}))
	if p.confirm != key {
		restoreBtn := widget.NewButtonWithIcon(lang.L("Restore"), theme.HistoryIcon(), restore)
		deleteBtn := dangerButton(lang.L("Delete permanently…"), func() { p.confirm, p.deleteSaves = key, false; p.render() })
		return vbox(cardHead(name, restoreBtn, deleteBtn), sub)
	}
	msg := lang.L("Delete {{.Name}} permanently? It can't be restored any more. Its cloud saves and backups are kept.", map[string]any{"Name": name})
	if p.deleteSaves {
		msg = lang.L("Delete {{.Name}} permanently together with its cloud saves and backups? This cannot be undone.", map[string]any{"Name": name})
	}
	saves := rebuildingCheck(checkLabel, p.deleteSaves, func(on bool) { p.deleteSaves = on; p.render() })
	var hint fyne.CanvasObject = muted("")
	if !p.deleteSaves {
		hint = muted(lang.L("The cloud saves stay and are listed under Leftover cloud saves below until you delete them."))
	}
	return vbox(cardHead(name), colored(msg, widget.MediumImportance), saves, hint,
		hbox(widget.NewButton(lang.L("Cancel"), func() { p.confirm, p.deleteSaves = "", false; p.render() }),
			dangerButton(lang.L("Delete"), func() {
				saves := p.deleteSaves
				p.run(func() error { return del(saves) }, nil)
			})))
}

// leftovers is the card of cloud save folders no game uses.
func (p *trashPage) leftovers() fyne.CanvasObject {
	title := lang.L("Leftover cloud saves")
	intro := muted(lang.L("Saves and backups in your cloud that no game in the library uses, for example after a game was deleted for good or an emulator was removed from it. Restore one to add it back to the library, or delete it to free the space."))
	switch {
	case p.u.settings.Remote == "":
		return card(title, intro, muted(lang.L("Set up cloud storage to look for them.")))
	case !p.leftLoaded:
		return card(title, intro, muted(lang.L("Looking in the cloud…")))
	case p.leftErr != "":
		return card(title, intro, colored(p.leftErr, widget.DangerImportance))
	case len(p.left) == 0:
		return card(title, intro, muted(lang.L("Nothing left over.")))
	}
	rows := stack(intro)
	for _, l := range p.left {
		rows.Add(p.leftoverRow(l))
	}
	return card(title, rows)
}

func (p *trashPage) leftoverRow(l services.LeftoverView) fyne.CanvasObject {
	name := l.GameName
	if name == "" {
		name = lang.L("Unknown game")
	}
	if l.EmulatorID != "" {
		emu := l.EmulatorName
		if emu == "" {
			emu = lang.L("Unknown emulator")
		}
		name += " · " + emu
	}
	sub := small(lang.L("{{.Count}} files, {{.Size}} · last synced {{.When}} · {{.Backups}} backups",
		map[string]any{"Count": l.Files, "Size": formatBytes(l.Bytes), "When": fmtDate(l.Time), "Backups": l.Backups}))
	key := "l:" + l.Key
	if p.confirm != key {
		restoreBtn := widget.NewButtonWithIcon(lang.L("Restore"), theme.HistoryIcon(), func() {
			p.busy = true
			async(func() (string, error) { return p.u.lib.RestoreLeftover(l.Key) }, func(id string, err error) {
				p.busy = false
				if err != nil {
					p.u.toastErr(err)
					return
				}
				p.u.toast(lang.L("Restored. Set where its saves are on this device, then sync."), toastOK)
				p.u.navigate("/edit/" + id)
			})
		})
		deleteBtn := dangerButton(lang.L("Delete…"), func() { p.confirm = key; p.render() })
		return vbox(cardHead(name, restoreBtn, deleteBtn), sub)
	}
	return vbox(cardHead(name), sub,
		colored(lang.L("Permanently delete these cloud saves and backups? This cannot be undone."), widget.MediumImportance),
		hbox(widget.NewButton(lang.L("Cancel"), func() { p.confirm = ""; p.render() }),
			dangerButton(lang.L("Delete"), func() { p.run(func() error { return p.u.lib.DeleteLeftover(l.Key) }, nil) })))
}
