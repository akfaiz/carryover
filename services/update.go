package services

import (
	"context"
	"errors"
	"log"
	"os"
	"sync"
	"time"

	"github.com/akfaiz/carryover/internal/app/update"
	"github.com/akfaiz/carryover/internal/app/version"
	"github.com/akfaiz/carryover/internal/core"
)

// UpdateStatus is where updating Carryover stands.
type UpdateStatus string

const (
	UpdateIdle        UpdateStatus = "idle" // not checked yet
	UpdateChecking    UpdateStatus = "checking"
	UpdateCurrent     UpdateStatus = "current" // nothing newer on the channel
	UpdateAvailable   UpdateStatus = "available"
	UpdateDownloading UpdateStatus = "downloading"
	UpdateReady       UpdateStatus = "ready"     // downloaded and verified
	UpdateInstalled   UpdateStatus = "installed" // takes effect when Carryover restarts
)

// UpdateState is what there is to know about updating.
type UpdateState struct {
	Status UpdateStatus `json:"status"`
	// Supported is false for a development build, which never updates.
	Supported bool   `json:"supported"`
	Current   string `json:"current"`
	// Latest, Prerelease, Notes and URL (its release page) describe the newer
	// version, when there is one.
	Latest     string `json:"latest,omitempty"`
	Prerelease bool   `json:"prerelease,omitempty"`
	Notes      string `json:"notes,omitempty"`
	URL        string `json:"url,omitempty"`
	// Installable is false when this install can't replace itself with Latest:
	// the user has to get it from URL.
	Installable bool  `json:"installable"`
	Done        int64 `json:"done,omitempty"` // bytes downloaded
	Total       int64 `json:"total,omitempty"`
	// File is the verified download while Status is ready.
	File string `json:"file,omitempty"`
	// Error is why the last check, download or install failed.
	Error string `json:"error,omitempty"`
}

// Newer reports whether there is a newer version to tell the user about.
func (s UpdateState) Newer() bool {
	switch s.Status {
	case UpdateAvailable, UpdateDownloading, UpdateReady:
		return true
	}
	return false
}

// UpdateOutcome is how the update started before the last restart went.
type UpdateOutcome struct {
	UpdatedTo string // the version now running
	Failed    string // the version that didn't get installed
}

var (
	// ErrUpdateBusy is returned by Install while a game is played or synced.
	ErrUpdateBusy = errors.New("a game is being played or a sync is running; install the update once it is done")
	// ErrUpdateCancelled is returned when the user cancels a download.
	ErrUpdateCancelled = errors.New("update download cancelled")
	// ErrNoUpdate is returned when there is nothing to download or install.
	ErrNoUpdate = errors.New("there is no update to install")
	// ErrUpdateRunning is returned while a check or download is already running.
	ErrUpdateRunning = errors.New("an update is already being downloaded")
)

// updateCheckEvery is how long the result of a check is reused before the
// releases are read again, unless the user asks.
const updateCheckEvery = 12 * time.Hour

// updateProgressEvery limits how often download progress is published.
const updateProgressEvery = 150 * time.Millisecond

// UpdateOptions describes the program being updated.
type UpdateOptions struct {
	Target update.Target
	// Current is the running version; "" is this build's.
	Current string
	// Dir holds the downloads; "" is the data dir's updates folder.
	Dir    string
	Client update.Client
	// Channel, if not nil, is used instead of the settings' channel.
	Channel *update.Channel
}

// UpdateService looks for newer releases of Carryover on the channel the
// settings name, downloads them and installs them.
type UpdateService struct {
	c       *core.Core
	bus     *Bus
	target  update.Target
	current string
	dir     string
	client  update.Client
	only    *update.Channel // overrides the settings' channel

	mu      sync.Mutex
	state   UpdateState
	release *update.Release // the newer release, if any
	on      update.Channel  // the channel last checked
	working bool            // a check or download is running
	cancel  context.CancelFunc
}

// NewUpdateService creates the service. bus may be nil.
func NewUpdateService(c *core.Core, bus *Bus, o UpdateOptions) *UpdateService {
	if o.Current == "" {
		o.Current = version.Get().Version
	}
	if o.Dir == "" {
		o.Dir = c.Dirs.UpdatesDir()
	}
	_, err := update.Parse(o.Current)
	return &UpdateService{
		c: c, bus: bus, target: o.Target, current: o.Current, dir: o.Dir, client: o.Client, only: o.Channel,
		state: UpdateState{Status: UpdateIdle, Supported: err == nil, Current: o.Current},
	}
}

