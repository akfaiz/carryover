package ui

import (
	"errors"
	"fmt"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/akfaiz/playrelay/services"
)

type libraryPage struct {
	u          *UI
	filter     string
	syncing    bool
	cancelling bool
	scroll     *container.Scroll
	search     *widget.Entry
	syncBtn    *widget.Button
	head       fyne.CanvasObject // header row, refreshed when syncBtn's label changes
	gridHost   *fyne.Container
	emptyHost  *fyne.Container
}

func newLibraryPage(u *UI) *libraryPage { return &libraryPage{u: u} }

func (p *libraryPage) view() fyne.CanvasObject {
	p.search = widget.NewEntry()
	p.search.SetPlaceHolder("Search…")
	p.search.OnChanged = func(s string) { p.filter = s; p.renderGrid() }
	p.syncBtn = widget.NewButtonWithIcon("Sync all", theme.ViewRefreshIcon(), p.syncAll)
	p.head = pageHead(nil, "Library",
		// Centered so the row (as tall as the heading) doesn't stretch the entry.
		container.NewCenter(fixedWidth(220, p.search)),
		p.syncBtn,
		widget.NewButton("Add from Steam", func() { showSteamImport(p.u) }),
		primary("＋ Add game", func() { p.u.navigate("/new") }),
	)
	p.gridHost = container.NewStack()
	p.emptyHost = container.NewStack()
	p.renderGrid()
	p.scroll = scrollPage(vbox(p.head, p.emptyHost, p.gridHost))
	return p.scroll
}

func (p *libraryPage) stateChanged(what change) {
	if what == changeProgress {
		return
	}
	p.renderGrid()
}

func (p *libraryPage) renderGrid() {
	u := p.u
	// While syncing, the same button cancels the run.
	switch {
	case p.cancelling:
		p.syncBtn.SetText("Cancelling…")
		p.syncBtn.SetIcon(theme.CancelIcon())
		p.syncBtn.Disable()
	case p.syncing:
		p.syncBtn.SetText("Cancel sync")
		p.syncBtn.SetIcon(theme.CancelIcon())
		p.syncBtn.Enable()
	default:
		p.syncBtn.SetText("Sync all")
		p.syncBtn.SetIcon(theme.ViewRefreshIcon())
		setEnabled(p.syncBtn, u.settings.Remote != "")
	}
	if p.head != nil {
		p.head.Refresh() // the button's label width changed
	}

	p.emptyHost.Objects = nil
	if u.loaded && len(u.games) == 0 {
		actions := hbox(primary("＋ Add your first game", func() { u.navigate("/new") }))
		if u.settings.Remote == "" {
			actions.Add(widget.NewButtonWithIcon("Set up cloud storage", theme.StorageIcon(), func() { u.navigate("/cloud") }))
		}
		p.emptyHost.Objects = []fyne.CanvasObject{card("No games yet",
			muted("Add a game, tell PlayRelay where its saves live on this device, and it will keep them in sync "+
				"through your cloud storage every time you play — from here or from Steam."),
			actions)}
	}
	p.emptyHost.Refresh()

	grid := container.NewGridWrap(fyne.NewSize(170, 330))
	f := strings.ToLower(p.filter)
	for _, g := range u.games {
		if !strings.Contains(strings.ToLower(g.Name), f) {
			continue
		}
		grid.Add(newGameTile(p.u, g))
	}
	p.gridHost.Objects = []fyne.CanvasObject{grid}
	p.gridHost.Refresh()
}

func (p *libraryPage) syncAll() {
	u := p.u
	if p.syncing {
		if u.sync.Cancel("") {
			p.cancelling = true
			p.renderGrid()
		}
		return
	}
	p.syncing = true
	p.renderGrid()
	async(u.sync.SyncAll, func(res []services.SyncResult, err error) {
		p.syncing, p.cancelling = false, false
		defer u.refresh()
		if errors.Is(err, services.ErrSyncCancelled) {
			u.toast(fmt.Sprintf("Sync cancelled after %d game(s)", len(res)), toastInfo)
			p.renderGrid()
			return
		}
		if err != nil {
			u.toastErr(err)
			p.renderGrid()
			return
		}
		var failed, conflicts []services.SyncResult
		for _, r := range res {
			if r.Error != "" {
				failed = append(failed, r)
			}
			if r.Action == "conflict" {
				conflicts = append(conflicts, r)
			}
		}
		switch {
		case len(failed) > 0:
			u.toast(fmt.Sprintf("%d game(s) failed to sync: %s: %s", len(failed), failed[0].Name, failed[0].Error), toastErr)
		case len(conflicts) > 0:
			u.toast(fmt.Sprintf("%d game(s) have save conflicts", len(conflicts)), toastErr)
		default:
			u.toast(fmt.Sprintf("Synced %d game(s)", len(res)), toastOK)
		}
		p.renderGrid()
	})
}
