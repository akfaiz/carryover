package cloud

import (
	"errors"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExplainOAuthError(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("windows-specific message")
	}
	in := errors.New("failed to start auth webserver: listen tcp 127.0.0.1:53682: bind: An attempt was made to access a socket in a way forbidden by its access permissions.")
	got := explainOAuthError(in).Error()
	require.Contains(t, got, "net stop winnat")
	require.True(t, strings.HasPrefix(got, in.Error()), "unexpected message: %s", got)
	require.Equal(t, got, explainOAuthError(explainOAuthError(in)).Error(), "hint added twice")
	other := errors.New("some other error")
	require.Equal(t, other.Error(), explainOAuthError(other).Error(), "unrelated error changed")
}

func TestUsesOAuth(t *testing.T) {
	assert.True(t, usesOAuth("dropbox"))
	assert.False(t, usesOAuth("sftp"))
}

func TestCheckOAuthPortReportsBlockedPort(t *testing.T) {
	// On this machine the port may be reserved; either outcome is fine, but a
	// failure must carry the remediation hint.
	if err := CheckOAuthPort(); err != nil {
		require.ErrorContains(t, err, "53682")
	}
}

func TestExplainOAuthErrorPortInUse(t *testing.T) {
	in := errors.New("listen tcp 127.0.0.1:53682: bind: address already in use")
	got := explainOAuthError(in)
	if runtime.GOOS == "windows" {
		// "bind" errors get the Windows reserved-port hint there.
		require.ErrorContains(t, got, "net stop winnat")
	} else {
		require.ErrorContains(t, got, "Another program")
	}
	require.True(t, strings.HasPrefix(got.Error(), in.Error()), "message = %v", got)
	require.Equal(t, got.Error(), explainOAuthError(got).Error(), "hint added twice")
	require.Nil(t, explainOAuthError(nil), "nil error changed")
	// An error that doesn't mention the OAuth port is returned as is.
	other := errors.New("address already in use")
	require.Equal(t, other, explainOAuthError(other), "unrelated error changed")
}
