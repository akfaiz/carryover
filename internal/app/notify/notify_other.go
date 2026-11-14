//go:build !windows

package notify

import (
	"github.com/godbus/dbus/v5"
)

// send calls org.freedesktop.Notifications.Notify on the session bus. Without
// DBUS_SESSION_BUS_ADDRESS (a systemd-run service may not have it) godbus
// falls back to $XDG_RUNTIME_DIR/bus.
func send(app App, title, body string) error {
	conn, err := dbus.ConnectSessionBus()
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close() }()
	hints := map[string]dbus.Variant{"desktop-entry": dbus.MakeVariant(app.ID)}
	obj := conn.Object("org.freedesktop.Notifications", "/org/freedesktop/Notifications")
	return obj.Call("org.freedesktop.Notifications.Notify", 0,
		app.Name, uint32(0), "", title, body, []string{}, hints, int32(-1)).Err
}
