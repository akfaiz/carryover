package services

import (
	"context"
	"errors"
	"sync"

	"github.com/akfaiz/carryover/internal/appdir"
	"github.com/akfaiz/carryover/internal/core"
	"github.com/akfaiz/carryover/internal/syncer"
)

// ErrSyncCancelled is returned when the user cancels a sync or restore.
var ErrSyncCancelled = errors.New("sync cancelled")

// SyncService runs manual syncs, manages backups and settings.
type SyncService struct {
	c    *core.Core
	bus  *Bus
	busy sync.Map // gameID -> struct{}

	mu      sync.Mutex
	cancels map[string]context.CancelFunc // gameID ("" = SyncAll) -> cancel
}

// start registers a cancelable operation for key ("" for SyncAll).
func (s *SyncService) start(parent context.Context, key string) (context.Context, func()) {
	ctx, cancel := context.WithCancel(parent)
	s.mu.Lock()
	if s.cancels == nil {
		s.cancels = map[string]context.CancelFunc{}
	}
	s.cancels[key] = cancel
	s.mu.Unlock()
	return ctx, func() {
		s.mu.Lock()
		delete(s.cancels, key)
		s.mu.Unlock()
		cancel()
	}
}

// Cancel stops a running sync or restore of a game; "" stops SyncAll and
// every other running sync. Files already transferred stay; the next sync
// picks up where it left off. Returns false if nothing was running.
func (s *SyncService) Cancel(gameID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	found := false
	for k, cancel := range s.cancels {
		if gameID == "" || k == gameID {
			cancel()
			found = true
		}
	}
	return found
}

// cancelled maps a context cancellation to ErrSyncCancelled.
func cancelled(ctx context.Context, err error) error {
	if err != nil && ctx.Err() != nil {
		return ErrSyncCancelled
	}
	return err
}

// NewSyncService creates the service. bus may be nil.
func NewSyncService(c *core.Core, bus *Bus) *SyncService { return &SyncService{c: c, bus: bus} }

// GetSettings returns device settings.
func (s *SyncService) GetSettings() appdir.Settings { return s.c.Settings.Get() }

// SaveSettings updates device settings.
func (s *SyncService) SaveSettings(st appdir.Settings) (appdir.Settings, error) {
	if err := s.c.Settings.Update(st); err != nil {
		return appdir.Settings{}, err
	}
	s.bus.libraryChanged("")
	return s.c.Settings.Get(), nil
}

// PlanView summarises what a sync would do.
type PlanView struct {
	Action    string         `json:"action"`
	Local     syncer.Summary `json:"local"`
	Remote    syncer.Summary `json:"remote"`
	HasRemote bool           `json:"hasRemote"`
}

func (s *SyncService) prepare(gameID string) (*syncer.Engine, syncer.Job, error) {
	g, err := s.c.Library.Get(gameID)
	if err != nil {
		return nil, syncer.Job{}, err
	}
	job, err := s.c.Job(g)
	if err != nil {
		return nil, syncer.Job{}, err
	}
	eng, err := s.c.Engine()
	return eng, job, err
}

// Plan compares local and cloud saves without transferring anything.
func (s *SyncService) Plan(gameID string) (PlanView, error) {
	eng, job, err := s.prepare(gameID)
	if err != nil {
		return PlanView{}, err
	}
	p, err := eng.Plan(context.Background(), job)
	if err != nil {
		return PlanView{}, err
	}
	return PlanView{Action: string(p.Action), Local: p.LocalSummary, Remote: p.RemoteSummary, HasRemote: p.HasRemote}, nil
}

// Sync syncs one game. resolution: "" (auto), "local" or "cloud".
// Returns the action performed; "conflict" means nothing was transferred.
func (s *SyncService) Sync(gameID, resolution string) (string, error) {
	return s.sync(context.Background(), gameID, resolution)
}

func (s *SyncService) sync(parent context.Context, gameID, resolution string) (string, error) {
	if _, loaded := s.busy.LoadOrStore(gameID, struct{}{}); loaded {
		return "", errors.New("a sync for this game is already running")
	}
	defer s.busy.Delete(gameID)
	lock, err := syncer.AcquireLocalLock(s.c.Dirs.StateDir(), gameID)
	if err != nil {
		return "", err
	}
	defer lock.Release()
	eng, job, err := s.prepare(gameID)
	if err != nil {
		return "", err
	}
	ctx, done := s.start(parent, gameID)
	defer done()
	_, action, err := eng.Sync(ctx, job, syncer.Resolution(resolution))
	defer s.bus.libraryChanged(gameID)
	if errors.Is(err, syncer.ErrConflict) {
		return string(syncer.ActionConflict), nil
	}
	if err = cancelled(ctx, err); errors.Is(err, ErrSyncCancelled) {
		return string(action), err // not a sync error: nothing to flag
	}
	if err != nil {
		s.c.States.SetError(gameID, err.Error())
		return string(action), err
	}
	return string(action), nil
}

