package mobile

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/rclone/rclone/lib/oauthutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/akfaiz/carryover/internal/cloudsync/cloud"
	"github.com/akfaiz/carryover/services"
)

// The browser question is answered by the app, so the sign-in starts at once;
// cancelling it leaves no account behind. Other questions reach the user as they are.
// (This file sorts after mobile_test.go: TestFlow's Init sets rclone's config first.)
func TestSignInAnswersTheBrowserQuestionAndCancelDropsTheAccount(t *testing.T) {
	require.NoError(t, cloud.Init(filepath.Join(t.TempDir(), "rclone.conf"))) // once per process: a no-op after TestFlow
	if err := cloud.CheckOAuthPort(); err != nil {
		t.Skipf("the sign-in port is busy: %v", err)
	}
	opened := make(chan string, 1)
	old := oauthutil.OpenURL
	oauthutil.OpenURL = func(url string) error { opened <- url; return nil }
	t.Cleanup(func() { oauthutil.OpenURL = old })

	s := &state{cloud: services.NewCloudService(nil)}
	q, err := s.cloud.CreateRemote("mb-dbx", "dropbox", nil)
	require.NoError(t, err)
	require.Equal(t, browserQuestion, q.Name, "rclone's first question")

	type result struct {
		q   *cloud.Question
		err error
	}
	out := make(chan result, 1)
	go func() {
		next, err := s.answerBrowser("mb-dbx", q, nil)
		out <- result{next, err}
	}()
	select {
	case <-opened:
	case <-time.After(10 * time.Second):
		t.Fatal("answering the browser question did not open the browser")
	}
	require.True(t, s.cloud.CancelSignIn())
	select {
	case r := <-out:
		require.ErrorIs(t, r.err, services.ErrSignInCancelled)
		assert.Nil(t, r.q)
	case <-time.After(10 * time.Second):
		t.Fatal("still waiting for the browser after the cancel")
	}
	for _, r := range s.cloud.Remotes() {
		assert.NotEqual(t, "mb-dbx", r.Name, "the account of a cancelled sign-in is dropped")
	}

	other := &cloud.Question{Name: "config_team_drive", State: "x"}
	got, err := s.answerBrowser("any", other, nil)
	require.NoError(t, err)
	assert.Same(t, other, got, "a question that is not the browser one is passed on")
	done := &cloud.Question{Done: true}
	got, err = s.answerBrowser("any", done, nil)
	require.NoError(t, err)
	assert.Same(t, done, got)
}

func TestCancelSignInWithNothingWaiting(t *testing.T) {
	assert.False(t, CancelSignIn(), "no sign-in is running (or the core isn't open)")
}
