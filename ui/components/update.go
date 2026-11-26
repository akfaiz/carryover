package components

import (
	"context"
	"errors"

	"github.com/egoist/mygo/ui"

	"github.com/akfaiz/carryover/services"
	"github.com/akfaiz/carryover/ui/env"
	"github.com/akfaiz/carryover/ui/i18n"
	"github.com/akfaiz/carryover/ui/kit"
)

// updateNotesHeight is how much of the release notes shows before they scroll.
const updateNotesHeight float32 = 240

// ShowUpdate opens the dialog about the newer version of Carryover: what is in
// it, and the button that downloads it, replaces this one and restarts.
func ShowUpdate(u env.Env) {
	d := &updateDialog{u: u}
	d.dialog = u.Dialogs().Show(&kit.Dialog{Title: i18n.L("Update Carryover"), Width: 560, Back: d.close, Body: d.body})
}

type updateDialog struct {
	u      env.Env
	dialog *kit.Dialog
	// closed: the user took the dialog down; a download goes on, but nothing
	// is installed and Carryover doesn't restart behind their back.
	closed   bool
	starting bool // between the button and the end of the install
}

func (d *updateDialog) close() {
	d.closed = true
	d.dialog.Close()
}

// start downloads the update (unless it already is) and then installs it.
func (d *updateDialog) start() {
	if d.starting {
		return
	}
	d.starting = true
	upd := d.u.Updates()
	kit.Async(func() (string, error) { return upd.Download(context.Background()) }, func(_ string, err error) {
		switch {
		case err != nil:
			d.failed(err)
		case d.closed:
			d.u.Toast(i18n.L("The update is downloaded. Open it again to install it."), kit.ToastInfo)
		default:
			d.install()
		}
	})
}

func (d *updateDialog) install() {
	// A game or a sync in this window; Install checks for other processes.
	if d.u.Busy() {
		d.failed(services.ErrUpdateBusy)
		return
	}
	upd := d.u.Updates()
	kit.Async(func() (func(bool) error, error) { return upd.Install(context.Background()) }, func(finish func(bool) error, err error) {
		if err != nil {
			d.failed(err)
			return
		}
		d.u.RestartForUpdate(finish)
	})
}

// failed ends an attempt. Errors of the download and the install show in the
// dialog (the state carries them); the ones that don't are toasted.
func (d *updateDialog) failed(err error) {
	d.starting = false
	switch {
	case errors.Is(err, services.ErrUpdateCancelled), errors.Is(err, services.ErrUpdateRunning):
	case errors.Is(err, services.ErrUpdateBusy):
		d.u.Toast(i18n.L("A game is being played or a sync is running. Install the update once it is done."), kit.ToastInfo)
	case d.closed || d.u.Updates().State().Error == "":
		d.u.ToastErr(err)
	}
}

func (d *updateDialog) body(c *ui.Context, nav *kit.Navigator) {
	st := d.u.Store().Update
	if st.Latest == "" {
		kit.Muted(c, i18n.L("Carryover is up to date."))
		kit.Footer(c, func() {
			kit.Btn(c, nav, "update-later", kit.BtnSecondary, nil, i18n.L("Close"), d.close, kit.BtnItem(kit.FocusSafe))
		})
		return
	}
	ui.Row(c).Gap(kit.S2).AlignItems(ui.Center).Wrap().Children(func() {
		ui.Text(c, i18n.L("Version {{.Latest}} is available. You have {{.Current}}.",
			map[string]any{"Latest": st.Latest, "Current": st.Current})).Wrap().Grow(1).Shrink(1)
		if st.Prerelease {
			kit.Badge(c, i18n.L("Pre-release"), kit.BadgeWarn)
		}
	})
	if st.Notes != "" {
		kit.ScrollBox(c, updateNotesHeight, kit.S2, func() { ui.Text(c, st.Notes).Wrap().FontSize(kit.TextSM) })
	}
	if st.URL != "" {
		open := func() { d.u.ToastErr(d.u.OpenURL(st.URL)) }
		kit.Btn(c, nav, "update-page", kit.BtnGhost, nil, i18n.L("Open release page"), open, kit.BtnSmall()).AlignSelf(ui.Start)
	}
	if st.Error != "" {
		kit.Callout(c, kit.BadgeErr, kit.IconAlert, st.Error, nil)
	}
	switch {
	case st.Status == services.UpdateDownloading:
		kit.ProgressLine(c, i18n.L("Downloading…"), st.Done, st.Total)
		kit.Footer(c, func() {
			kit.Btn(c, nav, "update-cancel", kit.BtnSecondary, nil, i18n.L("Cancel"), func() { d.u.Updates().Cancel() }, kit.BtnItem(kit.FocusSafe))
		})
	case st.Status == services.UpdateInstalled:
		kit.Muted(c, i18n.L("Restart Carryover to finish updating."))
		kit.Footer(c, func() {
			kit.Btn(c, nav, "update-later", kit.BtnSecondary, nil, i18n.L("Close"), d.close, kit.BtnItem(kit.FocusSafe))
		})
	case !st.Installable:
		kit.Muted(c, i18n.L("This install can't update itself. Download the new version from the release page."))
		kit.Footer(c, func() {
			kit.Btn(c, nav, "update-later", kit.BtnSecondary, nil, i18n.L("Close"), d.close, kit.BtnItem(kit.FocusSafe))
		})
	default:
		label := i18n.L("Download and restart")
		if st.Status == services.UpdateReady {
			label = i18n.L("Install and restart")
		}
		kit.Footer(c, func() {
			kit.Btn(c, nav, "update-later", kit.BtnSecondary, nil, i18n.L("Later"), d.close, kit.BtnItem(kit.FocusSafe))
			kit.Btn(c, nav, "update-install", kit.BtnPrimary, kit.IconDownload, label, d.start, kit.BtnDisabled(d.starting))
		})
	}
}
