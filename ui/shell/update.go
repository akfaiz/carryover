package shell

import (
	"context"
	"log"
	"time"

	"github.com/egoist/mygo"

	"github.com/akfaiz/carryover/internal/app/update"
	"github.com/akfaiz/carryover/internal/core"
	"github.com/akfaiz/carryover/services"
	"github.com/akfaiz/carryover/ui/components"
	"github.com/akfaiz/carryover/ui/i18n"
	"github.com/akfaiz/carryover/ui/kit"
)

// updateTick is how often a running Carryover considers looking for updates;
// the service itself reads the releases at most every 12 hours.
const updateTick = time.Hour

// updateTarget describes this program to the update service.
func updateTarget() update.Target {
	exe, err := core.SelfExecutable()
	if err != nil {
		log.Printf("update: %v", err)
	}
	return update.DesktopTarget(exe)
}

// startUpdates says how the last update went and then looks for newer
// versions, now and while the app stays open.
func (u *Shell) startUpdates() {
	go func() {
		if out := u.upd.Startup(); out != (services.UpdateOutcome{}) {
			kit.OnUI(func() { u.updateOutcome(out) })
		}
		check := func() {
			if !u.upd.AutoCheck() {
				return
			}
			// A failure is logged by the service and shows in Settings; offline
			// is no reason to bother the user.
			_, _ = u.upd.Check(context.Background(), false)
		}
		check()
		for range time.Tick(updateTick) {
			check()
		}
	}()
}

func (u *Shell) updateOutcome(out services.UpdateOutcome) {
	if out.UpdatedTo != "" {
		u.toast(i18n.L("Updated to version {{.Version}}", map[string]any{"Version": out.UpdatedTo}), kit.ToastOK)
		return
	}
	u.toast(i18n.L("Version {{.Version}} wasn't installed. Try updating again.", map[string]any{"Version": out.Failed}), kit.ToastErr)
}

// announceUpdate tells about a newer version once; the sidebar keeps showing it.
func (u *Shell) announceUpdate() {
	st := u.store.Update
	if !st.Newer() || st.Latest == u.toldUpdate {
		return
	}
	u.toldUpdate = st.Latest
	u.toast(i18n.L("Carryover {{.Version}} is available", map[string]any{"Version": st.Latest}), kit.ToastInfo)
}

// updateEntry is the sidebar's line about a newer version, "" when there is
// nothing to say.
func (u *Shell) updateEntry() string {
	st := u.store.Update
	switch st.Status {
	case services.UpdateDownloading:
		return i18n.L("Downloading update…")
	case services.UpdateReady:
		return i18n.L("Update ready to install")
	case services.UpdateInstalled:
		return i18n.L("Restart to finish updating")
	}
	if st.Newer() {
		return i18n.L("Update available: {{.Version}}", map[string]any{"Version": st.Latest})
	}
	return ""
}

func (u *Shell) ShowUpdate() { components.ShowUpdate(u) }

func (u *Shell) Busy() bool { return u.busy() }

func (u *Shell) RestartForUpdate(finish func(restart bool) error) {
	// The new version must not find this one still answering as the running
	// instance: it would only ask it to show itself, and exit.
	if u.instance != nil {
		u.instance.Close()
	}
	if err := finish(true); err != nil {
		log.Printf("update: restart: %v", err)
		u.toast(i18n.L("Carryover couldn't restart: {{.Err}}", map[string]any{"Err": i18n.Err(err)}), kit.ToastErr)
		return
	}
	u.closing = true
	if u.win != nil {
		mygo.App.Quit()
	}
}
