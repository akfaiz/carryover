package detach

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestWithoutSteam(t *testing.T) {
	env := []string{"PATH=/bin", "SteamGameId=123", "SteamAppId=4", "STEAM_COMPAT_DATA_PATH=/x", "HOME=/home/deck", "CARRYOVER_HOME=/d"}
	require.Equal(t, []string{"PATH=/bin", "HOME=/home/deck", "CARRYOVER_HOME=/d"}, withoutSteam(env))
	require.Equal(t, "SteamGameId=123", env[1], "input changed")
}

// Start really starts the process (through WMI on Windows, systemd-run or a
// new session on Linux), with an argument that needs quoting.
func TestStart(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "it's here")
	require.NoError(t, os.MkdirAll(dir, 0o755))
	made := filepath.Join(dir, "made by start")
	exe, args := `C:\Windows\System32\cmd.exe`, []string{"/c", "mkdir", made}
	if runtime.GOOS != "windows" {
		p, err := exec.LookPath("mkdir")
		require.NoError(t, err)
		exe, args = p, []string{made}
	}
	require.NoError(t, Start(exe, args))
	require.Eventually(t, func() bool {
		_, err := os.Stat(made)
		return err == nil
	}, 20*time.Second, 100*time.Millisecond, "started process didn't run")
}

func TestWhere(t *testing.T) {
	w := Where()
	require.NotEmpty(t, w)
	if runtime.GOOS == "windows" {
		require.True(t, strings.Contains(w, "job object") && strings.Contains(w, "parent "), w)
	}
}
