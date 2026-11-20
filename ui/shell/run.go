package shell

import (
	"log"

	"fyne.io/fyne/v2/lang"

	"github.com/akfaiz/carryover/internal/app/singleinstance"
	"github.com/akfaiz/carryover/services"
	"github.com/akfaiz/carryover/ui/kit"
)

// RunMain opens the launcher window and blocks until it closes. inst is the
// single-instance guard (nil when it could not be taken); a second launch asks
// it to show the window.
func (u *Shell) RunMain(inst *singleinstance.Instance) {
	u.BuildMain()
	if inst != nil {
		inst.OnShow(func() { kit.OnUI(func() { u.win.Show(); u.win.RequestFocus() }) })
	}
	// Coming back to the launcher (e.g. after playing without it) looks
	// for saves that changed since their last sync.
	u.app.Lifecycle().SetOnEnteredForeground(func() {
		kit.OnUI(u.checkLocalChanges)
		kit.OnUI(func() { u.readPad(true) })
		u.showNotices()
	})
	// A game started from here is played with the same controller: leave it
	// alone while another window is in front.
	u.app.Lifecycle().SetOnExitedForeground(func() { kit.OnUI(func() { u.readPad(false) }) })
	// Pick up games other devices added while this one was closed.
	u.app.Lifecycle().SetOnStarted(func() {
		kit.OnUI(u.checkLocalChanges)
		u.showNotices()
		// A session killed earlier (e.g. Steam's "Exit Game") may have left
		// its cloud lock behind: release it so other devices stop warning.
		go func() {
			if err := u.sync.RecoverSessions(); err != nil {
				log.Printf("recover sessions: %v", err)
			}
		}()
		kit.Async0(u.sync.SyncLibrary, func(err error) {
			if services.IsOffline(err) {
				u.toast(lang.L("Offline — library not synced"), kit.ToastInfo)
			} else if err != nil {
				u.toast(lang.L("Library sync: {{.Err}}", map[string]any{"Err": err}), kit.ToastErr)
			}
		})
	})
	u.win.ShowAndRun()
}
