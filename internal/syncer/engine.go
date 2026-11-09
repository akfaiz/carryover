package syncer

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/akfaiz/playrelay/internal/appdir"
	"github.com/akfaiz/playrelay/internal/cloud"
)

// ErrConflict is returned by Apply when both sides changed and no resolution was given.
var ErrConflict = errors.New("save conflict: both local and cloud saves changed")

// Device identifies this machine.
type Device struct {
	ID   string
	Name string
}

// Event reports sync progress.
type Event struct {
	GameID     string `json:"gameId"`
	Phase      string `json:"phase"` // scan, backup, upload, download, done, error
	Message    string `json:"message"`
	Bytes      int64  `json:"bytes"`
	TotalBytes int64  `json:"totalBytes"`
}

// Engine performs syncs against a cloud client.
type Engine struct {
	Client    *cloud.Client
	States    *StateStore
	Device    Device
	Retention int
	OnEvent   func(Event)
}

// Job describes the saves of one game on this device.
type Job struct {
	GameID string
	Slots  []Slot
}

// Plan is the result of comparing local and cloud.
type Plan struct {
	Job    Job       `json:"-"`
	Action Action    `json:"action"`
	Local  *Manifest `json:"-"`
	Remote *Manifest `json:"-"`
	Last   *Manifest `json:"-"`

	LocalSummary  Summary `json:"local"`
	RemoteSummary Summary `json:"remote"`
	HasRemote     bool    `json:"hasRemote"`
}

// Resolution decides what to do on conflict.
type Resolution string

const (
	ResolveAuto  Resolution = ""      // follow the plan; error on conflict
	ResolveLocal Resolution = "local" // force push (keep local)
	ResolveCloud Resolution = "cloud" // force pull (keep cloud)
)

func (e *Engine) emit(gameID, phase, msg string, p cloud.Progress) {
	if e.OnEvent != nil {
		e.OnEvent(Event{GameID: gameID, Phase: phase, Message: msg, Bytes: p.Bytes, TotalBytes: p.TotalBytes})
	}
}

func gameDir(id string) string      { return "games/" + id }
func manifestPath(id string) string { return gameDir(id) + "/manifest.json" }
func currentDir(id string) string   { return gameDir(id) + "/current" }
func backupsDir(id string) string   { return gameDir(id) + "/backups" }
func lockPath(id string) string     { return gameDir(id) + "/lock.json" }

func validateJob(job Job) error {
	if job.GameID == "" {
		return errors.New("missing game id")
	}
	if len(job.Slots) == 0 {
		return errors.New("game has no save locations configured")
	}
	for _, s := range job.Slots {
		if s.LocalPath == "" {
			return fmt.Errorf("save location %q has no path on this device", s.Key)
		}
		if !filepath.IsAbs(s.LocalPath) {
			return fmt.Errorf("save location %q must be an absolute path: %s", s.Key, s.LocalPath)
		}
	}
	return nil
}

// Plan scans local saves, fetches the cloud manifest and decides an action.
func (e *Engine) Plan(ctx context.Context, job Job) (*Plan, error) {
	if err := validateJob(job); err != nil {
		return nil, err
	}
	st, err := e.States.Load(job.GameID)
	if err != nil {
		return nil, err
	}
	e.emit(job.GameID, "scan", "Scanning local saves", cloud.Progress{})
	local, err := ScanLocal(job.Slots, st.LastSynced)
	if err != nil {
		return nil, fmt.Errorf("scan local saves: %w", err)
	}
	e.emit(job.GameID, "scan", "Checking cloud", cloud.Progress{})
	var remote Manifest
	found, err := e.Client.ReadJSON(ctx, manifestPath(job.GameID), &remote)
	if err != nil {
		return nil, fmt.Errorf("read cloud manifest: %w", err)
	}
	p := &Plan{Job: job, Local: local, Last: st.LastSynced, HasRemote: found}
	if found {
		p.Remote = &remote
	}
	p.Action = Decide(p.Local, p.Remote, p.Last)
	p.LocalSummary = local.Summarize()
	p.LocalSummary.DeviceName = e.Device.Name
	p.RemoteSummary = p.Remote.Summarize()
	return p, nil
}

