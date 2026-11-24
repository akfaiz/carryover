package shell

import (
	"github.com/egoist/mygo"

	"github.com/akfaiz/carryover/services"
	"github.com/akfaiz/carryover/ui/env"
	"github.com/akfaiz/carryover/ui/kit"
	"github.com/akfaiz/carryover/ui/store"
)

var _ env.Env = (*Shell)(nil)

func (u *Shell) Window() *mygo.Window                    { return u.win }
func (u *Shell) Navigator() *kit.Navigator               { return u.nav }
func (u *Shell) Dialogs() *kit.Dialogs                   { return &u.dialogs }
func (u *Shell) Store() *store.Store                     { return u.store }
func (u *Shell) Lib() *services.LibraryService           { return u.lib }
func (u *Shell) Sync() *services.SyncService             { return u.sync }
func (u *Shell) Cloud() *services.CloudService           { return u.cloud }
func (u *Shell) Steam() *services.SteamService           { return u.steam }
func (u *Shell) Emus() *services.EmulatorService         { return u.emus }
func (u *Shell) Sess() *services.SessionService          { return u.sess }
func (u *Shell) Navigate(path string)                    { u.navigate(path) }
func (u *Shell) Replace(path string)                     { u.replace(path) }
func (u *Shell) Back()                                   { u.back() }
func (u *Shell) Toast(text string, k kit.ToastKind)      { u.toast(text, k) }
func (u *Shell) ToastErr(err error)                      { u.toastErr(err) }
func (u *Shell) Confirm(o kit.ConfirmOptions, ok func()) { u.confirm.Show(o, ok); kit.Redraw() }
func (u *Shell) OpenFolder(path string)                  { u.openFolder(path) }
func (u *Shell) AddGame()                                { u.cfg.AddGame(u) }
func (u *Shell) SyncAll()                                { u.syncAll() }
func (u *Shell) ToggleFullScreen()                       { u.toggleFullScreen() }
func (u *Shell) FocusForPad()                            { u.focusForPad() }
func (u *Shell) ShowNotices()                            { u.showNotices() }

func (u *Shell) HelpOpen() map[string]bool {
	if u.helpOpen == nil {
		u.helpOpen = map[string]bool{}
	}
	return u.helpOpen
}

func (u *Shell) OpenURL(url string) error { return mygo.Shell.OpenExternal(url) }
func (u *Shell) Prefs() env.Prefs         { return u.prefs }
