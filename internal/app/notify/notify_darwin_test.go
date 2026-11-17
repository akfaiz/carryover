//go:build darwin

package notify

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The title and text are passed as arguments after the script, unquoted, so
// quotes in a game's name can't break it.
func TestSendArgs(t *testing.T) {
	var got []string
	osascript = func(args ...string) ([]byte, error) { got = args; return nil, nil }
	t.Cleanup(func() { osascript = defaultOsascript })

	require.NoError(t, Send(App{ID: "com.akfaiz.carryover", Name: "Carryover"}, `Say "hi"`, "it's done"))
	require.Len(t, got, len(script)+2)
	assert.Equal(t, script, got[:len(script)])
	assert.Equal(t, []string{`Say "hi"`, "it's done"}, got[len(script):])

	osascript = func(...string) ([]byte, error) { return nil, errors.New("boom") }
	require.Error(t, Send(App{}, "a", "b"))
}
