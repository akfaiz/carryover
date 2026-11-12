package syncer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/google/uuid"

	"github.com/akfaiz/carryover/internal/app/appdir"
	"github.com/akfaiz/carryover/internal/cloudsync/cloud"
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
	// SafetyDir holds safety copies of this device's saves, taken before a
	// download replaces them while cloud backups are off ("" = none).
	SafetyDir string
	OnEvent   func(Event)
}

// Job describes the saves of one game on this device.
type Job struct {
	// GameID names the cloud folder (games/<GameID>), lock and backups.
	GameID string
	// StateKey names this device's sync state (the last synced manifest). It
	// is GameID unless several local save folders share one cloud folder (two
	// emulators sharing saves): each then remembers what it last synced.
	StateKey string
	Slots    []Slot
}

// StateID is the key of the job's local sync state.
func (j Job) StateID() string {
	if j.StateKey != "" {
		return j.StateKey
	}
	return j.GameID
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

	// cloudFiles lists current/ (keys "<slot>/<path>") as of cloudListedAt;
	// nil if listing failed. A push reuses it while fresh.
	cloudFiles    map[string]int64
	cloudListedAt time.Time
}

// cloudListingMaxAge is how long a push trusts the plan's cloud listing (a
// conflict prompt can keep a plan waiting).
const cloudListingMaxAge = 2 * time.Minute

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
	st, err := e.States.Load(job.StateID())
	if err != nil {
		return nil, err
	}
	// Read the cloud manifest and list the cloud folder while the local saves
	// are scanned: each cloud round trip can take seconds.
	var (
		remote          Manifest
		found           bool
		manifestErr     error
		listed          map[string]int64
		listErr         error
		manifestDone    = make(chan struct{})
		listDone        = make(chan struct{})
		cloudListedTime = time.Now()
	)
	go func() {
		defer close(manifestDone)
		found, manifestErr = e.Client.ReadJSON(ctx, manifestPath(job.GameID), &remote)
	}()
	go func() {
		defer close(listDone)
		listed, listErr = e.Client.ListFiles(ctx, currentDir(job.GameID))
	}()
	e.emit(job.GameID, "scan", "Scanning local saves", cloud.Progress{})
	local, err := ScanLocal(ctx, job.Slots, st.LastSynced)
	e.emit(job.GameID, "scan", "Checking cloud", cloud.Progress{})
	<-manifestDone
	<-listDone
	if err != nil {
		return nil, fmt.Errorf("scan local saves: %w", err)
	}
	if manifestErr != nil {
		return nil, fmt.Errorf("read cloud manifest: %w", manifestErr)
	}
	p := &Plan{Job: job, Local: local, Last: st.LastSynced, HasRemote: found}
	if listErr == nil {
		p.cloudFiles, p.cloudListedAt = listed, cloudListedTime
	}
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
	action, err := p.ActionFor(res)
	if err != nil {
		return ActionNone, err
	}
	st, err := e.States.Load(p.Job.StateID())
	if err != nil {
		return ActionNone, err
	}
	done := doneMessage(action)
	switch action {
	case ActionConflict:
		st.Conflict = true
		_ = e.States.Save(p.Job.StateID(), st)
		return ActionConflict, ErrConflict
	case ActionNone:
		if p.Remote != nil && SameFiles(p.Local, p.Remote) {
			adopted := *p.Remote
			adopted.Files = p.Local.Files
			st.LastSynced = &adopted
		}
	case ActionPush:
		m, summary, err := e.push(ctx, p)
		if err != nil {
			return action, err
		}
		st.LastSynced = m
		done = summary
	case ActionPull:
		summary, err := e.pull(ctx, p)
		if err != nil {
			return action, err
		}
		done = summary
		st.LastSynced = p.Remote
	}
	st.Conflict = false
	st.LastError = ""
	st.LastSyncAt = time.Now().UTC()
	st.LastAction = action
	if err := e.States.Save(p.Job.StateID(), st); err != nil {
		return action, err
	}
	e.emit(p.Job.GameID, "done", done, cloud.Progress{})
	return action, nil
}

