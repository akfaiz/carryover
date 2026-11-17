package detach

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// The started process ends up adopted by launchd, not under this process:
// Steam signals every process it started when it stops a game.
func TestStartIsAdoptedByLaunchd(t *testing.T) {
	out := filepath.Join(t.TempDir(), "ppid")
	require.NoError(t, Start("/bin/sh", []string{"-c", `sleep 1; ps -o ppid= -p $$ > "$0"`, out}))
	var ppid string
	require.Eventually(t, func() bool {
		b, err := os.ReadFile(out)
		ppid = strings.TrimSpace(string(b))
		return err == nil && ppid != ""
	}, 20*time.Second, 100*time.Millisecond, "started process didn't run")
	require.Equal(t, "1", ppid)
}
