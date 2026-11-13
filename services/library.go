package services

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"maps"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"slices"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/akfaiz/carryover/internal/cloudsync/syncer"
	"github.com/akfaiz/carryover/internal/core"
	"github.com/akfaiz/carryover/internal/library"
	"github.com/akfaiz/carryover/internal/saves/ludusavi"
	"github.com/akfaiz/carryover/internal/saves/pathvar"
	"github.com/akfaiz/carryover/internal/stores/heroic"
	"github.com/akfaiz/carryover/internal/stores/steam"
	"github.com/akfaiz/carryover/internal/stores/steamgrid"
	"github.com/akfaiz/carryover/internal/stores/steamstore"
	"github.com/akfaiz/carryover/internal/stores/wineprefix"
)

// LibraryService manages games.
type LibraryService struct {
	c       *core.Core
	bus     *Bus
	running func(string) bool

	checkMu  sync.Mutex
	checks   map[string]localCheck // gameID -> last CheckLocalChanges result
	checking atomic.Bool

	// prefixes lists other launchers' Wine prefixes for save suggestions
	// (nil: none, i.e. not Linux).
	prefixes func() []wineprefix.Prefix
}

// localCheck is the last result of CheckLocalChanges for a game.
type localCheck struct {
	changed bool
	at      time.Time        // when the check started
	scan    *syncer.Manifest // reused by the next check to skip re-hashing
}

// NewLibraryService creates the service. bus and running may be nil.
func NewLibraryService(c *core.Core, bus *Bus, running func(string) bool) *LibraryService {
	s := &LibraryService{c: c, bus: bus, running: running, checks: map[string]localCheck{}}
	if runtime.GOOS == "linux" {
		s.prefixes = func() []wineprefix.Prefix { return wineprefix.Find(wineprefix.DefaultRoots()) }
	}
	return s
}

// GameStatus is the sync state of a game on this device.
type GameStatus struct {
	LastSyncAt string `json:"lastSyncAt"`
	LastAction string `json:"lastAction"`
	Conflict   bool   `json:"conflict"`
	LastError  string `json:"lastError"`
	Running    bool   `json:"running"`
	// LocalChanges: this device's saves changed since its last sync and
	// haven't been uploaded (e.g. the game was played without Carryover).
	LocalChanges bool `json:"localChanges"`
}

// GameView is a game as presented to the UI, from this device's perspective.
type GameView struct {
	ID            string               `json:"id"`
	Name          string               `json:"name"`
	CoverPath     string               `json:"coverPath"`    // absolute file, "" if none
	CoverVersion  int64                `json:"coverVersion"` // cover file mtime (image cache key)
	StoreAppID    string               `json:"storeAppId"`
	Art           map[string]ArtFile   `json:"art"` // extra Steam artwork by kind (wide, hero, logo, icon)
	Slots         []library.SaveSlot   `json:"slots"`
	Device        library.DeviceConfig `json:"device"`
	HasDevice     bool                 `json:"hasDevice"`
	Ready         bool                 `json:"ready"`     // all save paths + exe set on this device
	SetupHint     string               `json:"setupHint"` // what's missing
	ResolvedPaths map[string]string    `json:"resolvedPaths"`
	PathErrors    map[string]string    `json:"pathErrors"`
	OtherDevices  []DevicePaths        `json:"otherDevices"`
	Status        GameStatus           `json:"status"`
	// SafetyDir is the folder of the game's safety copies on this device
	// ("" if there are none).
	SafetyDir string `json:"safetyDir"`
	// Emulators are the game's emulator profiles; each has its own save slots,
	// cloud save and status. LastEmulator is the profile played last here.
	Emulators    []ProfileView `json:"emulators"`
	LastEmulator string        `json:"lastEmulator"`
	// Overall is the sync status of the game as a whole on this device: the
	// game's own saves and every emulator set up for it combined (see
	// combineStatus). Status alone is only the game's own saves. The library
	// list shows Overall.
	Overall GameStatus `json:"overall"`
}

// ProfileView is one emulator profile of a game, from this device's perspective.
type ProfileView struct {
	EmulatorID    string             `json:"emulatorId"`
	Name          string             `json:"name"`
	Key           string             `json:"key"`       // core.SyncKey: what sync, backup and status calls take
	ShareWith     string             `json:"shareWith"` // emulator whose cloud save this one uses ("" = its own)
	Slots         []library.SaveSlot `json:"slots"`
	SetUp         bool               `json:"setUp"` // configured for this game on this device
	Rom           string             `json:"rom"`
	Paths         map[string]string  `json:"paths"`
	ResolvedPaths map[string]string  `json:"resolvedPaths"`
	PathErrors    map[string]string  `json:"pathErrors"`
	Ready         bool               `json:"ready"`
	SetupHint     string             `json:"setupHint"`
	SteamAppID    string             `json:"steamAppId"`
	Status        GameStatus         `json:"status"`
	SafetyDir     string             `json:"safetyDir"`
}

// ArtFile is a stored artwork image.
type ArtFile struct {
	Path    string `json:"path"`
	Version int64  `json:"version"` // file mtime (image cache key)
}