// State returns where updating stands.
func (s *UpdateService) State() UpdateState {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.state
}

// AutoCheck reports whether to look for updates without being asked.
func (s *UpdateService) AutoCheck() bool {
	return s.State().Supported && !s.c.Settings.Get().NoUpdateCheck
}

// set changes the state and tells the listeners.
func (s *UpdateService) set(change func(st *UpdateState)) UpdateState {
	s.mu.Lock()
	change(&s.state)
	st := s.state
	s.mu.Unlock()
	s.bus.updateChanged(st)
	return st
}

// claim marks a check or download as running; false when one already is, or
// an update is installed and waits for the restart.
func (s *UpdateService) claim() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.working || s.state.Status == UpdateInstalled {
		return false
	}
	s.working = true
	return true
}

func (s *UpdateService) done() {
	s.mu.Lock()
	s.working = false
	s.cancel = nil
	s.mu.Unlock()
}

// Check looks for a newer release on the settings' channel. Without force it
// reuses a check made in the last 12 hours on the same channel. It does nothing
// for a development build, or while a download runs.
func (s *UpdateService) Check(ctx context.Context, force bool) (UpdateState, error) {
	if !s.State().Supported || !s.claim() {
		return s.State(), nil
	}
	defer s.done()
	channel := s.channel()
	// What another channel offered no longer counts, also if this check fails.
	s.mu.Lock()
	switched := s.release != nil && s.on != channel
	if switched {
		s.release = nil
	}
	s.on = channel
	s.mu.Unlock()
	if switched {
		s.set(func(st *UpdateState) {
			*st = UpdateState{Status: UpdateIdle, Supported: st.Supported, Current: st.Current}
		})
	}
	path := s.c.Dirs.UpdateStateFile()
	saved := update.LoadState(path)
	if age := time.Since(saved.LastCheck); !force && saved.Channel == channel && age >= 0 && age < updateCheckEvery {
		return s.found(saved.Latest), nil
	}
	before := s.State().Status
	s.set(func(st *UpdateState) { st.Status, st.Error = UpdateChecking, "" })
	var latest *update.Release
	rel, err := s.client.Latest(ctx, channel)
	switch {
	case errors.Is(err, update.ErrNoRelease):
	case err != nil:
		log.Printf("update check: %v", err)
		return s.set(func(st *UpdateState) { st.Status, st.Error = before, err.Error() }), err
	default:
		latest = &rel
	}
	st := s.found(latest)
	// A release is published before all its files are uploaded: without ours,
	// look again next time instead of in 12 hours.
	if latest == nil || !st.Newer() || s.hasAsset(*latest) || s.target.Method == update.MethodNone {
		saved = update.LoadState(path)
		saved.LastCheck, saved.Channel, saved.Latest = time.Now().UTC(), channel, latest
		if err := saved.Save(path); err != nil {
			log.Printf("update check: %v", err)
		}
	}
	return st, nil
}

func (s *UpdateService) channel() update.Channel {
	if s.only != nil {
		return *s.only
	}
	return update.Channel(s.c.Settings.Get().UpdateChannel)
}

func (s *UpdateService) hasAsset(r update.Release) bool {
	name, ok := s.target.AssetName(r.Version)
	if !ok {
		return false
	}
	_, ok = r.Asset(name)
	return ok
}

// found takes latest (nil: the channel has no release) as the newest release.
func (s *UpdateService) found(latest *update.Release) UpdateState {
	if latest == nil || !update.Newer(latest.Version, s.current) {
		s.mu.Lock()
		s.release = nil
		s.mu.Unlock()
		return s.set(func(st *UpdateState) {
			*st = UpdateState{Status: UpdateCurrent, Supported: st.Supported, Current: st.Current}
		})
	}
	r := *latest
	installable := s.hasAsset(r)
	s.mu.Lock()
	s.release = &r
	s.mu.Unlock()
	return s.set(func(st *UpdateState) {
		next := UpdateState{
			Status: UpdateAvailable, Supported: st.Supported, Current: st.Current,
			Latest: r.Version, Prerelease: r.Prerelease, Notes: r.Notes, URL: r.URL, Installable: installable,
		}
		if st.File != "" && st.Latest == r.Version { // already downloaded
			next.Status, next.File = UpdateReady, st.File
		}
		*st = next
	})
}

