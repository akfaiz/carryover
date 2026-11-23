// Package mobile is the API of the Android app, bound with gomobile
// (scripts/android-bind.sh). It wraps services the way the desktop GUI uses
// them. gomobile only passes basic types, so structured values cross the
// boundary as JSON (the same JSON the services types already define), and
// events reach the app through Host.
//
// Every call except Init may block on disk or network I/O: call them off the
// app's main thread.
package mobile

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/rclone/rclone/lib/oauthutil"

	"github.com/akfaiz/carryover/internal/app/appdir"
	"github.com/akfaiz/carryover/internal/app/applog"
	"github.com/akfaiz/carryover/internal/app/version"
	"github.com/akfaiz/carryover/internal/cloudsync/syncer"
	"github.com/akfaiz/carryover/internal/core"
	"github.com/akfaiz/carryover/services"
)

// Host is implemented by the app. Its methods are called on Go goroutines,
// not on the app's main thread.
type Host interface {
	// OpenURL opens url in the browser (the authorisation page of an OAuth
	// cloud provider; rclone waits for its redirect on 127.0.0.1).
	OpenURL(url string)
	// Event reports a change. kind is "library" (payload: game id, "" for
	// all), "sync" (a syncer.Event), "syncAll" (a services.SyncAllProgress),
	// "status" (a services.Status of a play session), "prompt" (a
	// services.Prompt to show and answer with Respond) or "promptClosed" (the
	// id of a prompt withdrawn without an answer).
	Event(kind, payload string)
}

// Event kinds passed to Host.Event.
const (
	EventLibrary      = "library"
	EventSync         = "sync"
	EventSyncAll      = "syncAll"
	EventStatus       = "status"
	EventPrompt       = "prompt"
	EventPromptClosed = "promptClosed"
)

type state struct {
	dataDir  string
	core     *core.Core
	cloud    *services.CloudService
	library  *services.LibraryService
	emus     *services.EmulatorService
	sync     *services.SyncService
	sessions *services.SessionService
}

var (
	mu      sync.Mutex
	app     *state
	logFile *applog.Log // open for the life of the process
)

// closeLog closes the log file (tests: Windows can't remove an open file).
func closeLog() {
	if logFile != nil {
		logFile.Close()
	}
}

// Init opens Carryover's data in dataDir (the app's private files folder).
// cacheDir is the app's cache folder, used for temporary files and rclone's
// cache. homeDir is what {home} expands to (the device's shared storage).
// deviceName names this device on a first start; later starts keep the name
// in the settings. deviceKey is the system's stable identifier of the device
// for this app (ANDROID_ID, "" if unknown): a first start (a new install, or
// one whose data was cleared) takes its device ID from it
// (appdir.DeviceIDFrom), so the library knows the device again instead of
// listing it once more; later starts keep the ID in the settings. Calling
// Init again with the same dataDir only replaces the host; the process can't
// switch to another dataDir (rclone's config is global).
func Init(dataDir, cacheDir, homeDir, deviceName, deviceKey string, host Host) error {
	mu.Lock()
	defer mu.Unlock()
	if host == nil {
		return errors.New("host is required")
	}
	if app != nil {
		if app.dataDir != dataDir {
			return fmt.Errorf("already opened %s", app.dataDir)
		}
		setHost(host)
		return nil
	}
	// Android gives an app process no HOME and no writable /tmp, and Go's
	// defaults for them (rclone's cache, os.TempDir) point at folders the
	// app can't write.
	env := map[string]string{
		appdir.EnvHome:    dataDir,
		"TMPDIR":          filepath.Join(cacheDir, "tmp"),
		"XDG_CACHE_HOME":  cacheDir,
		"XDG_CONFIG_HOME": filepath.Join(dataDir, "config"),
		"HOME":            homeDir,
	}
	for k, v := range env {
		if v == "" {
			continue
		}
		if err := os.Setenv(k, v); err != nil {
			return err
		}
	}
	if err := os.MkdirAll(filepath.Join(cacheDir, "tmp"), 0o700); err != nil {
		return err
	}
	_, statErr := os.Stat(filepath.Join(dataDir, "settings.json"))
	firstRun := errors.Is(statErr, os.ErrNotExist)

	c, err := core.New()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(c.Dirs.LogsDir(), 0o700); err != nil {
		return err
	}
	if logFile, err = applog.Open(c.Dirs.LogsDir(), "Carryover "+versionString()+" (Android) started"); err != nil {
		log.Printf("log file: %v", err)
	}
	if key := strings.TrimSpace(deviceKey); firstRun && key != "" {
		if err := c.Settings.SetDeviceID(appdir.DeviceIDFrom("android:" + key)); err != nil {
			return err
		}
	}
	if name := strings.TrimSpace(deviceName); firstRun && name != "" {
		st := c.Settings.Get()
		st.DeviceName = name
		if err := c.Settings.Update(st); err != nil {
			return err
		}
	}
	bus := services.NewBus()
	sessions := services.NewSessionService(c, bus) // also routes core sync progress to the bus
	// A game counts as running while its session is open: the app starts
	// the game itself, so no process holds it in between (core.StartSession).
	running := func(key string) bool { return sessions.IsRunning(key) || c.SessionOpen(c.CloudKeyOf(key)) }
	app = &state{
		dataDir:  dataDir,
		core:     c,
		cloud:    services.NewCloudService(c),
		library:  services.NewLibraryService(c, bus, running),
		emus:     services.NewEmulatorService(c, bus),
		sync:     services.NewSyncService(c, bus),
		sessions: sessions,
	}
	setHost(host)
	bus.OnLibrary(func(id string) { emit(EventLibrary, id) })
	bus.OnSync(func(ev syncer.Event) { emitJSON(EventSync, ev) })
	bus.OnSyncAll(func(p services.SyncAllProgress) { emitJSON(EventSyncAll, p) })
	bus.OnStatus(func(st services.Status) { emitJSON(EventStatus, st) })
	bus.OnPrompt(func(p services.Prompt) { emitJSON(EventPrompt, p) })
	bus.OnPromptClosed(func(id string) { emit(EventPromptClosed, id) })
	return nil
}