// DevicePaths shows another device's config for reference.
type DevicePaths struct {
	DeviceName string            `json:"deviceName"`
	Paths      map[string]string `json:"paths"`
	Exe        string            `json:"exe"`
}

func (s *LibraryService) view(g library.Game) GameView {
	devID := s.c.DeviceID()
	d, has := g.Device(devID)
	v := GameView{
		ID: g.ID, Name: g.Name, StoreAppID: g.StoreAppID, Slots: g.Slots, Device: d, HasDevice: has,
		ResolvedPaths: map[string]string{}, PathErrors: map[string]string{},
		Art: map[string]ArtFile{},
	}
	for kind, name := range g.Art {
		p := filepath.Join(s.c.Dirs.CoversDir(), name)
		if fi, err := os.Stat(p); err == nil {
			v.Art[kind] = ArtFile{Path: p, Version: fi.ModTime().UnixNano()}
		}
	}
	if v.Slots == nil {
		v.Slots = []library.SaveSlot{}
	}
	if d.Paths == nil {
		v.Device.Paths = map[string]string{}
	}
	if g.Cover != "" {
		p := filepath.Join(s.c.Dirs.CoversDir(), g.Cover)
		if fi, err := os.Stat(p); err == nil {
			v.CoverPath, v.CoverVersion = p, fi.ModTime().UnixNano()
		}
	}
	pctx := s.c.PathContext(g)
	var missing []string
	for _, sl := range g.Slots {
		raw := d.Paths[sl.Key]
		if raw == "" {
			missing = append(missing, "save path for "+labelOf(sl))
			continue
		}
		if p, err := pctx.Expand(raw); err != nil {
			v.PathErrors[sl.Key] = err.Error()
		} else {
			v.ResolvedPaths[sl.Key] = p
		}
	}
	if d.Exe == "" && d.LaunchMode != library.LaunchSteamWrap && len(g.Emulators) == 0 {
		missing = append(missing, "game executable") // an emulator game needs none of its own
	}
	v.Ready = len(missing) == 0 && len(v.PathErrors) == 0
	if len(missing) > 0 {
		v.SetupHint = "Missing " + strings.Join(missing, ", ")
	}
	for id, od := range g.Devices {
		if id == devID {
			continue
		}
		name := od.DeviceName
		if name == "" {
			name = id[:8]
		}
		v.OtherDevices = append(v.OtherDevices, DevicePaths{DeviceName: name, Paths: od.Paths, Exe: od.Exe})
	}
	v.Status, v.SafetyDir = s.statusOf(g.ID)
	v.LastEmulator = d.LastEmulator
	v.Emulators = s.profiles(g, d)
	v.Overall = overallStatus(g, v)
	if len(g.Emulators) > 0 && len(g.Slots) == 0 {
		// A game played only through emulators is ready when one of them is.
		v.Ready, v.SetupHint = false, ""
		for _, pv := range v.Emulators {
			if pv.Ready {
				v.Ready = true
			}
			if v.SetupHint == "" {
				v.SetupHint = pv.SetupHint
			}
		}
		if v.Ready {
			v.SetupHint = ""
		}
	}
	return v
}

// overallStatus combines the status of everything that syncs for the game on
// this device: its own saves (if it has any) and each emulator set up for it.
func overallStatus(g library.Game, v GameView) GameStatus {
	var all []GameStatus
	if len(g.Slots) > 0 || len(g.Emulators) == 0 {
		all = append(all, v.Status)
	}
	for _, pv := range v.Emulators {
		if pv.SetUp {
			all = append(all, pv.Status)
		}
	}
	return combineStatus(all, v.Status)
}

// combineStatus merges profile statuses: running, conflict, error and
// unuploaded changes if any profile has them; "synced" only when every profile
// has synced, with the oldest of their sync times. none is used if there is
// nothing to combine.
func combineStatus(all []GameStatus, none GameStatus) GameStatus {
	switch len(all) {
	case 0:
		return none
	case 1:
		return all[0]
	}
	var out GameStatus
	var oldest time.Time
	allSynced := true
	for _, s := range all {
		out.Running = out.Running || s.Running
		out.Conflict = out.Conflict || s.Conflict
		out.LocalChanges = out.LocalChanges || s.LocalChanges
		if out.LastError == "" {
			out.LastError = s.LastError
		}
		if out.LastAction == "" {
			out.LastAction = s.LastAction
		}
		t, err := time.Parse(time.RFC3339, s.LastSyncAt)
		if s.LastSyncAt == "" || err != nil {
			allSynced = false
			continue
		}
		if oldest.IsZero() || t.Before(oldest) {
			oldest = t
		}
	}
	if allSynced {
		out.LastSyncAt = fmtTime(oldest)
	}
	return out
}