// Download fetches and verifies the newer release's file and returns its path.
// Progress is published on the bus; Cancel stops it.
func (s *UpdateService) Download(ctx context.Context) (string, error) {
	if !s.claim() {
		return "", ErrUpdateRunning
	}
	defer s.done()
	s.mu.Lock()
	rel, st := s.release, s.state
	s.mu.Unlock()
	if rel == nil {
		return "", ErrNoUpdate
	}
	if !st.Installable {
		return "", update.ErrUnsupported
	}
	if st.Status == UpdateReady {
		if _, err := os.Stat(st.File); err == nil {
			return st.File, nil
		}
	}
	name, _ := s.target.AssetName(rel.Version)
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	s.mu.Lock()
	s.cancel = cancel
	s.mu.Unlock()
	s.set(func(st *UpdateState) {
		st.Status, st.Error, st.File, st.Done, st.Total = UpdateDownloading, "", "", 0, 0
	})
	var last time.Time
	file, err := s.client.Download(ctx, *rel, name, s.dir, func(done, total int64) {
		if now := time.Now(); done == total || now.Sub(last) >= updateProgressEvery {
			last = now
			s.set(func(st *UpdateState) { st.Done, st.Total = done, total })
		}
	})
	if err != nil {
		cancelled := errors.Is(ctx.Err(), context.Canceled)
		if cancelled {
			err = ErrUpdateCancelled
		} else {
			log.Printf("update download %s: %v", rel.Version, err)
		}
		s.set(func(st *UpdateState) {
			st.Status, st.Done, st.Total = UpdateAvailable, 0, 0
			if !cancelled {
				st.Error = err.Error()
			}
		})
		return "", err
	}
	s.set(func(st *UpdateState) { st.Status, st.File = UpdateReady, file })
	return file, nil
}

// Cancel stops a running download. It returns false if none was running.
func (s *UpdateService) Cancel() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cancel == nil {
		return false
	}
	s.cancel()
	return true
}

// Busy reports whether a game is being played or synced on this device, so the
// program must not be replaced (core.Core.InUse).
func (s *UpdateService) Busy() bool { return s.c.InUse() }

// Install replaces the program with the downloaded release (desktop only; on
// Android the app hands UpdateState.File to the system installer). The caller
// then quits, calling the returned function right before: with true it starts
// the new version. It is refused while a game is played or synced.
func (s *UpdateService) Install(ctx context.Context) (func(restart bool) error, error) {
	if s.target.Method == update.MethodAPK || s.target.Method == update.MethodNone {
		return nil, update.ErrUnsupported
	}
	if !s.claim() {
		return nil, ErrUpdateRunning
	}
	defer s.done()
	st := s.State()
	if st.Status != UpdateReady {
		return nil, ErrNoUpdate
	}
	if s.Busy() {
		return nil, ErrUpdateBusy
	}
	path := s.c.Dirs.UpdateStateFile()
	installing := func(v string) {
		saved := update.LoadState(path)
		saved.Installing = v
		if err := saved.Save(path); err != nil {
			log.Printf("update: %v", err)
		}
	}
	installing(st.Latest)
	failed := func(err error) {
		installing("")
		log.Printf("update to %s: %v", st.Latest, err)
		s.set(func(u *UpdateState) { u.Status, u.Error = UpdateReady, err.Error() })
	}
	finish, err := update.Apply(ctx, s.target, st.File)
	if err != nil {
		failed(err)
		return nil, err
	}
	log.Printf("update: installing %s (%s)", st.Latest, s.target.Method)
	s.set(func(u *UpdateState) { u.Status, u.Error = UpdateInstalled, "" })
	return func(restart bool) error {
		err := finish(restart)
		// The setup program is what installs: if it didn't start, nothing did.
		if err != nil && s.target.Method == update.MethodInstaller {
			failed(err)
		}
		return err
	}, nil
}

// Startup cleans up after earlier updates and says how the last one went. Call
// it once when the program starts, only from the one that owns the data dir's
// window (a second one could be downloading).
func (s *UpdateService) Startup() UpdateOutcome {
	update.Cleanup(s.target)
	path := s.c.Dirs.UpdateStateFile()
	saved := update.LoadState(path)
	var out UpdateOutcome
	if v := saved.Installing; v != "" {
		if _, err := update.Parse(s.current); err == nil && !update.Newer(v, s.current) {
			out.UpdatedTo = s.current
		} else {
			out.Failed = v
		}
		saved.Installing = ""
		if err := saved.Save(path); err != nil {
			log.Printf("update: %v", err)
		}
	}
	update.RemoveDownloads(s.dir, s.current)
	return out
}