// Apply executes a plan. With ResolveLocal/ResolveCloud the plan's action is overridden.
// It returns the action actually performed.
func (e *Engine) Apply(ctx context.Context, p *Plan, res Resolution) (Action, error) {
	action := p.Action
	switch res {
	case ResolveLocal:
		action = ActionPush
	case ResolveCloud:
		if p.Remote == nil {
			return ActionNone, errors.New("there are no cloud saves to restore")
		}
		action = ActionPull
	}
	st, err := e.States.Load(p.Job.GameID)
	if err != nil {
		return ActionNone, err
	}
	switch action {
	case ActionConflict:
		st.Conflict = true
		_ = e.States.Save(p.Job.GameID, st)
		return ActionConflict, ErrConflict
	case ActionNone:
		if p.Remote != nil && SameFiles(p.Local, p.Remote) {
			adopted := *p.Remote
			adopted.Files = p.Local.Files
			st.LastSynced = &adopted
		}
	case ActionPush:
		m, err := e.push(ctx, p)
		if err != nil {
			return action, err
		}
		st.LastSynced = m
	case ActionPull:
		if err := e.pull(ctx, p); err != nil {
			return action, err
		}
		st.LastSynced = p.Remote
	}
	st.Conflict = false
	st.LastError = ""
	st.LastSyncAt = time.Now().UTC()
	st.LastAction = action
	if err := e.States.Save(p.Job.GameID, st); err != nil {
		return action, err
	}
	e.emit(p.Job.GameID, "done", doneMessage(action), cloud.Progress{})
	return action, nil
}

func doneMessage(a Action) string {
	switch a {
	case ActionPush:
		return "Uploaded saves to cloud"
	case ActionPull:
		return "Downloaded saves from cloud"
	default:
		return "Saves are up to date"
	}
}

// Sync plans and applies in one step.
func (e *Engine) Sync(ctx context.Context, job Job, res Resolution) (*Plan, Action, error) {
	p, err := e.Plan(ctx, job)
	if err != nil {
		return nil, ActionNone, err
	}
	a, err := e.Apply(ctx, p, res)
	return p, a, err
}

func (e *Engine) progress(gameID, phase, msg string) func(cloud.Progress) {
	return func(pr cloud.Progress) { e.emit(gameID, phase, msg, pr) }
}

func (e *Engine) push(ctx context.Context, p *Plan) (*Manifest, error) {
	id := p.Job.GameID
	if p.Remote != nil && len(p.Remote.Files) > 0 && e.Retention > 0 {
		if err := e.backupCloud(ctx, id, p.Remote); err != nil {
			return nil, fmt.Errorf("backup cloud saves: %w", err)
		}
	}
	for _, s := range p.Job.Slots {
		msg := "Uploading " + s.Key
		if err := cloud.SyncDir(ctx, e.Client.Path(currentDir(id), s.Key), s.LocalPath, s.Filters, e.progress(id, "upload", msg)); err != nil {
			return nil, fmt.Errorf("upload %s: %w", s.Key, err)
		}
	}
	m := &Manifest{
		Revision:   uuid.NewString(),
		Device:     e.Device.ID,
		DeviceName: e.Device.Name,
		Time:       time.Now().UTC(),
		Files:      p.Local.Files,
	}
	if err := e.Client.WriteJSON(ctx, manifestPath(id), m); err != nil {
		return nil, fmt.Errorf("write cloud manifest: %w", err)
	}
	return m, nil
}

