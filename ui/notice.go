package ui

import (
	"sync"

	"fyne.io/fyne/v2/lang"

	"github.com/akfaiz/carryover/internal/core"
	"github.com/akfaiz/carryover/internal/ui/kit"
)

var translationsOnce sync.Once

// loadTranslations registers the catalogs for the language setting, once per
// process (the GUI at startup, the session watcher before its notification).
func loadTranslations(language string) {
	translationsOnce.Do(func() {
		startLanguage = language
		initTranslations(language)
	})
}

// NoticeText is the desktop notification for what the session watcher did, in
// the language chosen in Settings. The watcher has no window: it calls this
// before sending the notification.
func NoticeText(c *core.Core, n core.Notice) (title, body string) {
	loadTranslations(c.Settings.Get().Language)
	return "Carryover", noticeBody(n)
}

func noticeBody(n core.Notice) string {
	switch n.Kind {
	case core.NoticeUploaded:
		return lang.L("{{.Game}}: saves uploaded after Steam closed the game.", map[string]any{"Game": n.Title})
	case core.NoticeConflict:
		return lang.L("{{.Game}}: saves changed here and on another device. Open Carryover to choose which to keep.", map[string]any{"Game": n.Title})
	}
	return lang.L("{{.Game}}: saves could not be uploaded after Steam closed the game: {{.Err}}", map[string]any{"Game": n.Title, "Err": n.Err})
}

func noticeToastKind(k core.NoticeKind) kit.ToastKind {
	switch k {
	case core.NoticeUploaded:
		return kit.ToastOK
	case core.NoticeConflict:
		return kit.ToastInfo
	}
	return kit.ToastErr
}

// showNotices shows, once, what session watchers did while no notification
// could be shown (e.g. in Steam Deck Game Mode).
func (u *UI) showNotices() {
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