var (
	hostMu  sync.Mutex
	curHost Host
)

func setHost(h Host) {
	hostMu.Lock()
	curHost = h
	hostMu.Unlock()
	oauthutil.OpenURL = func(url string) error {
		currentHost().OpenURL(url)
		return nil
	}
}

func currentHost() Host {
	hostMu.Lock()
	defer hostMu.Unlock()
	return curHost
}

func emit(kind, payload string) {
	if h := currentHost(); h != nil {
		h.Event(kind, payload)
	}
}

func emitJSON(kind string, v any) {
	b, err := json.Marshal(v)
	if err != nil {
		log.Printf("event %s: %v", kind, err)
		return
	}
	emit(kind, string(b))
}

func get() (*state, error) {
	mu.Lock()
	defer mu.Unlock()
	if app == nil {
		return nil, errors.New("not initialised: call Init first")
	}
	return app, nil
}

func encode(v any, err error) (string, error) {
	if err != nil {
		return "", err
	}
	b, err := json.Marshal(v)
	return string(b), err
}

func versionString() string {
	v := version.Get()
	if v.Commit == "" {
		return v.Version
	}
	return v.Version + " (" + v.Commit + ")"
}

// Version returns the version of the Go core, such as "0.9.0 (abc1234)".
func Version() string { return versionString() }

// LogFile returns the path of the log file.
func LogFile() (string, error) {
	s, err := get()
	if err != nil {
		return "", err
	}
	return filepath.Join(s.core.Dirs.LogsDir(), applog.FileName), nil
}

// WaitBackground returns once background uploads of the library catalog have
// finished; call it before letting the system stop the process.
func WaitBackground() { services.WaitBackground() }

// Settings returns this device's settings (appdir.Settings) as JSON.
func Settings() (string, error) {
	s, err := get()
	if err != nil {
		return "", err
	}
	return encode(s.sync.GetSettings(), nil)
}

// SaveSettings stores settings given as JSON and returns them as stored. The
// device id can't be changed.
func SaveSettings(settingsJSON string) (string, error) {
	s, err := get()
	if err != nil {
		return "", err
	}
	var st appdir.Settings
	if err := json.Unmarshal([]byte(settingsJSON), &st); err != nil {
		return "", err
	}
	return encode(s.sync.SaveSettings(st))
}