func (e *Engine) pull(ctx context.Context, p *Plan) error {
	id := p.Job.GameID
	if len(p.Local.Files) > 0 && e.Retention > 0 {
		if err := e.backupLocal(ctx, p.Job, p.Local); err != nil {
			return fmt.Errorf("backup local saves: %w", err)
		}
	}
	for _, s := range p.Job.Slots {
		if err := os.MkdirAll(s.LocalPath, 0o755); err != nil {
			return err
		}
		msg := "Downloading " + s.Key
		if err := cloud.SyncDir(ctx, s.LocalPath, e.Client.Path(currentDir(id), s.Key), s.Filters, e.progress(id, "download", msg)); err != nil {
			return fmt.Errorf("download %s: %w", s.Key, err)
		}
	}
	return nil
}

var unsafeChars = regexp.MustCompile(`[^A-Za-z0-9._-]+`)

func (e *Engine) backupName(kind string) string {
	dev := strings.Trim(unsafeChars.ReplaceAllString(e.Device.Name, "_"), "_")
	if dev == "" {
		dev = "device"
	}
	return time.Now().UTC().Format(backupTimeLayout) + "_" + dev + "_" + kind
}

func (e *Engine) backupCloud(ctx context.Context, id string, m *Manifest) error {
	name := e.backupName("cloud")
	e.emit(id, "backup", "Backing up cloud saves", cloud.Progress{})
	dst := e.Client.Path(backupsDir(id), name)
	if err := cloud.CopyDir(ctx, dst, e.Client.Path(currentDir(id)), cloud.Filters{}, e.progress(id, "backup", "Backing up cloud saves")); err != nil {
		return err
	}
	if err := e.Client.WriteJSON(ctx, backupsDir(id)+"/"+name+"/manifest.json", m); err != nil {
		return err
	}
	return e.prune(ctx, id)
}

func (e *Engine) backupLocal(ctx context.Context, job Job, m *Manifest) error {
	name := e.backupName("local")
	e.emit(job.GameID, "backup", "Backing up local saves", cloud.Progress{})
	for _, s := range job.Slots {
		dst := e.Client.Path(backupsDir(job.GameID), name, s.Key)
		if err := cloud.CopyDir(ctx, dst, s.LocalPath, s.Filters, e.progress(job.GameID, "backup", "Backing up local saves")); err != nil {
			return err
		}
	}
	bm := *m
	bm.Device, bm.DeviceName, bm.Time = e.Device.ID, e.Device.Name, time.Now().UTC()
	if err := e.Client.WriteJSON(ctx, backupsDir(job.GameID)+"/"+name+"/manifest.json", &bm); err != nil {
		return err
	}
	return e.prune(ctx, job.GameID)
}

func (e *Engine) prune(ctx context.Context, id string) error {
	names, err := e.Client.ListDirs(ctx, backupsDir(id))
	if err != nil {
		return err
	}
	sort.Strings(names) // timestamp prefix sorts chronologically
	for len(names) > e.Retention {
		if err := e.Client.Purge(ctx, backupsDir(id)+"/"+names[0]); err != nil {
			return err
		}
		names = names[1:]
	}
	return nil
}

// Backup describes a stored backup.
type Backup struct {
	Name       string    `json:"name"`
	Time       time.Time `json:"time"`
	DeviceName string    `json:"deviceName"`
	Kind       string    `json:"kind"` // "local" (taken before a download) or "cloud" (taken before an upload)
}

// ListBackups returns backups for a game, newest first.
func (e *Engine) ListBackups(ctx context.Context, gameID string) ([]Backup, error) {
	names, err := e.Client.ListDirs(ctx, backupsDir(gameID))
	if err != nil {
		return nil, err
	}
	var out []Backup
	for _, n := range slices.Backward(names) {
		b := Backup{Name: n}
		parts := strings.Split(n, "_")
		if len(parts) >= 3 {
			b.Time, _ = time.Parse(backupTimeLayout, parts[0])
			b.Kind = parts[len(parts)-1]
			b.DeviceName = strings.Join(parts[1:len(parts)-1], "_")
		}
		out = append(out, b)
	}
	return out, nil
}