// ActionFor returns what applying the plan with res would do: the plan's own
// action, or a push/pull when the resolution forces one.
func (p *Plan) ActionFor(res Resolution) (Action, error) {
	switch res {
	case ResolveLocal:
		return ActionPush, nil
	case ResolveCloud:
		if p.Remote == nil {
			return ActionNone, errors.New("there are no cloud saves to restore")
		}
		return ActionPull, nil
	}
	return p.Action, nil
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

func (e *Engine) push(ctx context.Context, p *Plan) (*Manifest, string, error) {
	id := p.Job.GameID
	// One listing of the cloud folder serves the delta and the backup; the
	// plan's is reused while fresh.
	listed := p.cloudFiles
	if listed == nil || time.Since(p.cloudListedAt) > cloudListingMaxAge {
		var err error
		if listed, err = e.Client.ListFiles(ctx, currentDir(id)); err != nil {
			return nil, "", fmt.Errorf("list cloud saves: %w", err)
		}
	}
	deltas := make([]delta, len(p.Job.Slots))
	changes := false
	for i, s := range p.Job.Slots {
		d, err := pushDelta(p, s, cloudSlotFiles(listed, s.Key))
		if err != nil {
			return nil, "", fmt.Errorf("upload %s: %w", s.Key, err)
		}
		deltas[i] = d
		changes = changes || !d.empty()
	}
	// Back up the cloud side only when something there will change.
	if changes && len(listed) > 0 && e.Retention > 0 {
		remote := p.Remote
		if remote == nil {
			remote = &Manifest{}
		}
		if err := e.backupCloud(ctx, id, remote, listed); err != nil {
			return nil, "", fmt.Errorf("backup cloud saves: %w", err)
		}
		defer e.pruneInBackground(ctx, id)()
	}
	var copied, deleted int
	for i, s := range p.Job.Slots {
		d := deltas[i]
		if d.empty() {
			continue
		}
		dir := currentDir(id) + "/" + s.Key
		msg := "Uploading " + filesWord(len(d.copy)) + " (" + s.Key + ")"
		if err := e.Client.Upload(ctx, s.LocalPath, dir, d.copy, d.bytes, e.progress(id, "upload", msg)); err != nil {
			return nil, "", fmt.Errorf("upload %s: %w", s.Key, err)
		}
		if err := e.Client.DeleteFiles(ctx, dir, d.del); err != nil {
			return nil, "", fmt.Errorf("delete removed %s files from cloud: %w", s.Key, err)
		}
		copied += len(d.copy)
		deleted += len(d.del)
	}
	// Another device may have pushed since this plan was made (a conflict
	// prompt can keep a plan waiting): don't stack our files on top of theirs.
	var cur Manifest
	if found, err := e.Client.ReadJSON(ctx, manifestPath(id), &cur); err != nil {
		return nil, "", fmt.Errorf("read cloud manifest: %w", err)
	} else if (found != (p.Remote != nil)) || (found && cur.Revision != p.Remote.Revision) {
		return nil, "", errors.New("the cloud saves changed while syncing (another device synced); sync again")
	}
	m := &Manifest{
		Revision:   uuid.NewString(),
		Device:     e.Device.ID,
		DeviceName: e.Device.Name,
		Time:       time.Now().UTC(),
		Files:      p.Local.Files,
	}
	if err := e.Client.WriteJSON(ctx, manifestPath(id), m); err != nil {
		return nil, "", fmt.Errorf("write cloud manifest: %w", err)
	}
	return m, transferSummary("Uploaded", "from cloud", copied, deleted), nil
}

func (e *Engine) pull(ctx context.Context, p *Plan) (string, error) {
	id := p.Job.GameID
	deltas := make([]delta, len(p.Job.Slots))
	changes := false
	for i, s := range p.Job.Slots {
		flt, err := s.Filters.NewFilter()
		if err != nil {
			return "", err
		}
		// The local scan is fresh, so both sides' checksums are exact.
		deltas[i] = diff(slotFiles(p.Remote, s.Key, flt), slotFiles(p.Local, s.Key, nil))
		changes = changes || !deltas[i].empty()
	}
	// Back up local saves only when something here will change: in the
	// cloud when backups are on, otherwise as a safety copy on this device.
	if changes && len(p.Local.Files) > 0 {
		if e.Retention > 0 {
			if _, err := e.backupLocal(ctx, p.Job, p.Local, "local"); err != nil {
				return "", fmt.Errorf("backup local saves: %w", err)
			}
			defer e.pruneInBackground(ctx, id)()
		} else if err := e.safetyCopy(p.Job, p.Local); err != nil {
			return "", fmt.Errorf("keep a copy of local saves: %w", err)
		}
	}
	var copied, deleted int
	for i, s := range p.Job.Slots {
		d := deltas[i]
		if d.empty() {
			continue
		}
		if err := os.MkdirAll(s.LocalPath, 0o755); err != nil {
			return "", err
		}
		msg := "Downloading " + filesWord(len(d.copy)) + " (" + s.Key + ")"
		if err := e.Client.Download(ctx, currentDir(id)+"/"+s.Key, s.LocalPath, d.copy, d.bytes, e.progress(id, "download", msg)); err != nil {
			return "", fmt.Errorf("download %s: %w", s.Key, err)
		}
		for _, rel := range d.del {
			if err := os.Remove(filepath.Join(s.LocalPath, filepath.FromSlash(rel))); err != nil && !errors.Is(err, os.ErrNotExist) {
				return "", fmt.Errorf("delete %s: %w", rel, err)
			}
		}
		// A cloud folder can be ahead of its manifest (a push that was
		// interrupted before writing it): don't record content we didn't
		// expect as the synced state.
		for _, rel := range d.copy {
			want := p.Remote.Files[s.Key+"/"+rel]
			got, err := hashFile(filepath.Join(s.LocalPath, filepath.FromSlash(rel)))
			if err != nil {
				return "", fmt.Errorf("verify %s: %w", rel, err)
			}
			if got != want.Hash {
				return "", fmt.Errorf("downloaded %s doesn't match the cloud manifest (another device may be uploading); try again", rel)
			}
		}
		copied += len(d.copy)
		deleted += len(d.del)
	}
	return transferSummary("Downloaded", "on this device", copied, deleted), nil
}

// pushDelta works out what to upload and delete for one slot (listed: what
// is in the slot's cloud folder, by path relative to it). It compares the
// local scan with the cloud manifest, but checks the manifest against what is
// really in the cloud folder: an interrupted upload can leave files there that
// the manifest doesn't describe, and those are never trusted.
func pushDelta(p *Plan, s Slot, listed map[string]int64) (delta, error) {
	flt, err := s.Filters.NewFilter()
	if err != nil {
		return delta{}, err
	}
	remote := slotFiles(p.Remote, s.Key, nil)
	for k, f := range remote {
		if size, ok := listed[k]; !ok || size != f.Size {
			delete(remote, k)
		}
	}
	local := slotFiles(p.Local, s.Key, nil)
	d := diff(local, remote)
	// Files in the cloud folder the manifest doesn't know about. Excluded
	// files are kept, like rclone sync does without --delete-excluded.
	for k, size := range listed {
		if _, inLocal := local[k]; inLocal {
			continue
		}
		if _, inRemote := remote[k]; inRemote {
			continue // already in d.del
		}
		if flt.Include(k, size, time.Time{}, nil) {
			d.del = append(d.del, k)
		}
	}
	sort.Strings(d.del)
	return d, nil
}

// transferSummary is the "done" message for a push or pull.
func transferSummary(verb, where string, copied, deleted int) string {
	switch {
	case copied == 0 && deleted == 0:
		return "Saves are up to date (nothing changed)"
	case deleted == 0:
		return verb + " " + filesWord(copied)
	case copied == 0:
		return "Deleted " + filesWord(deleted) + " " + where
	}
	return verb + " " + filesWord(copied) + ", deleted " + filesWord(deleted) + " " + where
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
	if _, err := appdir.ReadJSON(s.path(id), &st); err != nil {
		var syn *json.SyntaxError
		if errors.As(err, &syn) || errors.Is(err, io.ErrUnexpectedEOF) {
			// Corrupt (e.g. cut short by a power loss): treat the game as never
			// synced; the next sync recomputes everything from the manifests.
			return State{}, nil
		}
		return State{}, err
	}
	return st, nil
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
