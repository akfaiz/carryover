package services

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"

	"github.com/akfaiz/playrelay/internal/cloud"
	"github.com/akfaiz/playrelay/internal/core"
)

// CloudService configures rclone remotes.
type CloudService struct {
	c *core.Core
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

// CreateRemote starts creating a remote. OAuth providers open the browser and
// this call returns once authorisation finishes (or times out after 10 minutes).
func (s *CloudService) CreateRemote(name, typ string, params map[string]string) (*cloud.Question, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	return cloud.CreateRemote(ctx, name, typ, params)
}

// ContinueRemote answers a configuration question.
func (s *CloudService) ContinueRemote(name, state, result string) (*cloud.Question, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	return cloud.ContinueRemote(ctx, name, state, result)
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

// ImportConfig imports remotes of supported providers from an rclone.conf.
// If path is empty a file dialog is shown.
func (s *CloudService) ImportConfig(path string) (cloud.ImportResult, error) {
	if path == "" {
		p, err := application.Get().Dialog.OpenFile().SetTitle("Select rclone.conf").
			AddFilter("rclone config", "*.conf").PromptForSingleSelection()
		if err != nil || p == "" {
			return cloud.ImportResult{Imported: []string{}, Skipped: []string{}}, err
		}
		path = p
	}
	return cloud.ImportConfig(path)
}