// RestoreBackup replaces the local saves with a backup and uploads them as the
// new cloud version. The current local saves are backed up first.
func (e *Engine) RestoreBackup(ctx context.Context, job Job, name string) error {
	if err := validateJob(job); err != nil {
		return err
	}
	if strings.ContainsAny(name, `/\`) || name == "" || name == "." || name == ".." {
		return errors.New("invalid backup name")
	}
	st, err := e.States.Load(job.GameID)
	if err != nil {
		return err
	}
	local, err := ScanLocal(job.Slots, st.LastSynced)
	if err != nil {
		return err
	}
	if len(local.Files) > 0 && e.Retention > 0 {
		if err := e.backupLocal(ctx, job, local); err != nil {
			return fmt.Errorf("backup local saves: %w", err)
		}
	}
	for _, s := range job.Slots {
		if err := os.MkdirAll(s.LocalPath, 0o755); err != nil {
			return err
		}
		src := e.Client.Path(backupsDir(job.GameID), name, s.Key)
		if err := cloud.SyncDir(ctx, s.LocalPath, src, s.Filters, e.progress(job.GameID, "download", "Restoring "+s.Key)); err != nil {
			return fmt.Errorf("restore %s: %w", s.Key, err)
		}
	}
	_, _, err = e.Sync(ctx, job, ResolveLocal)
	return err
}

// Lock marks a game as being played on a device.
type Lock struct {
	Device     string    `json:"device"`
	DeviceName string    `json:"deviceName"`
	Since      time.Time `json:"since"`
}

// LockStaleAfter is how long a cloud lock is honoured.
const LockStaleAfter = 12 * time.Hour

// CheckLock returns the lock held by another device, if any and not stale.
func (e *Engine) CheckLock(ctx context.Context, gameID string) (*Lock, error) {
	var l Lock
	found, err := e.Client.ReadJSON(ctx, lockPath(gameID), &l)
	if err != nil || !found {
		return nil, err
	}
	if l.Device == e.Device.ID || time.Since(l.Since) > LockStaleAfter {
		return nil, nil
	}
	return &l, nil
}

// AcquireLock writes this device's lock.
func (e *Engine) AcquireLock(ctx context.Context, gameID string) error {
	return e.Client.WriteJSON(ctx, lockPath(gameID), Lock{Device: e.Device.ID, DeviceName: e.Device.Name, Since: time.Now().UTC()})
}

// ReleaseLock removes this device's lock (leaves other devices' locks alone).
func (e *Engine) ReleaseLock(ctx context.Context, gameID string) error {
	var l Lock
	found, err := e.Client.ReadJSON(ctx, lockPath(gameID), &l)
	if err != nil || !found || l.Device != e.Device.ID {
		return err
	}
	return e.Client.Delete(ctx, lockPath(gameID))
}

// State is the per-game local sync state.
type State struct {
	LastSynced *Manifest `json:"lastSynced"`
	LastSyncAt time.Time `json:"lastSyncAt"`
	LastAction Action    `json:"lastAction"`
	Conflict   bool      `json:"conflict"`
	LastError  string    `json:"lastError"`
}

// StateStore persists State per game in a directory.
type StateStore struct {
	Dir string
}

func (s *StateStore) path(id string) string { return filepath.Join(s.Dir, id+".json") }

// Load returns the state for a game (zero value if none).
func (s *StateStore) Load(id string) (State, error) {
	var st State
	_, err := appdir.ReadJSON(s.path(id), &st)
	return st, err
}

// Save writes the state for a game.
func (s *StateStore) Save(id string, st State) error {
	return appdir.WriteJSONAtomic(s.path(id), st)
}

// SetError records the last error (or clears it with "").
func (s *StateStore) SetError(id, msg string) {
	st, err := s.Load(id)
	if err != nil {
		return
	}
	st.LastError = msg
	_ = s.Save(id, st)
}

// backupTimeLayout is the timestamp prefix of backup folder names; it sorts chronologically.
const backupTimeLayout = "20060102T150405.000Z"
