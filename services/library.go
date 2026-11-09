package services

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/wailsapp/wails/v3/pkg/application"

	"github.com/akfaiz/playrelay/internal/core"
	"github.com/akfaiz/playrelay/internal/library"
	"github.com/akfaiz/playrelay/internal/pathvar"
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
		ID: g.ID, Name: g.Name, Slots: g.Slots, Device: d, HasDevice: has,
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
	ID     string               `json:"id"`
	Name   string               `json:"name"`
	Slots  []library.SaveSlot   `json:"slots"`
	Device library.DeviceConfig `json:"device"`
}

// Save creates or updates a game.
func (s *LibraryService) Save(in GameInput) (GameView, error) {
	devID := s.c.DeviceID()
	g := library.Game{ID: in.ID, Name: strings.TrimSpace(in.Name), Slots: in.Slots}
	if in.ID != "" {
		old, err := s.c.Library.Get(in.ID)
		if err != nil {
			return GameView{}, err
		}
		g.Cover = old.Cover
		g.Devices = old.Devices
		// Steam app id is managed by AddToSteam, not the editor.
		in.Device.SteamAppID = old.Devices[devID].SteamAppID
	}
	for i := range g.Slots {
		g.Slots[i].Key = strings.TrimSpace(g.Slots[i].Key)
		g.Slots[i].Include = cleanList(g.Slots[i].Include)
		g.Slots[i].Exclude = cleanList(g.Slots[i].Exclude)
	}
	d := in.Device
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
