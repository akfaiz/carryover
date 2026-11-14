// Package notify shows desktop notifications from a process without a window
// (the session watcher): through the freedesktop notification service on
// Linux, and as a toast on Windows. It has no user-visible text of its own.
package notify

// App identifies the application a notification comes from.
type App struct {
	ID   string // reverse-DNS id, e.g. com.akfaiz.carryover (the desktop entry / AppUserModelID)
	Name string // shown as the sender
}

// Send shows a notification with title and body. It returns an error when no
// notification could be shown (no notification service, as in Steam Deck Game
// Mode, or the call failed).
func Send(app App, title, body string) error {
	return send(app, title, body)
}
