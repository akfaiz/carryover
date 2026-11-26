package shell

import (
	"log"
	"os"

	"github.com/egoist/mygo"

	"github.com/akfaiz/carryover/internal/app/singleinstance"
	"github.com/akfaiz/carryover/services"
	"github.com/akfaiz/carryover/ui/i18n"
	"github.com/akfaiz/carryover/ui/kit"
)

// RunMain builds the main window, runs the application until it quits, and
// returns. inst (nil without a single-instance guard) is asked to show the
// window when a second launch finds this one running.
func (u *Shell) RunMain(inst *singleinstance.Instance) {
	u.instance = inst
	mygo.App.WhenReady(func() {
		u.BuildMain()
		u.startUpdates()
		if inst != nil {
			inst.OnShow(func() { kit.OnUI(func() { u.win.Show(); u.win.Focus() }) })
		}
		u.win.OnFocus(func() {
			kit.OnUI(u.checkLocalChanges)
			kit.OnUI(func() { u.readPad(true) })
			u.showNotices()
		})
		u.win.OnBlur(func() { kit.OnUI(func() { u.readPad(false) }) })
		kit.OnUI(u.checkLocalChanges)
		u.showNotices()
		go func() {
			if err := u.sync.RecoverSessions(); err != nil {
				log.Printf("recover sessions: %v", err)
			}
		}()
		kit.Async0(u.sync.SyncLibrary, func(err error) {
			if services.IsOffline(err) {
				u.toast(i18n.L("Offline — library not synced"), kit.ToastInfo)
			} else if err != nil {
				u.toast(i18n.L("Library sync: {{.Err}}", map[string]any{"Err": err}), kit.ToastErr)
			}
		})
	})
	if err := mygo.App.Run(); err != nil {
		log.Printf("ui: %v", err)
		os.Exit(1)
	}
}
