package cloud

import (
	"errors"
	"runtime"
	"strings"
	"testing"
)

func TestExplainOAuthError(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("windows-specific message")
	}
	in := errors.New("failed to start auth webserver: listen tcp 127.0.0.1:53682: bind: An attempt was made to access a socket in a way forbidden by its access permissions.")
	got := explainOAuthError(in).Error()
	if !strings.Contains(got, "net stop winnat") || !strings.HasPrefix(got, in.Error()) {
		t.Fatalf("unexpected message: %s", got)
	}
	if explainOAuthError(explainOAuthError(in)).Error() != got {
		t.Fatal("hint added twice")
	}
	other := errors.New("some other error")
	if got := explainOAuthError(other); got.Error() != other.Error() {
		t.Fatal("unrelated error changed")
	}
}

func TestUsesOAuth(t *testing.T) {
	if !usesOAuth("dropbox") || usesOAuth("sftp") {
		t.Fatal("usesOAuth wrong")
	}
}

func TestCheckOAuthPortReportsBlockedPort(t *testing.T) {
	// On this machine the port may be reserved; either outcome is fine, but a
	// failure must carry the remediation hint.
	if err := CheckOAuthPort(); err != nil && !strings.Contains(err.Error(), "53682") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestExplainOAuthErrorPortInUse(t *testing.T) {
	in := errors.New("listen tcp 127.0.0.1:53682: bind: address already in use")
	got := explainOAuthError(in)
	if !strings.Contains(got.Error(), "Another program") || !strings.HasPrefix(got.Error(), in.Error()) {
		t.Fatalf("message = %v", got)
	}
	if again := explainOAuthError(got); again.Error() != got.Error() {
		t.Fatal("hint added twice")
	}
	if explainOAuthError(nil) != nil {
		t.Fatal("nil error changed")
	}
	// An error that doesn't mention the OAuth port is returned as is.
	other := errors.New("address already in use")
	if explainOAuthError(other) != other {
		t.Fatal("unrelated error changed")
	}
}
