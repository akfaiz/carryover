package services

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/akfaiz/carryover/internal/cloudsync/cloud"
	"github.com/akfaiz/carryover/internal/core"
)

// ErrSignInCancelled is what CreateRemote and ContinueRemote return when
// CancelSignIn stopped the wait for the browser.
var ErrSignInCancelled = errors.New("sign-in cancelled")

// signInTimeout is how long a step waits for the user to sign in in the browser.
const signInTimeout = 10 * time.Minute

// CloudService configures rclone remotes.
type CloudService struct {
	c *core.Core

	signMu     sync.Mutex
	signCancel context.CancelFunc // stops the step that waits for the browser, if any
	signSeq    int
}

// NewCloudService creates the service.
func NewCloudService(c *core.Core) *CloudService { return &CloudService{c: c} }

// Providers lists supported storage providers.
func (s *CloudService) Providers() []cloud.Provider { return cloud.Providers() }

// Remotes lists configured remotes.
func (s *CloudService) Remotes() []cloud.Remote {
	r := cloud.ListRemotes()
	if r == nil {
		r = []cloud.Remote{}
	}
	return r
}

// signIn starts a step that may wait for the browser: its context ends after
// signInTimeout or when CancelSignIn is called, and done releases it.
func (s *CloudService) signIn() (ctx context.Context, done func()) {
	ctx, cancel := context.WithTimeout(context.Background(), signInTimeout)
	s.signMu.Lock()
	s.signSeq++
	seq := s.signSeq
	s.signCancel = cancel
	s.signMu.Unlock()
	return ctx, func() {
		s.signMu.Lock()
		if s.signSeq == seq {
			s.signCancel = nil
		}
		s.signMu.Unlock()
		cancel()
	}
}

// CancelSignIn stops the wait for the browser of a CreateRemote or ContinueRemote
// that is running (which then fails with ErrSignInCancelled and drops the remote
// it had begun), and reports whether there was one.
func (s *CloudService) CancelSignIn() bool {
	s.signMu.Lock()
	cancel := s.signCancel
	s.signCancel = nil
	s.signMu.Unlock()
	if cancel == nil {
		return false
	}
	cancel()
	return true
}

// signInError turns the error of a step that CancelSignIn stopped into
// ErrSignInCancelled and drops the remote that never got a token.
func signInError(ctx context.Context, name string, err error) error {
	if err != nil && errors.Is(ctx.Err(), context.Canceled) {
		cloud.DropUnfinished(name)
		return ErrSignInCancelled
	}
	return err
}

// CreateRemote starts creating a remote. OAuth providers open the browser and
// this call returns once authorisation finishes, is cancelled with CancelSignIn
// or times out after 10 minutes. A failure leaves no half-made OAuth remote behind.
func (s *CloudService) CreateRemote(name, typ string, params map[string]string) (*cloud.Question, error) {
	ctx, done := s.signIn()
	defer done()
	q, err := cloud.CreateRemote(ctx, name, typ, params)
	if err != nil {
		cloud.DropUnfinished(name) // the form is shown again: CreateRemote starts over
		return nil, signInError(ctx, name, err)
	}
	return q, nil
}

// ContinueRemote answers a configuration question. Like CreateRemote it can
// wait for the browser; after a cancel the remote it had begun is dropped.
func (s *CloudService) ContinueRemote(name, state, result string) (*cloud.Question, error) {
	ctx, done := s.signIn()
	defer done()
	q, err := cloud.ContinueRemote(ctx, name, state, result)
	if err != nil {
		return nil, signInError(ctx, name, err)
	}
	return q, nil
}

// DeleteRemote removes a remote. If it is the active one, sync is disabled.
func (s *CloudService) DeleteRemote(name string) error {
	cloud.DeleteRemote(name)
	st := s.c.Settings.Get()
	if st.Remote == name {
		st.Remote = ""
		return s.c.Settings.Update(st)
	}
	return nil
}

// Test checks access to remote:root.
func (s *CloudService) Test(remote, root string) error {
	if remote == "" {
		return errors.New("choose a remote")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	return cloud.Test(ctx, remote+":"+root)
}

// DefaultRcloneConfig returns the path of an existing standard rclone.conf, or "".
func (s *CloudService) DefaultRcloneConfig() string {
	var cands []string
	if v := os.Getenv("RCLONE_CONFIG"); v != "" {
		cands = append(cands, v)
	}
	home, _ := os.UserHomeDir()
	if runtime.GOOS == "windows" {
		cands = append(cands, filepath.Join(os.Getenv("APPDATA"), "rclone", "rclone.conf"))
	}
	if cfg, err := os.UserConfigDir(); err == nil {
		cands = append(cands, filepath.Join(cfg, "rclone", "rclone.conf"))
	}
	cands = append(cands, filepath.Join(home, ".config", "rclone", "rclone.conf"), filepath.Join(home, ".rclone.conf"))
	for _, c := range cands {
		if fi, err := os.Stat(c); err == nil && !fi.IsDir() {
			return c
		}
	}
	return ""
}

// ImportConfigData imports remotes of supported providers from the contents of
// an rclone.conf (the Android app has no file path to give).
func (s *CloudService) ImportConfigData(contents string) (cloud.ImportResult, error) {
	if strings.TrimSpace(contents) == "" {
		return cloud.ImportResult{}, errors.New("the file is empty")
	}
	return cloud.ImportConfigData([]byte(contents))
}

// ImportConfig imports remotes of supported providers from an rclone.conf.
func (s *CloudService) ImportConfig(path string) (cloud.ImportResult, error) {
	if path == "" {
		return cloud.ImportResult{}, errors.New("choose an rclone.conf file")
	}
	return cloud.ImportConfig(path)
}