// SyncResult is the outcome of one game in SyncAll.
type SyncResult struct {
	GameID string `json:"gameId"`
	Name   string `json:"name"`
	Action string `json:"action"`
	Error  string `json:"error"`
}

// SyncAll syncs the library catalog and every ready game. Cancel("") stops
// it; the games synced so far are returned along with ErrSyncCancelled.
func (s *SyncService) SyncAll() ([]SyncResult, error) {
	if !s.c.CloudConfigured() {
		return nil, errors.New("no cloud remote configured")
	}
	ctx, done := s.start(context.Background(), "")
	defer done()
	if err := s.c.SyncLibrary(ctx); err != nil {
		return nil, cancelled(ctx, err)
	}
	s.bus.libraryChanged("")
	var out []SyncResult
	for _, g := range s.c.Library.List() {
		if _, err := s.c.Job(g); err != nil {
			continue // not set up on this device
		}
		if ctx.Err() != nil {
			return out, ErrSyncCancelled
		}
		r := SyncResult{GameID: g.ID, Name: g.Name}
		a, err := s.sync(ctx, g.ID, "")
		if errors.Is(err, ErrSyncCancelled) {
			return out, err
		}
		r.Action = a
		if err != nil {
			r.Error = err.Error()
		}
		out = append(out, r)
	}
	return out, nil
}

// SyncLibrary merges the cloud library catalog.
func (s *SyncService) SyncLibrary() error {
	err := s.c.SyncLibrary(context.Background())
	s.bus.libraryChanged("")
	return err
}

// Backups lists a game's backups.
func (s *SyncService) Backups(gameID string) ([]syncer.Backup, error) {
	eng, err := s.c.Engine()
	if err != nil {
		return nil, err
	}
	b, err := eng.ListBackups(context.Background(), gameID)
	if b == nil {
		b = []syncer.Backup{}
	}
	return b, err
}

// BackupNow stores a pinned backup of this device's current saves, whether or
// not automatic backups are on.
func (s *SyncService) BackupNow(gameID string) error {
	if _, loaded := s.busy.LoadOrStore(gameID, struct{}{}); loaded {
		return errors.New("a sync for this game is already running")
	}
	defer s.busy.Delete(gameID)
	lock, err := syncer.AcquireLocalLock(s.c.Dirs.StateDir(), gameID)
	if err != nil {
		return err
	}
	defer lock.Release()
	eng, job, err := s.prepare(gameID)
	if err != nil {
		return err
	}
	ctx, done := s.start(context.Background(), gameID)
	defer done()
	_, err = eng.BackupNow(ctx, job)
	return cancelled(ctx, err)
}

// SetBackupPinned pins or unpins a backup; pinned backups are never pruned.
func (s *SyncService) SetBackupPinned(gameID, name string, pinned bool) error {
	eng, err := s.c.Engine()
	if err != nil {
		return err
	}
	return eng.SetBackupPinned(context.Background(), gameID, name, pinned)
}

// DeleteBackup permanently deletes a backup, for every device.
func (s *SyncService) DeleteBackup(gameID, name string) error {
	if _, loaded := s.busy.LoadOrStore(gameID, struct{}{}); loaded {
		return errors.New("a sync for this game is already running")
	}
	defer s.busy.Delete(gameID)
	eng, err := s.c.Engine()
	if err != nil {
		return err
	}
	return eng.DeleteBackup(context.Background(), gameID, name)
}

// RestoreBackup restores a backup locally and uploads it as the current cloud version.
func (s *SyncService) RestoreBackup(gameID, name string) error {
	if _, loaded := s.busy.LoadOrStore(gameID, struct{}{}); loaded {
		return errors.New("a sync for this game is already running")
	}
	defer s.busy.Delete(gameID)
	lock, err := syncer.AcquireLocalLock(s.c.Dirs.StateDir(), gameID)
	if err != nil {
		return err
	}
	defer lock.Release()
	eng, job, err := s.prepare(gameID)
	if err != nil {
		return err
	}
	defer s.bus.libraryChanged(gameID)
	ctx, done := s.start(context.Background(), gameID)
	defer done()
	return cancelled(ctx, eng.RestoreBackup(ctx, job, name))
}

// RecoverSessions releases the cloud locks left by play sessions that were
// killed before they could release them (see core.Core.RecoverSessions).
func (s *SyncService) RecoverSessions() error {
	_, err := s.c.RecoverSessions(context.Background())
	return err
}
