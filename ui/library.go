package ui

import (
	"errors"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/lang"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/akfaiz/carryover/services"
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
	barHost    *fyne.Container // "Sync all" progress, shown only while syncing
	bar        *widget.ProgressBar
	barLabel   *widget.Label
}

func newLibraryPage(u *UI) *libraryPage { return &libraryPage{u: u} }

func (p *libraryPage) view() fyne.CanvasObject {
	p.search = widget.NewEntry()
	p.search.SetPlaceHolder(lang.L("Search…"))
	p.search.OnChanged = func(s string) { p.filter = s; p.renderGrid() }
	p.syncBtn = widget.NewButtonWithIcon(lang.L("Sync all"), theme.ViewRefreshIcon(), p.syncAll)
	p.head = pageHead(nil, lang.L("Library"),
		// Centered so the row (as tall as the heading) doesn't stretch the entry.
		container.NewCenter(fixedWidth(220, p.search)),
		p.syncBtn,
		widget.NewButton(lang.L("Add from Steam"), func() { showSteamImport(p.u) }),
		primary(lang.L("＋ Add game"), func() { p.u.navigate("/new") }),
	)
	p.gridHost = container.NewStack()
	p.emptyHost = container.NewStack()
	p.renderGrid()
	p.bar = widget.NewProgressBar()
	p.barLabel = small("")
	p.barHost = container.NewVBox(p.barLabel, p.bar)
	p.barHost.Hide()
	p.scroll = scrollPage(vbox(p.head, p.barHost, p.emptyHost, p.gridHost))
	return p.scroll
}

// updateProgress shows how many games "Sync all" has finished.
func (p *libraryPage) updateProgress() {
	if p.barHost == nil {
		return
	}
	pr := p.u.syncAllProg
	if !p.syncing || pr.Total == 0 {
		p.barHost.Hide()
		return
	}
	p.bar.Max = float64(pr.Total)
	p.bar.SetValue(float64(pr.Done))
	text := lang.L("Synced {{.Done}} of {{.Total}}", map[string]any{"Done": pr.Done, "Total": pr.Total})
	if pr.Name != "" {
		text += " · " + pr.Name
	}
	p.barLabel.SetText(text)
	p.barHost.Show()
}

func (p *libraryPage) stateChanged(what change) {
	if what == changeProgress {
		p.updateProgress()
		return
	}
	p.renderGrid()
}

func (p *libraryPage) renderGrid() {
	u := p.u
	// While syncing, the same button cancels the run.
	switch {
	case p.cancelling:
		p.syncBtn.SetText(lang.L("Cancelling…"))
		p.syncBtn.SetIcon(theme.CancelIcon())
		p.syncBtn.Disable()
	case p.syncing:
		p.syncBtn.SetText(lang.L("Cancel sync"))
		p.syncBtn.SetIcon(theme.CancelIcon())
		p.syncBtn.Enable()
	default:
		p.syncBtn.SetText(lang.L("Sync all"))
		p.syncBtn.SetIcon(theme.ViewRefreshIcon())
		setEnabled(p.syncBtn, u.settings.Remote != "")
	}
	if p.head != nil {
		p.head.Refresh() // the button's label width changed
	}

	p.emptyHost.Objects = nil
	if u.loaded && len(u.games) == 0 {
		actions := hbox(primary(lang.L("＋ Add your first game"), func() { u.navigate("/new") }))
		if u.settings.Remote == "" {
			actions.Add(widget.NewButtonWithIcon(lang.L("Set up cloud storage"), theme.StorageIcon(), func() { u.navigate("/cloud") }))
		}
		p.emptyHost.Objects = []fyne.CanvasObject{card(lang.L("No games yet"),
			muted(lang.L("Add a game, tell Carryover where its saves live on this device, and it will keep them in sync through your cloud storage every time you play — from here or from Steam.")),
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
		p.updateProgress()
		defer u.refresh()
		if errors.Is(err, services.ErrSyncCancelled) {
			u.toast(lang.N("Sync cancelled after {{.Count}} game", len(res), map[string]any{"Count": len(res)}), toastInfo)
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
			u.toast(lang.N("{{.Count}} game failed to sync: {{.Name}}: {{.Error}}", len(failed),
				map[string]any{"Count": len(failed), "Name": failed[0].Name, "Error": failed[0].Error}), toastErr)
		case len(conflicts) > 0:
			u.toast(lang.N("{{.Count}} game has save conflicts", len(conflicts), map[string]any{"Count": len(conflicts)}), toastErr)
		default:
			u.toast(lang.N("Synced {{.Count}} game", len(res), map[string]any{"Count": len(res)}), toastOK)
		}
		p.renderGrid()
	})
}
