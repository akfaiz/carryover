package ui

import (
	_ "embed"

	"github.com/akfaiz/carryover/internal/core"
	"github.com/akfaiz/carryover/ui/shell"
)

//go:embed assets/appicon.png
var appIcon []byte

// The sidebar's logo, for each theme. appIcon, the icon of the window and of
// the system, is the dark one in every theme.
var (
	//go:embed assets/appicon.svg
	logoDark []byte
	//go:embed assets/appicon-light.svg
	logoLight []byte
)

// AppID identifies the app to desktop notifications.
const AppID = "com.akfaiz.carryover"

// NoticeText is the desktop notification for what the session watcher did, in
// the language chosen in Settings (the watcher has no window).
func NoticeText(c *core.Core, n core.Notice) (title, body string) {
	return shell.NoticeText(c, n)
}
