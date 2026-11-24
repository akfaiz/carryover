package ui

import (
	_ "embed"

	"github.com/akfaiz/carryover/internal/core"
	"github.com/akfaiz/carryover/ui/shell"
)

//go:embed assets/appicon.png
var appIcon []byte

// AppID identifies the app (toolkit preferences, single instance).
const AppID = "com.akfaiz.carryover"

// NoticeText is the desktop notification for what the session watcher did, in
// the language chosen in Settings (the watcher has no window).
func NoticeText(c *core.Core, n core.Notice) (title, body string) {
	return shell.NoticeText(c, n)
}
