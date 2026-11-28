package game

import (
	"github.com/egoist/mygo/ui"

	"github.com/akfaiz/carryover/internal/cloudsync/syncer"
	"github.com/akfaiz/carryover/ui/components"
	"github.com/akfaiz/carryover/ui/i18n"
	"github.com/akfaiz/carryover/ui/kit"
)

// backupRow is one backup with Pin, Delete and Restore buttons (no Pin for a
// safety copy kept on this device).
func (p *Page) backupRow(c *ui.Context, first bool, prof string, b syncer.Backup, restoring, canRestore, canPin bool) {
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
	kit.ListRow(c, first, components.FmtDate(b.Time), func() { kit.Muted(c, desc) }, func() {
		if !b.Local {
			p.btn(c, "pin:"+key, kit.BtnSecondary, kit.IconPin, pinLabel, !canPin, func() { p.setPinned(prof, b) })
		}
		p.iconBtn(c, "del:"+key, kit.BtnDanger, kit.IconTrash, i18n.L("Delete"), !canPin, func() { p.deleteBackup(prof, b) })
		p.btn(c, "restore:"+key, kit.BtnSecondary, kit.IconRestore, label, !canRestore, func() { p.restore(prof, b) })
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
			if g.Ready { // there are no saves here to back up until the game is set up
				p.btn(c, "backup-now:"+pe.id, kit.BtnSecondary, kit.IconUpload, label, busy || !hasRemote || g.Status.Running, func() { p.backupNow(pe.id) })
			}
			p.iconBtn(c, "backups-refresh:"+pe.id, kit.BtnSecondary, kit.IconRefresh, i18n.L("Refresh"), !hasRemote || st.backupsLoading, func() { p.loadBackups(pe.id) })
		})
		components.Help(c, p.u, summary, details...)
		if g.SafetyDir != "" {
			dir := g.SafetyDir
			p.btn(c, "safety:"+pe.id, kit.BtnSecondary, kit.IconFolderOpen, i18n.L("Open copies on this device"), false, func() { p.u.OpenFolder(dir) }).AlignSelf(ui.Start)
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
		case !g.Ready:
			kit.Small(c, i18n.L("Set up this game on this device to restore a backup."))
		}
		for i, b := range st.backups {
			p.backupRow(c, i == 0, pe.id, b, p.vm.busyFor("restore", pe.id), !busy && g.Ready, !busy)
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
				st.backupsErr = i18n.Err(err)
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
