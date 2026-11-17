//go:build windows || darwin

package gamemode

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// On Windows and macOS the keyboard isn't requested (see CanRequestKeyboard).
func TestNoKeyboardRequest(t *testing.T) {
	assert.False(t, CanRequestKeyboard)
	ran := false
	run = func(context.Context, []string, string, ...string) ([]byte, error) { ran = true; return nil, nil }
	t.Cleanup(func() { run = runDefault })
	_, err := OpenKeyboard()
	require.Error(t, err)
	assert.False(t, ran, "nothing is started")
}
