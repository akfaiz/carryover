package game

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/akfaiz/carryover/ui/i18n"

	"github.com/akfaiz/carryover/internal/cloudsync/syncer"
	"github.com/akfaiz/carryover/ui/components"
	"github.com/akfaiz/carryover/ui/kit"
)

// backupsCard lists and takes the backups of one profile's cloud save. The
// list loads by itself the first time the card is shown.
func (p *Page) backupsCard(pe profileEntry, busy, hasRemote bool) fyne.CanvasObject {
	g := pe.view
	st := p.vm.state(pe.id)
	if hasRemote && !st.backupsLoaded && !st.backupsLoading {
		p.loadBackups(pe.id)
	}
	refresh := widget.NewButtonWithIcon("", theme.ViewRefreshIcon(), func() { p.loadBackups(pe.id) })
	kit.SetEnabled(refresh, hasRemote && !st.backupsLoading)
	nowLabel := i18n.L("Back up now")
	if p.vm.busyFor("backup", pe.id) {
		nowLabel = i18n.L("Backing up…")
	}
	nowBtn := widget.NewButton(nowLabel, func() { p.backupNow(pe.id) })
	kit.SetEnabled(nowBtn, !busy && hasRemote && g.Ready && !g.Status.Running)

	summary := i18n.L("Cloud backups are off. A copy of this device's saves is kept here before a download replaces them.")
	details := []string{i18n.L("Cloud backups are off. Before a download replaces this device's saves, a copy is kept on this device (the last {{.Count}}). Turn on cloud backups in Settings to keep backups in the cloud.",
		map[string]any{"Count": syncer.SafetyKeep})}
	if n := p.u.Store().Settings.BackupRetention; n > 0 {
		summary = i18n.N("Cloud backups are on: the last {{.Count}} backup is kept.", n, map[string]any{"Count": n})
		details = []string{i18n.N("A backup is taken in the cloud whenever saves are about to be overwritten (keeping the last {{.Count}} backup).", n,
			map[string]any{"Count": n})}
	}
	details = append(details, i18n.L("Backups you make with “Back up now” are pinned. Pinned backups are never deleted automatically."))
	c := kit.VBox(kit.CardHead(i18n.L("Backups"), nowBtn, refresh), components.Help(p.u, summary, details...))
	if g.SafetyDir != "" {
		dir := g.SafetyDir
		c.Add(kit.HBox(widget.NewButtonWithIcon(i18n.L("Open copies on this device"), theme.FolderOpenIcon(), func() { p.u.OpenFolder(dir) })))
	}
	if (p.vm.busy == "restore" || p.vm.busy == "backup") && p.vm.activeProfile() == pe.id {
		c.Add(p.transferCard())
	}
	if p.vm.busyFor("pin", pe.id) {
		c.Add(kit.BusyLine(i18n.L("Updating backups…")))
	}
	switch {
	case !hasRemote:
		c.Add(kit.Muted(i18n.L("Set up cloud storage to keep backups in the cloud.")))
	case st.backupsLoading && !st.backupsLoaded:
		c.Add(kit.BusyLine(i18n.L("Loading backups…")))
	case st.backupsErr != "":
		c.Add(kit.Colored(st.backupsErr, widget.DangerImportance))
	case len(st.backups) == 0:
		c.Add(kit.Muted(i18n.L("No backups yet.")))
	}
	for _, b := range st.backups {
		c.Add(backupRow(b, p.vm.busyFor("restore", pe.id), !busy && g.Ready, !busy,
			func() { p.restore(pe.id, b) }, func() { p.setPinned(pe.id, b) }, func() { p.deleteBackup(pe.id, b) }))
	}
	return kit.Card("", c)
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
			p.render()
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
