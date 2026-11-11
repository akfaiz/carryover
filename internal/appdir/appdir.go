// Package appdir resolves Carryover's on-disk locations and persistent settings.
package appdir

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/google/uuid"
)

// EnvHome overrides the root directory (useful for testing and portable installs).
const EnvHome = "CARRYOVER_HOME"

// Dirs holds the paths Carryover uses.
type Dirs struct {
	Root string
}

// Resolve returns the Carryover directories, creating the root if needed.
func Resolve() (Dirs, error) {
	root := os.Getenv(EnvHome)
	if root == "" {
		base, err := os.UserConfigDir()
		if err != nil {
			return Dirs{}, fmt.Errorf("locate config dir: %w", err)
		}
		root = filepath.Join(base, "Carryover")
	}
	d := Dirs{Root: root}
	for _, p := range []string{d.Root, d.StateDir(), d.CoversDir(), d.LogsDir()} {
		if err := os.MkdirAll(p, 0o755); err != nil {
			return Dirs{}, err
		}
	}
	return d, nil
}

func (d Dirs) SettingsFile() string { return filepath.Join(d.Root, "settings.json") }
func (d Dirs) LibraryFile() string  { return filepath.Join(d.Root, "library.json") }
func (d Dirs) RcloneConfig() string { return filepath.Join(d.Root, "rclone.conf") }
func (d Dirs) StateDir() string     { return filepath.Join(d.Root, "state") }
func (d Dirs) CoversDir() string    { return filepath.Join(d.Root, "covers") }
func (d Dirs) LogsDir() string      { return filepath.Join(d.Root, "logs") }

// SafetyDir holds the copies of saves taken on this device before a download
// replaces them (created when first needed).
func (d Dirs) SafetyDir() string { return filepath.Join(d.Root, "safety") }

// Settings are per-device preferences.
type Settings struct {
	DeviceID        string `json:"deviceId"`
	DeviceName      string `json:"deviceName"`
	Remote          string `json:"remote"`          // rclone remote name, without trailing colon
	RemoteRoot      string `json:"remoteRoot"`      // folder inside the remote
	BackupRetention int    `json:"backupRetention"` // 0 = no backups (default)
	SyncLibrary     bool   `json:"syncLibrary"`     // mirror library catalog to the cloud
	Theme           string `json:"theme,omitempty"` // "dark" (default) or "light"
	// SteamGridDBKey is the user's own SteamGridDB API key (optional), used
	// to find cover art. Stored only on this device.
	SteamGridDBKey string `json:"steamGridDbKey,omitempty"`
}

// CloudBase returns the rclone path of Carryover's cloud folder, or "" if no remote is configured.
func (s Settings) CloudBase() string {
	if s.Remote == "" {
		return ""
	}
	root := s.RemoteRoot
	if root == "" {
		root = "Carryover"
	}
	return s.Remote + ":" + root
}

// SettingsStore loads and saves Settings.
type SettingsStore struct {
	mu   sync.Mutex
	path string
	s    Settings
}

// OpenSettings loads settings, filling defaults (including a new device ID) on first run.
func OpenSettings(path string) (*SettingsStore, error) {
	st := &SettingsStore{path: path}
	b, err := os.ReadFile(path)
	switch {
	case errors.Is(err, os.ErrNotExist):
	case err != nil:
		return nil, err
	default:
		if err := json.Unmarshal(b, &st.s); err != nil {
			return nil, fmt.Errorf("parse %s: %w", path, err)
		}
	}
	changed := false
	if st.s.DeviceID == "" {
		// First run. BackupRetention stays 0: backups are opt-in because the zip
		// upload after a play session slows the sync down.
		st.s.DeviceID = uuid.NewString()
		st.s.SyncLibrary = true
		changed = true
	}
	if st.s.DeviceName == "" {
		st.s.DeviceName, _ = os.Hostname()
		if st.s.DeviceName == "" {
			st.s.DeviceName = "device"
		}
		changed = true
	}
	if st.s.RemoteRoot == "" {
		st.s.RemoteRoot = "Carryover"
		changed = true
	}
	if changed {
		if err := st.save(); err != nil {
			return nil, err
		}
	}
	return st, nil
}

// Get returns a copy of the settings.
func (st *SettingsStore) Get() Settings {
	st.mu.Lock()
	defer st.mu.Unlock()
	return st.s
}

// Update replaces the settings (device ID is preserved).
func (st *SettingsStore) Update(s Settings) error {
	st.mu.Lock()
	defer st.mu.Unlock()
	s.DeviceID = st.s.DeviceID
	if s.Theme != "light" {
		s.Theme = "dark"
	}
	if s.BackupRetention < 0 {
		s.BackupRetention = 0
	}
	st.s = s
	return st.save()
}

func (st *SettingsStore) save() error {
	return WriteJSONAtomic(st.path, st.s)
}

// WriteJSONAtomic writes v as indented JSON via a temp file + rename.
func WriteJSONAtomic(path string, v any) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	// A unique temp file, so two processes writing the same file don't collide.
	f, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".*.tmp")
	if err != nil {
		return err
	}
	tmp := f.Name()
	_, err = f.Write(b)
	if err == nil {
		err = f.Sync() // flush to disk so a power loss can't leave an empty file
	}
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err == nil {
		err = os.Chmod(tmp, 0o644)
	}
	if err == nil {
		err = os.Rename(tmp, path)
	}
	if err != nil {
		_ = os.Remove(tmp)
	}
	return err
}

// ReadJSON reads JSON from path into v. Returns false if the file doesn't exist.
func ReadJSON(path string, v any) (bool, error) {
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, json.Unmarshal(b, v)
}
