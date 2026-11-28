package syncer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"time"

	"github.com/google/uuid"

	"github.com/akfaiz/carryover/internal/app/appdir"
	"github.com/akfaiz/carryover/internal/app/msg"
	"github.com/akfaiz/carryover/internal/cloudsync/cloud"
	"github.com/akfaiz/carryover/internal/cloudsync/localfs"
)

// ErrConflict is returned by Apply when both sides changed and no resolution was given.
var ErrConflict = msg.SyncConflict.Err()

// Device identifies this machine.
type Device struct {
	ID   string
	Name string
}

// Event reports sync progress.
type Event struct {
	GameID  string `json:"gameId"`
	Phase   string `json:"phase"` // scan, backup, upload, download, done, error
	Message string `json:"message"`
	// Msg is Message for the apps to translate (nil: it has no code).
	Msg        *msg.Message `json:"msg,omitempty"`
	Bytes      int64        `json:"bytes"`
	TotalBytes int64        `json:"totalBytes"`
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
	// WriteFormat is how a push stores the files (FormatRaw, FormatZstd).
	// Every format is read; a push to a folder in another format uploads all
	// of it again in this one.
	WriteFormat int
	OnEvent     func(Event)
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
	// plannedAt is when the cloud was read for this plan.
	plannedAt time.Time
}

// cloudListingMaxAge is how long a push trusts the plan's cloud listing (a
// conflict prompt can keep a plan waiting).
const cloudListingMaxAge = 2 * time.Minute

// planFreshFor is how long after planning a push goes ahead without looking
// at the cloud manifest again before it uploads. A plan that waited longer (a
// conflict prompt) may be out of date: another device may have pushed since,
// and uploading over its files would leave the cloud with a mix of both
// versions that matches no manifest.
var planFreshFor = 10 * time.Second

// errCloudChanged is returned by a push when another device synced meanwhile.
var errCloudChanged = msg.SyncCloudChanged.Err()

// Resolution decides what to do on conflict.
type Resolution string

const (
	ResolveAuto  Resolution = ""      // follow the plan; error on conflict
	ResolveLocal Resolution = "local" // force push (keep local)
	ResolveCloud Resolution = "cloud" // force pull (keep cloud)
)

func (e *Engine) emit(gameID, phase string, text msg.Message, p cloud.Progress) {
	if e.OnEvent != nil {
		e.OnEvent(Event{GameID: gameID, Phase: phase, Message: text.Text, Msg: text.Ptr(), Bytes: p.Bytes, TotalBytes: p.TotalBytes})
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
		return msg.SyncNoSlots.Err()
	}
	for _, s := range job.Slots {
		if s.LocalPath == "" {
			return msg.SyncSlotNoPath.Err("Slot", s.Key)
		}
		if !localfs.IsAbs(s.LocalPath) {
			return msg.SyncSlotNotAbsolute.Err("Slot", s.Key, "Path", s.LocalPath)
		}
	}
	return nil
}

// Plan scans local saves, fetches the cloud manifest and decides an action.
func (e *Engine) Plan(ctx context.Context, job Job) (*Plan, error) {
	p, err := e.plan(ctx, job)
	if err != nil && ctx.Err() == nil {
		log.Printf("sync %s: plan failed: %v", job.StateID(), err)
	}
	return p, err
}

