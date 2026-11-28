package cloud

import (
	"fmt"
	"net"
	"runtime"
	"strings"

	"github.com/rclone/rclone/fs"

	"github.com/akfaiz/carryover/internal/app/msg"
)

// oauthAddr is where rclone's OAuth helper listens for the browser redirect.
// rclone hard-codes this port, and providers only accept rclone's redirect URL
// on it, so it cannot be changed without a custom client id.
const oauthAddr = "127.0.0.1:53682"

func usesOAuth(typ string) bool {
	ri, err := fs.Find(typ)
	if err != nil {
		return false
	}
	for _, o := range ri.Options {
		if o.Name == "token" {
			return true
		}
	}
	return false
}

// CheckOAuthPort verifies that the OAuth callback port can be bound.
func CheckOAuthPort() error {
	l, err := net.Listen("tcp", oauthAddr)
	if err != nil {
		return explainOAuthError(fmt.Errorf("failed to start auth webserver: %w", err))
	}
	return l.Close()
}

// explainOAuthError adds remediation steps when the OAuth port can't be bound.
func explainOAuthError(err error) error {
	if err == nil {
		return nil
	}
	text := err.Error()
	if !strings.Contains(text, "53682") || strings.Contains(text, "To fix") || strings.Contains(text, "Another program") {
		return err
	}
	switch {
	case runtime.GOOS == "windows" && (strings.Contains(text, "forbidden by its access permissions") || strings.Contains(text, "bind")):
		return msg.CloudPortReserved.Err("Detail", text)
	case strings.Contains(text, "address already in use") || strings.Contains(text, "Only one usage"):
		return msg.CloudPortInUse.Err("Detail", text)
	}
	return err
}
