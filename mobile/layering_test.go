package mobile

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// coreReads are the internal/core calls mobile may make directly. They only
// read. Anything that changes a game, an emulator, the settings or a game's
// saves goes through services, which holds the locks, the session checks
// (ErrPlaying) and the Bus events the desktop app gets too. Add a read here;
// add a write to services instead.
var coreReads = []string{
	"CloudKeyOf",
	"DeviceID",
	"Dirs.LogsDir",
	"Library.Emulator",
	"Library.Get",
	"OpenSessions",
	"PendingSessions",
	"ProfileName",
	"SessionOpen",
}

var coreUse = regexp.MustCompile(`\.core((?:\.[A-Z]\w*)+)`)

// mobile changes nothing through internal/core directly (see coreReads).
func TestMobileWritesThroughServices(t *testing.T) {
	files, err := filepath.Glob("*.go")
	require.NoError(t, err)
	seen := 0
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		src, err := os.ReadFile(f)
		require.NoError(t, err)
		for _, m := range coreUse.FindAllStringSubmatch(string(src), -1) {
			seen++
			call := strings.TrimPrefix(m[1], ".")
			assert.True(t, slices.Contains(coreReads, call),
				"%s calls core.%s directly: go through services if it changes anything, or add it to coreReads if it only reads", f, call)
		}
	}
	assert.Positive(t, seen, "the pattern finds mobile's core calls")
}