// statusOf is the sync status and safety-copy folder of a sync key.
func (s *LibraryService) statusOf(key string) (GameStatus, string) {
	var gs GameStatus
	if st, err := s.c.States.Load(key); err == nil {
		gs = GameStatus{
			LastSyncAt: fmtTime(st.LastSyncAt), LastAction: string(st.LastAction),
			Conflict: st.Conflict, LastError: st.LastError,
			LocalChanges: !st.Conflict && s.localChanged(key, st),
		}
	}
	if s.running != nil {
		gs.Running = s.running(key)
	}
	dir := ""
	if root := s.c.Dirs.SafetyDir(); syncer.HasSafetyCopies(root, s.c.CloudKeyOf(key)) {
		dir = syncer.SafetyGameDir(root, s.c.CloudKeyOf(key))
	}
	return gs, dir
}

// profiles builds the emulator profiles of a game.
func (s *LibraryService) profiles(g library.Game, d library.DeviceConfig) []ProfileView {
	out := make([]ProfileView, 0, len(g.Emulators))
	for _, ge := range g.Emulators {
		pv := ProfileView{
			EmulatorID: ge.EmulatorID, Name: s.c.ProfileName(ge.EmulatorID), Key: core.SyncKey(g.ID, ge.EmulatorID), ShareWith: ge.ShareWith,
			Slots: ge.Slots, Paths: map[string]string{}, ResolvedPaths: map[string]string{}, PathErrors: map[string]string{},
		}
		if pv.Slots == nil {
			pv.Slots = []library.SaveSlot{}
		}
		ec, set := d.Emulators[ge.EmulatorID]
		pv.SetUp, pv.Rom, pv.SteamAppID = set, ec.Rom, ec.SteamAppID
		maps.Copy(pv.Paths, ec.Paths)
		var missing []string
		if set {
			pctx := s.c.PathContextFor(g, ge.EmulatorID)
			for _, sl := range ge.Slots {
				raw := ec.Paths[sl.Key]
				if raw == "" {
					missing = append(missing, "save path for "+labelOf(sl))
				} else if p, err := pctx.Expand(raw); err != nil {
					pv.PathErrors[sl.Key] = err.Error()
				} else {
					pv.ResolvedPaths[sl.Key] = p
				}
			}
		} else {
			missing = append(missing, "setup on this device")
		}
		emu, err := s.c.Library.Emulator(ge.EmulatorID)
		if err != nil {
			missing = append(missing, "emulator (deleted)")
		} else if ed := emu.Devices[s.c.DeviceID()]; ed.Exe == "" {
			missing = append(missing, "emulator executable on this device")
		}
		pv.Ready = len(missing) == 0 && len(pv.PathErrors) == 0
		if len(missing) > 0 {
			pv.SetupHint = "Missing " + strings.Join(missing, ", ")
		}
		pv.Status, pv.SafetyDir = s.statusOf(pv.Key)
		out = append(out, pv)
	}
	return out
}

// localChanged reports the last check's result for a game, unless a sync has
// finished since that check started (the sync settled it).
func (s *LibraryService) localChanged(id string, st syncer.State) bool {
	s.checkMu.Lock()
	c := s.checks[id]
	s.checkMu.Unlock()
	return c.changed && !st.LastSyncAt.After(c.at)
}

// CheckLocalChanges scans the saves of every game set up on this device,
// without touching the cloud, and flags the ones changed since their last
// sync (GameStatus.LocalChanges). Games being played are skipped. Library
// subscribers are notified when a flag changes. A call while a check is
// running returns at once.
func (s *LibraryService) CheckLocalChanges() {
	if !s.checking.CompareAndSwap(false, true) {
		return
	}
	defer s.checking.Store(false)
	notify := false
	for _, g := range s.c.Library.List() {
		for _, emuID := range s.c.Profiles(g) {
			key := core.SyncKey(g.ID, emuID)
			if s.running != nil && s.running(key) {
				continue
			}
			job, err := s.c.JobFor(g, emuID)
			if err != nil {
				continue // not set up on this device
			}
			st, err := s.c.States.Load(key)
			if err != nil {
				continue
			}
			before := s.localChanged(key, st)
			s.checkMu.Lock()
			prev := s.checks[key].scan
			s.checkMu.Unlock()
			at := time.Now()
			changed, scan, err := syncer.LocalChanges(context.Background(), job.Slots, st.LastSynced, prev)
			if err != nil {
				continue // e.g. an unreadable folder: the next sync reports it
			}
			s.checkMu.Lock()
			s.checks[key] = localCheck{changed: changed, at: at, scan: scan}
			s.checkMu.Unlock()
			if changed != before {
				notify = true
			}
		}
	}
	if notify {
		s.bus.libraryChanged("")
	}
}

func labelOf(sl library.SaveSlot) string {
	if sl.Label != "" {
		return sl.Label
	}
	return sl.Key
}

// List returns all games.
func (s *LibraryService) List() []GameView {
	games := s.c.Library.List()
	out := make([]GameView, 0, len(games))
	for _, g := range games {
		out = append(out, s.view(g))
	}
	return out
}

// Get returns one game.
func (s *LibraryService) Get(id string) (GameView, error) {
	g, err := s.c.Library.Get(id)
	if err != nil {
		return GameView{}, err
	}
	return s.view(g), nil
}

