package shell

import (
	"github.com/akfaiz/carryover/internal/core"
	"github.com/akfaiz/carryover/ui/i18n"
	"github.com/akfaiz/carryover/ui/kit"
)

// NoticeText is the desktop notification for what the session watcher did, in
// the language chosen in Settings. The watcher has no window: it calls this
// before sending the notification.
func NoticeText(c *core.Core, n core.Notice) (title, body string) {
	i18n.Load(c.Settings.Get().Language)
	return "Carryover", noticeBody(n)
}

func noticeBody(n core.Notice) string {
	switch n.Kind {
	case core.NoticeUploaded:
		return i18n.L("{{.Game}}: saves uploaded after Steam closed the game.", map[string]any{"Game": n.Title})
	case core.NoticeConflict:
		return i18n.L("{{.Game}}: saves changed here and on another device. Open Carryover to choose which to keep.", map[string]any{"Game": n.Title})
	case core.NoticeOffline:
		return i18n.L("{{.Game}}: saves were not uploaded after Steam closed the game: no internet connection. They upload on the next sync.", map[string]any{"Game": n.Title})
	}
	return i18n.L("{{.Game}}: saves could not be uploaded after Steam closed the game: {{.Err}}", map[string]any{"Game": n.Title, "Err": n.Err})
}

func noticeToastKind(k core.NoticeKind) kit.ToastKind {
	switch k {
	case core.NoticeUploaded:
		return kit.ToastOK
	case core.NoticeConflict, core.NoticeOffline:
		return kit.ToastInfo
	}
	return kit.ToastErr
}

// showNotices shows, once, what session watchers did while no notification
// could be shown (e.g. in Steam Deck Game Mode).
func (u *Shell) showNotices() {
	go func() {
		notices := u.sync.TakeNotices()
		if len(notices) == 0 {
			return
		}
		kit.OnUI(func() {
			for _, n := range notices {
				u.toast(noticeBody(n), noticeToastKind(n.Kind))
			}
		})
	}()
}
