package settings

import (
	"context"

	"github.com/egoist/mygo/ui"

	"github.com/akfaiz/carryover/services"
	"github.com/akfaiz/carryover/ui/i18n"
	"github.com/akfaiz/carryover/ui/kit"
)

// checkUpdates looks for a newer version; force reads the releases again even
// if that was just done. What it finds, or why it failed, shows on the card.
func (p *Page) checkUpdates(force bool) {
	upd := p.u.Updates()
	kit.Async0(func() error {
		_, err := upd.Check(context.Background(), force)
		return err
	}, func(error) {})
}

// updatesCard chooses which releases updates come from, and shows what there
// is to install.
func (p *Page) updatesCard(c *ui.Context) {
	nav, f, st := p.u.Navigator(), &p.f, p.u.Store().Update
	kit.Card(c, i18n.L("Updates"), func() {
		kit.Labeled(c, i18n.L("Update channel"), func() {
			kit.Chips(c, nav, "channel", []kit.ChipOption{
				{Value: "", Label: i18n.L("Stable")},
				{Value: "prerelease", Label: i18n.L("Pre-release")},
			}, f.channel, func(v string) { f.channel = v })
		})
		if f.channel == "" {
			kit.Muted(c, i18n.L("Only finished releases."))
		} else {
			kit.Muted(c, i18n.L("Also alpha, beta and release candidate builds. They get new features first and may have bugs."))
		}
		p.check(c, "autocheck", &f.autoCheck, i18n.L("Check for updates automatically"),
			i18n.L("Carryover tells you when there is a new version. It never installs one without asking."), false)
		ui.Divider(c)
		if !st.Supported {
			kit.Muted(c, i18n.L("Development builds don't update themselves."))
			return
		}
		switch {
		case st.Newer():
			kit.Callout(c, kit.BadgeInfo, kit.IconDownload,
				i18n.L("Version {{.Latest}} is available. You have {{.Current}}.", map[string]any{"Latest": st.Latest, "Current": st.Current}),
				func() {
					kit.Btn(c, nav, "update-open", kit.BtnPrimary, nil, i18n.L("View update"), p.u.ShowUpdate)
				})
		case st.Status == services.UpdateInstalled:
			kit.Muted(c, i18n.L("Restart Carryover to finish updating."))
		case st.Status == services.UpdateCurrent && st.Error == "":
			kit.Muted(c, i18n.L("Carryover is up to date."))
		}
		if st.Error != "" {
			kit.Callout(c, kit.BadgeErr, kit.IconAlert, st.Error, nil)
		}
		label := i18n.L("Check for updates")
		checking := st.Status == services.UpdateChecking
		if checking {
			label = i18n.L("Checking…")
		}
		kit.Btn(c, nav, "update-check", kit.BtnSecondary, kit.IconRefresh, label, func() { p.checkUpdates(true) },
			kit.BtnDisabled(checking || st.Status == services.UpdateDownloading || st.Status == services.UpdateInstalled),
			kit.BtnBusy(checking)).AlignSelf(ui.Start)
	})
}
