package services

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"

	"github.com/akfaiz/playrelay/internal/core"
	"github.com/akfaiz/playrelay/internal/library"
	"github.com/akfaiz/playrelay/internal/ludusavi"
	"github.com/akfaiz/playrelay/internal/pathvar"
	"github.com/akfaiz/playrelay/internal/steam"
	"github.com/akfaiz/playrelay/internal/steamstore"
)

// LibraryService manages games.
type LibraryService struct {
	c       *core.Core
	running func(string) bool
}

// NewLibraryService creates the service.
func NewLibraryService(c *core.Core, running func(string) bool) *LibraryService {
	return &LibraryService{c: c, running: running}
}

// GameStatus is the sync state of a game on this device.
type GameStatus struct {
	LastSyncAt string `json:"lastSyncAt"`
	LastAction string `json:"lastAction"`
	Conflict   bool   `json:"conflict"`
	LastError  string `json:"lastError"`
	Running    bool   `json:"running"`
}

// GameView is a game as presented to the UI, from this device's perspective.
type GameView struct {
	ID            string               `json:"id"`
	Name          string               `json:"name"`
	CoverURL      string               `json:"coverUrl"`
	StoreAppID    string               `json:"storeAppId"`
	Slots         []library.SaveSlot   `json:"slots"`
	Device        library.DeviceConfig `json:"device"`
	HasDevice     bool                 `json:"hasDevice"`
	Ready         bool                 `json:"ready"`     // all save paths + exe set on this device
	SetupHint     string               `json:"setupHint"` // what's missing
	ResolvedPaths map[string]string    `json:"resolvedPaths"`
	PathErrors    map[string]string    `json:"pathErrors"`
	OtherDevices  []DevicePaths        `json:"otherDevices"`
	Status        GameStatus           `json:"status"`
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
	}
	if v.Slots == nil {
		v.Slots = []library.SaveSlot{}
	}
	if d.Paths == nil {
		v.Device.Paths = map[string]string{}
	}
	if g.Cover != "" {
		v.CoverURL = "/covers/" + g.Cover
		if fi, err := os.Stat(filepath.Join(s.c.Dirs.CoversDir(), g.Cover)); err == nil {
			v.CoverURL += fmt.Sprintf("?v=%d", fi.ModTime().Unix())
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
	if d.Exe == "" && d.LaunchMode != library.LaunchSteamWrap {
		missing = append(missing, "game executable")
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
	if st, err := s.c.States.Load(g.ID); err == nil {
		v.Status = GameStatus{
			LastSyncAt: fmtTime(st.LastSyncAt), LastAction: string(st.LastAction),
			Conflict: st.Conflict, LastError: st.LastError,
		}
	}
	if s.running != nil {
		v.Status.Running = s.running(g.ID)
	}
	return v
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
}

// DeviceInput is the editor's view of this device's config. Server-managed
// fields (timestamps, device name, Steam app id) are deliberately absent so
// the frontend can't send invalid values for them.
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
	if in.ID != "" {
		old, err := s.c.Library.Get(in.ID)
		if err != nil {
			return GameView{}, err
		}
		g.Cover = old.Cover
		g.Devices = old.Devices
		// Steam app id is managed by AddToSteam, not the editor.
		d.SteamAppID = old.Devices[devID].SteamAppID
	}
	for i := range g.Slots {
		g.Slots[i].Key = strings.TrimSpace(g.Slots[i].Key)
		g.Slots[i].Include = cleanList(g.Slots[i].Include)
		g.Slots[i].Exclude = cleanList(g.Slots[i].Exclude)
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
	emit(EventLibrary, saved.ID)
	return s.view(saved), nil
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
	emit(EventLibrary, id)
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
	if g.Cover != "" {
		_ = os.Remove(filepath.Join(s.c.Dirs.CoversDir(), g.Cover))
	}
	name := g.ID + ext
	if err := copyFile(src, filepath.Join(s.c.Dirs.CoversDir(), name)); err != nil {
		return GameView{}, err
	}
	g.Cover = name
	saved, err := s.c.Library.Save(g, "")
	if err != nil {
		return GameView{}, err
	}
	emit(EventLibrary, saved.ID)
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

// PickFile shows an open-file dialog. Returns "" if cancelled.
func (s *LibraryService) PickFile(title, filterName, pattern string) (string, error) {
	d := application.Get().Dialog.OpenFile().SetTitle(title).CanChooseFiles(true)
	if pattern != "" {
		d = d.AddFilter(filterName, pattern)
	}
	return d.PromptForSingleSelection()
}

// PickFolder shows a choose-folder dialog. Returns "" if cancelled.
func (s *LibraryService) PickFolder(title string) (string, error) {
	return application.Get().Dialog.OpenFile().SetTitle(title).
		CanChooseDirectories(true).CanChooseFiles(false).CanCreateDirectories(true).
		PromptForSingleSelection()
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

// OpenFolder opens a folder in the system file manager.
func (s *LibraryService) OpenFolder(p string) error {
	if fi, err := os.Stat(p); err != nil || !fi.IsDir() {
		return fmt.Errorf("folder does not exist: %s", p)
	}
	return application.Get().Browser.OpenFile(p)
}

// CoversHandler serves cover images under /covers/ and delegates everything else.
func CoversHandler(c *core.Core, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if name, ok := strings.CutPrefix(r.URL.Path, "/covers/"); ok {
			name = filepath.Base(name)
			w.Header().Set("Cache-Control", "max-age=31536000")
			http.ServeFile(w, r, filepath.Join(c.Dirs.CoversDir(), name))
			return
		}
		next.ServeHTTP(w, r)
	})
}

// SearchSteam looks up games on the Steam store by name, for filling in a
// game's title and cover.
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

// FetchSteamCover downloads the game's cover from Steam (local Steam cache
// first, then Steam's CDN) using its store app id.
func (s *LibraryService) FetchSteamCover(id string) (GameView, error) {
	g, err := s.c.Library.Get(id)
	if err != nil {
		return GameView{}, err
	}
	if g.StoreAppID == "" {
		return GameView{}, errors.New("link the game to Steam first")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	data, err := steamstore.Cover(ctx, s.c.SteamRoot(), g.StoreAppID)
	if errors.Is(err, steamstore.ErrNotFound) {
		return GameView{}, errors.New("no cover art on Steam for this game")
	}
	if err != nil {
		return GameView{}, fmt.Errorf("download cover from Steam: %w", err)
	}
	if g.Cover != "" {
		_ = os.Remove(filepath.Join(s.c.Dirs.CoversDir(), g.Cover))
	}
	name := g.ID + ".jpg"
	if err := os.WriteFile(filepath.Join(s.c.Dirs.CoversDir(), name), data, 0o644); err != nil {
		return GameView{}, err
	}
	g.Cover = name
	saved, err := s.c.Library.Save(g, "")
	if err != nil {
		return GameView{}, err
	}
	emit(EventLibrary, saved.ID)
	return s.view(saved), nil
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
	if env.InstallDir == "" && strings.TrimSpace(in.Exe) != "" {
		if exe, err := pctx.Expand(in.Exe); err == nil && filepath.IsAbs(exe) {
			env.InstallDir = filepath.Dir(exe)
		}
	}
	if env.InstallDir != "" {
		env.InstallDir = pctx.Contract(env.InstallDir) // e.g. {steam}/steamapps/common/Game
	}
	if runtime.GOOS == "linux" {
		// A game Steam installed runs in its own app id's prefix; anything else
		// runs through PlayRelay's non-Steam shortcut.
		env.Proton = "{proton:shortcut}"
		if steamInstalled && dev.SteamAppID == "" {
			env.Proton = "{proton:" + in.StoreAppID + "}"
		}
	}

	for _, sg := range ludusavi.Suggest(g, env) {
		out := SaveSuggestion{Suggestion: sg}
		if out.Include == nil {
			out.Include = []string{}
		}
		if p, err := pctx.Expand(sg.Path); err == nil {
			out.Resolved = p
			out.Exists, out.Files, out.Bytes, out.Newest = scanFolder(p, sg.Include)
		}
		res.Suggestions = append(res.Suggestions, out)
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
