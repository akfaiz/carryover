//go:build windows

package notify

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// Titles and bodies go into the toast script as PowerShell string literals:
// a game name with an apostrophe must not end the string early.
func TestPSQuote(t *testing.T) {
	require.Equal(t, `'Baldur''s Gate 3: saves uploaded'`, psQuote(`Baldur's Gate 3: saves uploaded`))
	require.Equal(t, `''`, psQuote(""))
}
