//go:build !windows

package notify

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// Without a notification service Send fails, so the caller can keep the
// notice for later (Steam Deck Game Mode).
func TestSendWithoutBus(t *testing.T) {
	t.Setenv("DBUS_SESSION_BUS_ADDRESS", "unix:path="+filepath.Join(t.TempDir(), "no-bus"))
	require.Error(t, Send(App{ID: "com.akfaiz.carryover", Name: "Carryover"}, "Carryover", "test"))
}
