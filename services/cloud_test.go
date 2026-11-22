package services

import (
	"testing"
	"time"

	"github.com/rclone/rclone/lib/oauthutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/akfaiz/carryover/internal/app/appdir"
	"github.com/akfaiz/carryover/internal/cloudsync/cloud"
	"github.com/akfaiz/carryover/internal/core"
)

// A sign-in that waits for the browser can be cancelled: the step ends with
// ErrSignInCancelled, the port is free again and the remote it had begun (no
// token) is gone, so a half-made account is not left looking connected.
func TestCancelSignInStopsTheWaitAndDropsTheRemote(t *testing.T) {
	if err := cloud.CheckOAuthPort(); err != nil {
		t.Skipf("the sign-in port is busy: %v", err)
	}
	t.Setenv(appdir.EnvHome, t.TempDir())
	t.Cleanup(background.Wait)
	c, err := core.New()
	require.NoError(t, err)

	opened := make(chan string, 1)
	old := oauthutil.OpenURL
	oauthutil.OpenURL = func(url string) error { opened <- url; return nil }
	t.Cleanup(func() { oauthutil.OpenURL = old })

	s := NewCloudService(c)
	assert.False(t, s.CancelSignIn(), "nothing is waiting yet")

	q, err := s.CreateRemote("sv-dbx", "dropbox", nil)
	require.NoError(t, err)
	require.False(t, q.Done)
	require.Equal(t, "config_is_local", q.Name, "rclone asks first whether to use the web browser")

	type result struct {
		q   *cloud.Question
		err error
	}
	out := make(chan result, 1)
	go func() {
		next, err := s.ContinueRemote("sv-dbx", q.State, "true")
		out <- result{next, err}
	}()
	select {
	case <-opened:
	case <-time.After(10 * time.Second):
		t.Fatal("the browser was not opened")
	}
	require.True(t, s.CancelSignIn())
	select {
	case r := <-out:
		require.ErrorIs(t, r.err, ErrSignInCancelled)
		assert.Nil(t, r.q)
	case <-time.After(10 * time.Second):
		t.Fatal("still waiting for the browser after the cancel")
	}

	for _, r := range s.Remotes() {
		assert.NotEqual(t, "sv-dbx", r.Name, "the unfinished remote is dropped")
	}
	assert.False(t, s.CancelSignIn(), "nothing is waiting any more")
	require.NoError(t, cloud.CheckOAuthPort(), "the sign-in port is released")
}
