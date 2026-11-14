//go:build windows

package detach

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// Start really starts the process through WMI, with arguments that need
// quoting (a space and an apostrophe, as in "C:\Users\O'Brien Games").
func TestStartWMI(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "it's here")
	require.NoError(t, os.MkdirAll(dir, 0o755))
	made := filepath.Join(dir, "made by wmi")
	require.NoError(t, startWMI(`C:\Windows\System32\cmd.exe`, []string{"/c", "mkdir", made}))
	require.Eventually(t, func() bool {
		_, err := os.Stat(made)
		return err == nil
	}, 20*time.Second, 100*time.Millisecond, "process started through WMI didn't run")
}

func TestPSQuote(t *testing.T) {
	require.Equal(t, `'a ''b'' c'`, psQuote(`a 'b' c`))
}
