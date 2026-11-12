package cloud

import (
	"errors"
	"fmt"
	"net"
	"runtime"
	"strings"

	"github.com/rclone/rclone/fs"
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
	msg := err.Error()
	if !strings.Contains(msg, "53682") || strings.Contains(msg, "To fix") || strings.Contains(msg, "Another program") {
		return err
	}
	var hint string
	switch {
	case runtime.GOOS == "windows" && (strings.Contains(msg, "forbidden by its access permissions") || strings.Contains(msg, "bind")):
		hint = "Windows has reserved TCP port 53682, which sign-in needs (usually done by Hyper-V, WSL2 or Docker). " +
			"To fix, run in an Administrator terminal:\n" +
			"  net stop winnat\n" +
			"  netsh int ipv4 add excludedportrange protocol=tcp startport=53682 numberofports=1\n" +
			"  net start winnat\n" +
			"then try again. Check reserved ranges with: netsh int ipv4 show excludedportrange protocol=tcp"
	case strings.Contains(msg, "address already in use") || strings.Contains(msg, "Only one usage"):
		hint = "Another program (often another rclone sign-in) is using port 53682. Close it and try again."
	default:
		return err
	}
	return errors.New(msg + "\n\n" + hint)
}