// GameInput is the editable part of a game: shared fields plus this device's config.
type GameInput struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	// StoreAppID optionally links the game to its Steam store entry.
	StoreAppID string             `json:"storeAppId"`
	Slots      []library.SaveSlot `json:"slots"`
	Device     DeviceInput        `json:"device"`
	// Emulators are the emulators the game is set up with: their own save
	// slots, and this device's ROM and save paths.
	Emulators []GameEmulatorInput `json:"emulators"`
}

// GameEmulatorInput is the editor's view of one emulator profile of a game.
// A profile with neither a ROM nor a path is kept for the game (other devices
// may use it) but isn't set up on this device.
type GameEmulatorInput struct {
	EmulatorID string             `json:"emulatorId"`
	Slots      []library.SaveSlot `json:"slots"`
	Rom        string             `json:"rom"`
	Paths      map[string]string  `json:"paths"`
	// ShareWith: another emulator of the game whose cloud save this one uses.
	// Its save locations (and so the cloud layout) are that emulator's.
	ShareWith string `json:"shareWith"`
}

// DeviceInput is the editor's view of this device's config. Server-managed
// fields (timestamps, device name, Steam app id) are deliberately absent so
// the editor can't send invalid values for them.
type DeviceInput struct {
	Paths       map[string]string  `json:"paths"`
	Exe         string             `json:"exe"`
	Args        string             `json:"args"`
	WorkDir     string             `json:"workDir"`
	Env         map[string]string  `json:"env"`
	WaitProcess string             `json:"waitProcess"`
	LaunchMode  library.LaunchMode `json:"launchMode"`
}

// Save creates or updates a game.
func (s *LibraryService) Save(in GameInput) (GameView, error) {
	devID := s.c.DeviceID()
	in.StoreAppID = strings.TrimSpace(in.StoreAppID)
	if in.StoreAppID != "" && !steamstore.ValidAppID(in.StoreAppID) {
		return GameView{}, fmt.Errorf("invalid Steam app id %q", in.StoreAppID)
	}
	g := library.Game{ID: in.ID, Name: strings.TrimSpace(in.Name), StoreAppID: in.StoreAppID, Slots: in.Slots}
	d := library.DeviceConfig{
		Paths: in.Device.Paths, Exe: strings.TrimSpace(in.Device.Exe), Args: in.Device.Args,
		WorkDir: strings.TrimSpace(in.Device.WorkDir), Env: in.Device.Env,
		WaitProcess: strings.TrimSpace(in.Device.WaitProcess), LaunchMode: in.Device.LaunchMode,
	}
	var prev library.DeviceConfig // this device's stored config
	var oldGame library.Game
	if in.ID != "" {
		old, err := s.c.Library.Get(in.ID)
		if err != nil {
			return GameView{}, err
		}
		oldGame = old
		prev = old.Devices[devID]
		g.Cover = old.Cover
		g.Art = old.Art
		g.Devices = old.Devices
		// Steam app id is managed by AddToSteam, not the editor.
		d.SteamAppID = old.Devices[devID].SteamAppID
	}
	cleanSlots(g.Slots)
	keep := map[string]library.EmulatorConfig{}
	for _, ei := range in.Emulators {
		if _, err := s.c.Library.Emulator(ei.EmulatorID); err != nil {
			return GameView{}, fmt.Errorf("unknown emulator %q", ei.EmulatorID)
		}
		cleanSlots(ei.Slots)
		slots := ei.Slots
		if ei.ShareWith != "" {
			// A sharing emulator uses the other one's cloud layout.
			slots = nil
			for _, o := range in.Emulators {
				if o.EmulatorID == ei.ShareWith && o.ShareWith == "" {
					slots = slices.Clone(o.Slots)
					cleanSlots(slots)
				}
			}
		}
		g.Emulators = append(g.Emulators, library.GameEmulator{EmulatorID: ei.EmulatorID, Slots: slots, ShareWith: ei.ShareWith})
		rom := strings.TrimSpace(ei.Rom)
		paths := map[string]string{}
		for k, v := range ei.Paths {
			if v = strings.TrimSpace(v); v != "" {
				paths[k] = v
			}
		}
		if rom == "" && len(paths) == 0 {
			continue // not set up on this device
		}
		keep[ei.EmulatorID] = library.EmulatorConfig{Rom: rom, Paths: paths, SteamAppID: prev.Emulators[ei.EmulatorID].SteamAppID}
	}
	if len(keep) > 0 {
		d.Emulators = keep
	}
	if _, ok := keep[prev.LastEmulator]; ok {
		d.LastEmulator = prev.LastEmulator
	}
	d.DeviceName = s.c.Settings.Get().DeviceName
	if d.Paths == nil {
		d.Paths = map[string]string{}
	}
	for k, v := range d.Paths {
		d.Paths[k] = strings.TrimSpace(v)
	}
	if g.Devices == nil {
		g.Devices = map[string]library.DeviceConfig{}
	}
	g.Devices[devID] = d
	saved, err := s.c.Library.Save(g, devID)
	if err != nil {
		return GameView{}, err
	}
	// An emulator that now shares (or stopped sharing) another's cloud save syncs
	// with a different cloud folder. What it last synced belongs to the old one,
	// so forget it: the next sync then compares both sides from scratch and, if
	// they differ, asks which to keep instead of silently replacing one.
	if in.ID != "" {
		for _, ge := range saved.Emulators {
			if was, ok := oldGame.Emulator(ge.EmulatorID); ok && was.ShareWith != ge.ShareWith {
				_ = s.c.States.Save(core.SyncKey(saved.ID, ge.EmulatorID), syncer.State{})
			}
		}
	}
	s.bus.libraryChanged(saved.ID)
	s.publish()
	return s.view(saved), nil
}