func (e *Engine) plan(ctx context.Context, job Job) (*Plan, error) {
	if err := validateJob(job); err != nil {
		return nil, err
	}
	job = withResolvedSlots(job)
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
	e.emit(job.GameID, "scan", msg.SyncScanning.Msg(), cloud.Progress{})
	local, err := ScanLocal(ctx, job.Slots, st.LastSynced)
	e.emit(job.GameID, "scan", msg.SyncCheckingCloud.Msg(), cloud.Progress{})
	<-manifestDone
	<-listDone
	if err != nil {
		return nil, msg.SyncScanFailed.Wrap(err)
	}
	if manifestErr != nil {
		return nil, msg.SyncManifestRead.Wrap(manifestErr)
	}
	if found && remote.Format > maxFormat {
		return nil, ErrNewerSaveFormat
	}
	// Compare like with like: the local scan only has what the slots' filters
	// let through, so the manifests are cut down the same way.
	p := &Plan{Job: job, Local: local, Last: filterManifest(st.LastSynced, job.Slots), HasRemote: found, plannedAt: cloudListedTime}
	if listErr == nil {
		p.cloudFiles, p.cloudListedAt = listed, cloudListedTime
	}
	if found {
		p.Remote = filterManifest(&remote, job.Slots)
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
	start := time.Now()
	action, summary, err := e.apply(ctx, p, res)
	logApply(p, res, action, summary, err, ctx.Err() != nil, time.Since(start))
	return action, err
}

// logApply writes one line per sync to the log: what was decided, from what
// (file counts and sizes; no paths or device names), and how it ended.
func logApply(p *Plan, res Resolution, action Action, summary string, err error, cancelled bool, took time.Duration) {
	resolution := "auto"
	if res != ResolveAuto {
		resolution = "keep " + string(res)
	}
	l, r := p.LocalSummary, p.RemoteSummary
	cloud := "none"
	if p.HasRemote {
		cloud = fmt.Sprintf("%d files %d B", r.Files, r.Bytes)
		if r.StoredBytes != r.Bytes {
			cloud += fmt.Sprintf(" (%d B stored)", r.StoredBytes)
		}
	}
	last := "never synced"
	if p.Last != nil {
		ls := p.Last.Summarize()
		last = fmt.Sprintf("%d files %d B", ls.Files, ls.Bytes)
	}
	outcome := summary
	switch {
	case cancelled:
		outcome = "cancelled"
	case errors.Is(err, ErrConflict):
		outcome = "conflict, nothing changed"
	case err != nil:
		outcome = "failed: " + err.Error()
	}
	log.Printf("sync %s: planned %s, %s -> %s; local %d files %d B, cloud %s, last synced %s; %s (%s)",
		p.Job.StateID(), p.Action, resolution, action, l.Files, l.Bytes, cloud, last, outcome, took.Round(time.Millisecond))
}

func (e *Engine) apply(ctx context.Context, p *Plan, res Resolution) (Action, string, error) {
	action, err := p.ActionFor(res)
	if err != nil {
		return ActionNone, "", err
	}
	st, err := e.States.Load(p.Job.StateID())
	if err != nil {
		return ActionNone, "", err
	}
	done := doneMessage(action)
	switch action {
	case ActionConflict:
		st.Conflict = true
		_ = e.States.Save(p.Job.StateID(), st)
		return ActionConflict, "", ErrConflict
	case ActionNone:
		if p.Remote != nil && SameFiles(p.Local, p.Remote) {
			adopted := *p.Remote
			adopted.Files = p.Local.Files
			st.LastSynced = &adopted
		}
	case ActionPush:
		m, summary, err := e.push(ctx, p)
		if err != nil {
			return action, "", err
		}
		st.LastSynced = m
		done = summary
	case ActionPull:
		summary, err := e.pull(ctx, p)
		if err != nil {
			return action, "", err
		}
		done = summary
		st.LastSynced = p.Remote
	}
	st.Conflict = false
	st.LastError, st.LastErrorMsg = "", nil
	st.LastSyncAt = time.Now().UTC()
	st.LastAction = action
	if err := e.States.Save(p.Job.StateID(), st); err != nil {
		return action, "", err
	}
	e.emit(p.Job.GameID, "done", done, cloud.Progress{})
	return action, done.Text, nil
}

// ActionFor returns what applying the plan with res would do: the plan's own
// action, or a push/pull when the resolution forces one.
func (p *Plan) ActionFor(res Resolution) (Action, error) {
	switch res {
	case ResolveLocal:
		return ActionPush, nil
	case ResolveCloud:
		if p.Remote == nil {
			return ActionNone, msg.SyncNoCloudSaves.Err()
		}
		return ActionPull, nil
	}
	return p.Action, nil
}

func doneMessage(a Action) msg.Message {
	switch a {
	case ActionPush:
		return msg.SyncUploaded.Msg()
	case ActionPull:
		return msg.SyncDownloaded.Msg()
	default:
		return msg.SyncUpToDate.Msg()
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

func (e *Engine) progress(gameID, phase string, text msg.Message) func(cloud.Progress) {
	return func(pr cloud.Progress) { e.emit(gameID, phase, text, pr) }
}

func (e *Engine) push(ctx context.Context, p *Plan) (*Manifest, msg.Message, error) {
	id := p.Job.GameID
	if time.Since(p.plannedAt) > planFreshFor {
		if err := e.checkCloudUnchanged(ctx, p); err != nil {
			return nil, msg.Message{}, err
		}
	}
	// One listing of the cloud folder serves the delta and the backup; the
	// plan's is reused while fresh.
	listed := p.cloudFiles
	if listed == nil || time.Since(p.cloudListedAt) > cloudListingMaxAge {
		var err error
		if listed, err = e.Client.ListFiles(ctx, currentDir(id)); err != nil {
			return nil, msg.Message{}, msg.SyncListFailed.Wrap(err)
		}
	}
	format := e.WriteFormat
	// A cloud folder in another format is uploaded whole in this one: none of
	// its objects has the name a file gets now.
	base := p.Remote
	if base != nil && base.Format != format {
		base = nil
	}
	inCloud, others := cloudView(listed, format)
	// The objects of the previous format are deleted only once the new
	// manifest is written, so an interrupted push leaves the folder the old
	// manifest describes.
	previous := map[string]bool{}
	if p.Remote != nil && p.Remote.Format != format {
		for k := range p.Remote.Files {
			previous[storedName(k, p.Remote.Format)] = true
		}
	}
	deltas := make([]delta, len(p.Job.Slots))
	late := make([][]string, len(p.Job.Slots)) // object names to delete after the manifest
	changes := false
	for i, s := range p.Job.Slots {
		d, err := pushDelta(p.Local, base, s, cloudSlotFiles(inCloud, s.Key))
		if err != nil {
			return nil, msg.Message{}, msg.SyncUploadFailed.Wrap(err, "Slot", s.Key)
		}
		var now []string
		for _, rel := range d.del {
			if obj := storedName(rel, format); previous[s.Key+"/"+obj] {
				late[i] = append(late[i], obj)
			} else {
				now = append(now, rel)
			}
		}
		d.del = now
		stray, err := includedObjects(s, cloudSlotFiles(others, s.Key))
		if err != nil {
			return nil, msg.Message{}, err
		}
		late[i] = append(late[i], stray...)
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
			return nil, msg.Message{}, msg.BackupCloudFailed.Wrap(err)
		}
		defer e.pruneInBackground(ctx, id)()
	}
	var tmp string
	if changes && format == FormatZstd {
		var err error
		if tmp, err = os.MkdirTemp("", "carryover-push-"); err != nil {
			return nil, msg.Message{}, err
		}
		defer os.RemoveAll(tmp)
	}
	stored := map[string]int64{} // the uploaded files' object sizes
	var copied, deleted int
	for i, s := range p.Job.Slots {
		d := deltas[i]
		if d.empty() {
			continue
		}
		dir := currentDir(id) + "/" + s.Key
		src, names, total := s.LocalPath, d.copy, d.bytes
		if format == FormatZstd && len(d.copy) > 0 {
			e.emit(id, "upload", msg.SyncCompressing.Msg(msg.CountArg, len(d.copy), "Slot", s.Key), cloud.Progress{})
			src = filepath.Join(tmp, strconv.Itoa(i))
			var err error
			if names, total, err = compressSlot(ctx, p.Local, s, d.copy, src, stored); err != nil {
				return nil, msg.Message{}, msg.SyncCompressFailed.Wrap(err, "Slot", s.Key)
			}
		}
		text := msg.SyncUploading.Msg(msg.CountArg, len(d.copy), "Slot", s.Key)
		if err := e.Client.Upload(ctx, src, dir, names, total, e.progress(id, "upload", text)); err != nil {
			return nil, msg.Message{}, msg.SyncUploadFailed.Wrap(err, "Slot", s.Key)
		}
		if err := e.Client.DeleteFiles(ctx, dir, storedNames(d.del, format)); err != nil {
			return nil, msg.Message{}, msg.SyncCloudDeleteFailed.Wrap(err, "Slot", s.Key)
		}
		copied += len(d.copy)
		deleted += len(d.del)
	}
	// Another device may have pushed since this plan was made (a conflict
	// prompt can keep a plan waiting): don't stack our files on top of theirs.
	if err := e.checkCloudUnchanged(ctx, p); err != nil {
		return nil, msg.Message{}, err
	}
	m := &Manifest{
		Format:     format,
		Revision:   uuid.NewString(),
		Device:     e.Device.ID,
		DeviceName: e.Device.Name,
		Time:       time.Now().UTC(),
		Files:      manifestFiles(p.Local, format, stored, inCloud),
	}
	if err := e.Client.WriteJSON(ctx, manifestPath(id), m); err != nil {
		return nil, msg.Message{}, msg.SyncManifestWrite.Wrap(err)
	}
	// The push is complete: what is left of another format is only clutter,
	// and the next push tries again if deleting it fails.
	for i, s := range p.Job.Slots {
		if err := e.Client.DeleteFiles(ctx, currentDir(id)+"/"+s.Key, late[i]); err != nil {
			log.Printf("sync %s: delete files of the previous format: %v", p.Job.StateID(), err)
		}
	}
	return m, transferSummary(msg.SyncDoneUploaded, msg.SyncDeletedCloud, msg.SyncAndDeletedCloud, copied, deleted), nil
}

// compressSlot compresses the slot's files (paths relative to it) into dir as
// the objects to upload and returns their names and combined size. stored
// gets each file's object size, keyed "<slot>/<path>".
func compressSlot(ctx context.Context, local *Manifest, s Slot, files []string, dir string, stored map[string]int64) ([]string, int64, error) {
	c, err := newCompressor()
	if err != nil {
		return nil, 0, err
	}
	names := make([]string, len(files))
	var total int64
	for i, rel := range files {
		if err := ctx.Err(); err != nil {
			return nil, 0, err
		}
		key := s.Key + "/" + rel
		names[i] = storedName(rel, FormatZstd)
		n, err := c.file(localfs.Join(s.LocalPath, rel), filepath.Join(dir, filepath.FromSlash(names[i])), local.Files[key].ModTime)
		if err != nil {
			return nil, 0, fmt.Errorf("%s: %w", rel, err)
		}
		stored[key] = n
		total += n
	}
	return names, total, nil
}

// manifestFiles is the file list of a pushed manifest: the local scan, with
// each object's size in a compressed format (uploaded now, or already in the
// cloud as checked by pushDelta).
func manifestFiles(local *Manifest, format int, stored, inCloud map[string]int64) map[string]FileInfo {
	out := make(map[string]FileInfo, len(local.Files))
	for k, f := range local.Files {
		f.StoredSize = 0
		if format != FormatRaw {
			if n, ok := stored[k]; ok {
				f.StoredSize = n
			} else {
				f.StoredSize = inCloud[k]
			}
		}
		out[k] = f
	}
	return out
}

// storedNames are the object names of files in the given format.
func storedNames(files []string, format int) []string {
	out := make([]string, len(files))
	for i, f := range files {
		out[i] = storedName(f, format)
	}
	return out
}

// includedObjects returns the objects (paths relative to the slot) whose
// names the slot's filters include: excluded files are left in the cloud.
func includedObjects(s Slot, objects map[string]int64) ([]string, error) {
	if len(objects) == 0 {
		return nil, nil
	}
	flt, err := s.Filters.NewFilter()
	if err != nil {
		return nil, err
	}
	var out []string
	for obj, size := range objects {
		if flt.Include(obj, size, time.Time{}, nil) {
			out = append(out, obj)
		}
	}
	sort.Strings(out)
	return out, nil
}

// checkCloudUnchanged returns errCloudChanged if the cloud manifest is no
// longer the one the plan was made from.
func (e *Engine) checkCloudUnchanged(ctx context.Context, p *Plan) error {
	var cur Manifest
	found, err := e.Client.ReadJSON(ctx, manifestPath(p.Job.GameID), &cur)
	if err != nil {
		return msg.SyncManifestRead.Wrap(err)
	}
	if found != (p.Remote != nil) || (found && cur.Revision != p.Remote.Revision) {
		return errCloudChanged
	}
	return nil
}

func (e *Engine) pull(ctx context.Context, p *Plan) (msg.Message, error) {
	id := p.Job.GameID
	deltas := make([]delta, len(p.Job.Slots))
	changes := false
	for i, s := range p.Job.Slots {
		flt, err := s.Filters.NewFilter()
		if err != nil {
			return msg.Message{}, err
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
				return msg.Message{}, msg.BackupLocalFailed.Wrap(err)
			}
			defer e.pruneInBackground(ctx, id)()
		} else if err := e.safetyCopy(p.Job, p.Local); err != nil {
			return msg.Message{}, msg.BackupCopyFailed.Wrap(err)
		}
	}
	var copied, deleted int
	for i, s := range p.Job.Slots {
		d := deltas[i]
		if d.empty() {
			continue
		}
		if err := localfs.MkdirAll(s.LocalPath); err != nil {
			return msg.Message{}, err
		}
		var err error
		if p.Remote.Format == FormatZstd {
			err = e.downloadCompressed(ctx, p, s, d)
		} else {
			err = e.downloadRaw(ctx, p, s, d)
		}
		if err != nil {
			return msg.Message{}, err
		}
		for _, rel := range d.del {
			if err := localfs.Remove(localfs.Join(s.LocalPath, rel)); err != nil && !errors.Is(err, os.ErrNotExist) {
				return msg.Message{}, msg.SyncDeleteFailed.Wrap(err, "File", rel)
			}
		}
		copied += len(d.copy)
		deleted += len(d.del)
	}
	return transferSummary(msg.SyncDoneDownloaded, msg.SyncDeletedDevice, msg.SyncAndDeletedDevice, copied, deleted), nil
}

// errNotAsManifest is the error for a downloaded file that isn't the one the
// cloud manifest describes: the cloud folder can be ahead of its manifest (a
// push that was interrupted before writing it, or one still running).
func errNotAsManifest(rel string) error {
	return msg.SyncMismatch.Err("File", rel)
}

// downloadRaw downloads the slot's changed files of a FormatRaw folder
// straight into the save folder and checks them against the manifest.
func (e *Engine) downloadRaw(ctx context.Context, p *Plan, s Slot, d delta) error {
	id := p.Job.GameID
	text := msg.SyncDownloading.Msg(msg.CountArg, len(d.copy), "Slot", s.Key)
	if err := e.Client.Download(ctx, currentDir(id)+"/"+s.Key, s.LocalPath, d.copy, d.bytes, e.progress(id, "download", text)); err != nil {
		return msg.SyncDownloadFailed.Wrap(err, "Slot", s.Key)
	}
	// Don't record content we didn't expect as the synced state.
	for _, rel := range d.copy {
		want := p.Remote.Files[s.Key+"/"+rel]
		got, err := hashFile(localfs.Join(s.LocalPath, rel))
		if err != nil {
			return msg.SyncVerifyFailed.Wrap(err, "File", rel)
		}
		if got != want.Hash {
			return errNotAsManifest(rel)
		}
	}
	return nil
}

// downloadCompressed downloads the slot's changed files of a FormatZstd
// folder into a temporary folder and decompresses them into the save folder.
// Each file is checked before it replaces a save, so one that doesn't match
// the manifest leaves that save as it was.
func (e *Engine) downloadCompressed(ctx context.Context, p *Plan, s Slot, d delta) error {
	id := p.Job.GameID
	tmp, err := os.MkdirTemp("", "carryover-pull-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)
	names := storedNames(d.copy, FormatZstd)
	var total int64
	for _, rel := range d.copy {
		if !filepath.IsLocal(filepath.FromSlash(rel)) {
			return msg.SyncUnsafeManifest.Err("Path", rel)
		}
		total += p.Remote.Files[s.Key+"/"+rel].stored()
	}
	text := msg.SyncDownloading.Msg(msg.CountArg, len(d.copy), "Slot", s.Key)
	if err := e.Client.Download(ctx, currentDir(id)+"/"+s.Key, tmp, names, total, e.progress(id, "download", text)); err != nil {
		return msg.SyncDownloadFailed.Wrap(err, "Slot", s.Key)
	}
	for i, rel := range d.copy {
		if err := ctx.Err(); err != nil {
			return err
		}
		err := decompressTo(filepath.Join(tmp, filepath.FromSlash(names[i])), localfs.Join(s.LocalPath, rel), p.Remote.Files[s.Key+"/"+rel])
		if errors.Is(err, errNoMatch) {
			return errNotAsManifest(rel)
		}
		if err != nil {
			return msg.SyncDecompressFailed.Wrap(err, "File", rel)
		}
	}
	return nil
}

// pushDelta works out what to upload and delete for one slot (listed: the
// files in the slot's cloud folder, by path relative to it, with the size of
// their objects). It compares the local scan with the cloud manifest (nil:
// none usable), but checks the manifest against what is really in the cloud
// folder: an interrupted upload can leave files there that the manifest
// doesn't describe, and those are never trusted.
func pushDelta(localScan, remoteManifest *Manifest, s Slot, listed map[string]int64) (delta, error) {
	flt, err := s.Filters.NewFilter()
	if err != nil {
		return delta{}, err
	}
	// The cloud side is filtered like the local one: a file the filters now
	// exclude is neither uploaded nor deleted (the slot's filters may have just
	// changed), it stays in the cloud as it is.
	remote := slotFiles(remoteManifest, s.Key, flt)
	for k, f := range remote {
		if size, ok := listed[k]; !ok || size != f.stored() {
			delete(remote, k)
		}
	}
	local := slotFiles(localScan, s.Key, nil)
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

// transferSummary is the "done" message for a push or pull: copiedMsg counts
// the files transferred and deletedMsg the ones removed where they went
// (andDeleted is deletedMsg after something else).
func transferSummary(copiedMsg, deletedMsg, andDeleted msg.Def, copied, deleted int) msg.Message {
	switch {
	case copied == 0 && deleted == 0:
		return msg.SyncNothingChanged.Msg()
	case deleted == 0:
		return copiedMsg.Msg(msg.CountArg, copied)
	case copied == 0:
		return deletedMsg.Msg(msg.CountArg, deleted)
	}
	return msg.SyncDoneParts.Msg("Parts", msg.List{copiedMsg.Msg(msg.CountArg, copied), andDeleted.Msg(msg.CountArg, deleted)})
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
	// LastErrorMsg is LastError for the apps to translate (nil in older data,
	// or for an error without a code).
	LastErrorMsg *msg.Message `json:"lastErrorMsg,omitempty"`
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

// Delete forgets the state of a game (no error if there is none).
func (s *StateStore) Delete(id string) error {
	if err := os.Remove(s.path(id)); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

// SetError records the last error (or clears it with "").
func (s *StateStore) SetError(id string, failure error) {
	st, err := s.Load(id)
	if err != nil {
		return
	}
	m := msg.Of(failure)
	st.LastError, st.LastErrorMsg = m.Text, m.Ptr()
	_ = s.Save(id, st)
}