// publish syncs the library catalog with the cloud in the background, so a
// game added or changed on this device shows up on the others without
// pressing "Sync all". Failures (offline, no cloud yet) are not fatal: the
// change is kept locally and goes out with the next library sync.
func (s *LibraryService) publish() {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		if err := s.c.SyncLibrary(ctx); err != nil {
			log.Printf("library sync: %v", err)
			return
		}
		s.bus.libraryChanged("")
	}()
}

// cleanSlots trims slot keys and filter lists in place.
func cleanSlots(slots []library.SaveSlot) {
	for i := range slots {
		slots[i].Key = strings.TrimSpace(slots[i].Key)
		slots[i].Include = cleanList(slots[i].Include)
		slots[i].Exclude = cleanList(slots[i].Exclude)
	}
}

func cleanList(in []string) []string {
	var out []string
	for _, s := range in {
		if s = strings.TrimSpace(s); s != "" {
			out = append(out, s)
		}
	}
	return out
}

// Delete removes a game from the library (cloud saves are kept).
func (s *LibraryService) Delete(id string) error {
	if err := s.c.Library.Delete(id); err != nil {
		return err
	}
	s.bus.libraryChanged(id)
	s.publish()
	return nil
}

// SetCover copies an image file as the game's cover.
func (s *LibraryService) SetCover(id, src string) (GameView, error) {
	g, err := s.c.Library.Get(id)
	if err != nil {
		return GameView{}, err
	}
	ext := strings.ToLower(filepath.Ext(src))
	switch ext {
	case ".png", ".jpg", ".jpeg", ".webp":
	default:
		return GameView{}, errors.New("cover must be a PNG, JPEG or WebP image")
	}
	name := g.ID + ext
	dst := filepath.Join(s.c.Dirs.CoversDir(), name)
	if filepath.Clean(src) != filepath.Clean(dst) {
		// Copy first, so a failure keeps the current cover.
		if err := copyFile(src, dst); err != nil {
			return GameView{}, err
		}
	}
	if g.Cover != "" && g.Cover != name {
		_ = os.Remove(filepath.Join(s.c.Dirs.CoversDir(), g.Cover))
	}
	g.Cover = name
	saved, err := s.c.Library.Save(g, "")
	if err != nil {
		return GameView{}, err
	}
	s.bus.libraryChanged(saved.ID)
	s.publish()
	return s.view(saved), nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

// Tokens lists path tokens for a game on this device (gameID may be "").
func (s *LibraryService) Tokens(gameID string) []pathvar.Token {
	ctx := pathvar.Context{SteamRoot: s.c.SteamRoot()}
	if g, err := s.c.Library.Get(gameID); err == nil {
		ctx = s.c.PathContext(g)
	}
	return ctx.Tokens()
}

// PreviewPath expands tokens in raw for display.
func (s *LibraryService) PreviewPath(gameID, raw string) (string, error) {
	ctx := pathvar.Context{SteamRoot: s.c.SteamRoot()}
	if g, err := s.c.Library.Get(gameID); err == nil {
		ctx = s.c.PathContext(g)
	}
	return ctx.Expand(raw)
}

// ContractPath rewrites an absolute path using tokens (e.g. after browsing).
func (s *LibraryService) ContractPath(p string) string {
	return pathvar.Context{SteamRoot: s.c.SteamRoot()}.Contract(p)
}

// CheckFolder returns an error unless p is an existing folder (checked before
// opening it in the file manager).
func (s *LibraryService) CheckFolder(p string) error {
	if fi, err := os.Stat(p); err != nil || !fi.IsDir() {
		return fmt.Errorf("folder does not exist: %s", p)
	}
	return nil
}

// SearchSteam looks up games on the Steam store by name, for filling in a
// game's title.
func (s *LibraryService) SearchSteam(term string) ([]steamstore.Match, error) {
	term = strings.TrimSpace(term)
	if len(term) < 2 {
		return []steamstore.Match{}, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return steamstore.Search(ctx, term)
}

// SteamLibraryGame is a game installed through Steam on this device.
type SteamLibraryGame struct {
	steam.InstalledApp
	// GameID is set when the library already has a game linked to this app.
	GameID string `json:"gameId"`
}

// SteamLibrary lists games installed through Steam on this device.
func (s *LibraryService) SteamLibrary() []SteamLibraryGame {
	linked := map[string]string{}
	for _, g := range s.c.Library.List() {
		if g.StoreAppID != "" {
			linked[g.StoreAppID] = g.ID
		}
	}
	out := []SteamLibraryGame{}
	for _, a := range steam.InstalledApps(s.c.SteamRoot()) {
		out = append(out, SteamLibraryGame{InstalledApp: a, GameID: linked[a.AppID]})
	}
	return out
}

// storeCover saves image bytes as the game's cover, replacing any previous one.
func (s *LibraryService) storeCover(g library.Game, data []byte, ext string) (GameView, error) {
	if g.Cover != "" {
		_ = os.Remove(filepath.Join(s.c.Dirs.CoversDir(), g.Cover))
	}
	name := g.ID + ext
	if err := os.WriteFile(filepath.Join(s.c.Dirs.CoversDir(), name), data, 0o644); err != nil {
		return GameView{}, err
	}
	g.Cover = name
	saved, err := s.c.Library.Save(g, "")
	if err != nil {
		return GameView{}, err
	}
	s.bus.libraryChanged(saved.ID)
	s.publish()
	return s.view(saved), nil
}

func (s *LibraryService) gridClient() (*steamgrid.Client, error) {
	key := s.c.Settings.Get().SteamGridDBKey
	if key == "" {
		return nil, errors.New("add your SteamGridDB API key in Settings first")
	}
	return steamgrid.New(key), nil
}

// HasSteamGridKey reports whether a SteamGridDB API key is configured.
func (s *LibraryService) HasSteamGridKey() bool {
	return s.c.Settings.Get().SteamGridDBKey != ""
}

// TestSteamGridKey checks a SteamGridDB API key without saving it.
func (s *LibraryService) TestSteamGridKey(key string) error {
	if strings.TrimSpace(key) == "" {
		return errors.New("enter an API key")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	return steamgrid.New(key).Validate(ctx)
}

// SearchGridGames finds games on SteamGridDB by name.
func (s *LibraryService) SearchGridGames(term string) ([]steamgrid.Game, error) {
	c, err := s.gridClient()
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(term) == "" {
		return []steamgrid.Game{}, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	return c.SearchGames(ctx, strings.TrimSpace(term))
}

// GridArt lists SteamGridDB artwork of a kind ("cover", "wide", "hero", "logo"
// or "icon"), for a SteamGridDB game id or (when gridGameID is 0) a Steam app id.
func (s *LibraryService) GridArt(kind string, gridGameID int, steamAppID string) ([]steamgrid.Grid, error) {
	c, err := s.gridClient()
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	return c.Art(ctx, steamgrid.Kind(kind), gridGameID, steamAppID)
}

// SetCoverFromURL downloads a SteamGridDB image and makes it the game's cover.
func (s *LibraryService) SetCoverFromURL(id, imageURL string) (GameView, error) {
	g, err := s.c.Library.Get(id)
	if err != nil {
		return GameView{}, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	data, ext, err := steamgrid.Download(ctx, &http.Client{Timeout: 60 * time.Second}, imageURL)
	if err != nil {
		return GameView{}, err
	}
	return s.storeCover(g, data, ext)
}

func artKind(kind string) error {
	for _, k := range steamgrid.ArtKinds {
		if string(k) == kind {
			return nil
		}
	}
	return fmt.Errorf("unknown artwork kind %q", kind)
}

// storeArt saves image bytes as the game's artwork of a kind, replacing any previous one.
func (s *LibraryService) storeArt(g library.Game, kind string, data []byte, ext string) (GameView, error) {
	name := g.ID + "_" + kind + ext
	if err := os.WriteFile(filepath.Join(s.c.Dirs.CoversDir(), name), data, 0o644); err != nil {
		return GameView{}, err
	}
	return s.saveArt(g, kind, name)
}

// saveArt points the game's artwork of a kind at a file ("" removes it) and
// deletes the file it replaces.
func (s *LibraryService) saveArt(g library.Game, kind, name string) (GameView, error) {
	if old := g.Art[kind]; old != "" && old != name {
		_ = os.Remove(filepath.Join(s.c.Dirs.CoversDir(), old))
	}
	art := map[string]string{}
	maps.Copy(art, g.Art)
	if name == "" {
		delete(art, kind)
	} else {
		art[kind] = name
	}
	g.Art = art
	saved, err := s.c.Library.Save(g, "")
	if err != nil {
		return GameView{}, err
	}
	s.bus.libraryChanged(saved.ID)
	s.publish()
	return s.view(saved), nil
}

// SetArt copies an image file as the game's artwork of a kind ("wide", "hero",
// "logo" or "icon").
func (s *LibraryService) SetArt(id, kind, src string) (GameView, error) {
	if err := artKind(kind); err != nil {
		return GameView{}, err
	}
	g, err := s.c.Library.Get(id)
	if err != nil {
		return GameView{}, err
	}
	ext := strings.ToLower(filepath.Ext(src))
	switch ext {
	case ".png", ".jpg", ".jpeg", ".webp":
	default:
		return GameView{}, errors.New("image must be a PNG, JPEG or WebP file")
	}
	data, err := os.ReadFile(src)
	if err != nil {
		return GameView{}, err
	}
	return s.storeArt(g, kind, data, ext)
}

// SetArtFromURL downloads a SteamGridDB image as the game's artwork of a kind.
func (s *LibraryService) SetArtFromURL(id, kind, imageURL string) (GameView, error) {
	if err := artKind(kind); err != nil {
		return GameView{}, err
	}
	g, err := s.c.Library.Get(id)
	if err != nil {
		return GameView{}, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	data, ext, err := steamgrid.Download(ctx, &http.Client{Timeout: 60 * time.Second}, imageURL)
	if err != nil {
		return GameView{}, err
	}
	return s.storeArt(g, kind, data, ext)
}

// ClearArt removes the game's artwork of a kind.
func (s *LibraryService) ClearArt(id, kind string) (GameView, error) {
	if err := artKind(kind); err != nil {
		return GameView{}, err
	}
	g, err := s.c.Library.Get(id)
	if err != nil {
		return GameView{}, err
	}
	return s.saveArt(g, kind, "")
}

// SuggestInput is the editor's current view of a game (it may not be saved yet).
type SuggestInput struct {
	GameID     string             `json:"gameId"`
	Name       string             `json:"name"`
	StoreAppID string             `json:"storeAppId"`
	Exe        string             `json:"exe"`
	LaunchMode library.LaunchMode `json:"launchMode"`
}

// SaveSuggestion is a suggested save location, checked on this device.
type SaveSuggestion struct {
	ludusavi.Suggestion
	Resolved string `json:"resolved"` // absolute path on this device
	Exists   bool   `json:"exists"`
	Files    int    `json:"files"`
	Bytes    int64  `json:"bytes"`
	Newest   string `json:"newest"`
}

// SuggestResult lists save locations from the Ludusavi manifest.
type SuggestResult struct {
	Matched     string           `json:"matched"` // manifest game name; "" if unknown
	Suggestions []SaveSuggestion `json:"suggestions"`
}

// SuggestSaves looks the game up in the Ludusavi manifest and returns its save
// locations for this device, existing folders first.
func (s *LibraryService) SuggestSaves(in SuggestInput) (SuggestResult, error) {
	res := SuggestResult{Suggestions: []SaveSuggestion{}}
	g, err := ludusavi.Find(strings.TrimSpace(in.StoreAppID), in.Name)
	if err != nil || g == nil {
		return res, err
	}
	res.Matched = g.Name

	steamRoot := s.c.SteamRoot()
	env := ludusavi.Env{GOOS: runtime.GOOS}
	if steamRoot != "" {
		env.SteamRoot = "{steam}"
	}
	pctx := pathvar.Context{SteamRoot: steamRoot}
	var dev library.DeviceConfig
	if game, err := s.c.Library.Get(in.GameID); err == nil {
		pctx = s.c.PathContext(game)
		dev, _ = game.Device(s.c.DeviceID())
	}
	// Install folder: Steam's, if the game is installed through Steam here;
	// otherwise the executable's folder.
	var steamInstalled bool
	if in.StoreAppID != "" {
		for _, a := range steam.InstalledApps(steamRoot) {
			if a.AppID == in.StoreAppID {
				env.InstallDir, steamInstalled = a.InstallDir, true
				break
			}
		}
	}
	var exe string
	if strings.TrimSpace(in.Exe) != "" {
		if p, err := pctx.Expand(in.Exe); err == nil && filepath.IsAbs(p) {
			exe = p
		}
	}
	if env.InstallDir == "" && exe != "" {
		env.InstallDir = filepath.Dir(exe)
	}
	if env.InstallDir != "" {
		env.InstallDir = pctx.Contract(env.InstallDir) // e.g. {steam}/steamapps/common/Game
	}
	if runtime.GOOS == "linux" {
		// A game Steam installed runs in its own app id's prefix; anything else
		// runs through Carryover's non-Steam shortcut.
		env.Proton = "{proton:shortcut}"
		if steamInstalled && dev.SteamAppID == "" {
			env.Proton = "{proton:" + in.StoreAppID + "}"
		}
		// Started through Heroic, it runs in Heroic's prefix, never Steam's.
		if in.LaunchMode == library.LaunchHeroic {
			env.Proton = ""
		}
	}
	names := []string{g.Name, in.Name}
	if exe != "" {
		if hg, ok := s.heroicGame(exe); ok && hg.Title != "" {
			names = append(names, hg.Title)
		}
	}

	check := func(sg ludusavi.Suggestion) SaveSuggestion {
		out := SaveSuggestion{Suggestion: sg}
		if out.Include == nil {
			out.Include = []string{}
		}
		if p, err := pctx.Expand(sg.Path); err == nil {
			out.Resolved = p
			out.Exists, out.Files, out.Bytes, out.Newest = scanFolder(p, sg.Include)
		}
		return out
	}
	for _, sg := range ludusavi.Suggest(g, env) {
		res.Suggestions = append(res.Suggestions, check(sg))
	}
	// Heroic, Lutris and Bottles prefixes: all locations in the game's own
	// prefix, and in any other prefix only those that hold saves.
	if s.prefixes != nil {
		for _, pf := range s.prefixes() {
			mine := prefixIsFor(pf, exe, names...)
			for _, user := range pf.Users {
				userDir := filepath.ToSlash(pctx.Contract(user))
				for _, sg := range ludusavi.SuggestPrefix(g, userDir, filepath.Base(user)) {
					sg.Prefix = prefixLabel(pf)
					if out := check(sg); mine || out.Files > 0 {
						res.Suggestions = append(res.Suggestions, out)
					}
				}
			}
		}
	}
	sort.SliceStable(res.Suggestions, func(i, j int) bool {
		a, b := res.Suggestions[i], res.Suggestions[j]
		if a.Files > 0 != (b.Files > 0) {
			return a.Files > 0
		}
		if a.Exists != b.Exists {
			return a.Exists
		}
		// Unlikely locations last: the Microsoft Store version's folder, and
		// Steam Cloud's own folder (Steam already syncs that one).
		if a.MSStore != b.MSStore {
			return !a.MSStore
		}
		return a.Kind != ludusavi.KindSteamCloud && b.Kind == ludusavi.KindSteamCloud
	})
	return res, nil
}

// heroicGame returns the Heroic-installed game whose folder holds exe.
func (s *LibraryService) heroicGame(exe string) (heroic.Game, bool) {
	if runtime.GOOS != "linux" && runtime.GOOS != "windows" {
		return heroic.Game{}, false
	}
	home, _ := os.UserHomeDir()
	return heroic.ForExe(home, exe)
}

// HeroicGame returns the title (or Heroic's app name) of the game Heroic
// installed in the folder holding exe, and whether there is one (Linux and Windows).
func (s *LibraryService) HeroicGame(exe string) (string, bool) {
	g, ok := s.heroicGame(strings.TrimSpace(exe))
	if !ok {
		return "", false
	}
	if g.Title != "" {
		return g.Title, true
	}
	return g.AppName, true
}

// prefixIsFor reports whether a Wine prefix belongs to the game: the launcher
// lists it under one of the game's names, or the game's exe is inside it.
func prefixIsFor(pf wineprefix.Prefix, exe string, names ...string) bool {
	if exe != "" && strings.HasPrefix(filepath.Clean(exe), filepath.Clean(pf.Path)+string(filepath.Separator)) {
		return true
	}
	for _, n := range names {
		if ludusavi.SameTitle(pf.Name, n) || slices.ContainsFunc(pf.Games, func(g string) bool { return ludusavi.SameTitle(g, n) }) {
			return true
		}
	}
	return false
}

// prefixLabel names a prefix for the user, e.g. "Heroic: Hades".
func prefixLabel(pf wineprefix.Prefix) string {
	launcher := map[string]string{
		wineprefix.Heroic: "Heroic", wineprefix.Lutris: "Lutris", wineprefix.Bottles: "Bottles",
	}[pf.Launcher]
	if launcher == "" {
		launcher = "Wine"
	}
	return launcher + ": " + pf.Name
}

// scanFolder reports whether dir exists and counts matching files (bounded).
func scanFolder(dir string, include []string) (exists bool, files int, bytes int64, newest string) {
	fi, err := os.Stat(dir)
	if err != nil || !fi.IsDir() {
		return false, 0, 0, ""
	}
	var newestT time.Time
	_ = filepath.WalkDir(dir, func(p string, d fs.DirEntry, walkErr error) error {
		// Unreadable entries are skipped: this is only a preview.
		if walkErr != nil || d.IsDir() || files >= 5000 {
			return nil //nolint:nilerr // best-effort preview
		}
		rel, _ := filepath.Rel(dir, p)
		rel = filepath.ToSlash(rel)
		if len(include) > 0 && !matchesAny(include, rel) {
			return nil
		}
		info, infoErr := d.Info()
		if infoErr != nil {
			return nil //nolint:nilerr // best-effort preview
		}
		files++
		bytes += info.Size()
		if info.ModTime().After(newestT) {
			newestT = info.ModTime()
		}
		return nil
	})
	return true, files, bytes, fmtTime(newestT)
}

// matchesAny is a light approximation of rclone include globs for previews.
func matchesAny(globs []string, rel string) bool {
	for _, g := range globs {
		if prefix, ok := strings.CutSuffix(g, "/**"); ok {
			if ok, _ := path.Match(prefix, firstSegments(rel, strings.Count(g, "/"))); ok {
				return true
			}
			continue
		}
		if ok, _ := path.Match(g, rel); ok {
			return true
		}
		if ok, _ := path.Match(g, path.Base(rel)); ok && !strings.Contains(g, "/") {
			return true
		}
	}
	return false
}

func firstSegments(rel string, n int) string {
	parts := strings.Split(rel, "/")
	if len(parts) > n {
		parts = parts[:n]
	}
	return strings.Join(parts, "/")
}
