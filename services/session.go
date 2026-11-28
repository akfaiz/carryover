package services

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/akfaiz/carryover/internal/app/msg"
	"github.com/akfaiz/carryover/internal/cloudsync/syncer"
	"github.com/akfaiz/carryover/internal/core"
	"github.com/akfaiz/carryover/internal/library"
)

func goos() string { return runtime.GOOS }

// SessionService launches games (pull → play → push) and relays prompts.
type SessionService struct {
	c   *core.Core
	p   *prompter
	bus *Bus

	mu      sync.Mutex
	running map[string]bool
	status  map[string]Status
	cancels map[string]func() // gameID -> cancel for the sync phase in progress

	// askProfile: in wrapper mode, ask which emulator to play with when the
	// game has several ways to be played and none was named.
	askProfile bool

	// Wrapper mode (carryover run <id>).
	wrapGameID string
	wrapOpts   core.RunOptions
	done       chan SessionOutcome
	startOnce  sync.Once
	// OnGameRunning is invoked when the game starts/stops (used to hide the window).
	OnGameRunning func(bool)
	// OnPrompt is invoked when a prompt is raised (used to show the window).
	OnPrompt func()
}

// SessionOutcome is the result of a wrapper-mode session.
type SessionOutcome struct {
	ExitCode int
	Err      error
}

// NewSessionService creates the service and routes core sync progress to bus.
func NewSessionService(c *core.Core, bus *Bus) *SessionService {
	s := &SessionService{c: c, p: newPrompter(bus), bus: bus, running: map[string]bool{}, status: map[string]Status{}}
	c.SetEventHandler(bus.syncProgress)
	return s
}

// ConfigureWrapper sets up wrapper mode; the session starts when the UI calls Start.
func ConfigureWrapper(s *SessionService, gameID string, opts core.RunOptions) <-chan SessionOutcome {
	s.wrapGameID, s.wrapOpts = gameID, opts
	s.askProfile = true
	s.done = make(chan SessionOutcome, 1)
	s.p.onAsk = func() {
		if s.OnPrompt != nil {
			s.OnPrompt()
		}
	}
	return s.done
}

// Start begins the wrapper session (no-op in launcher mode).
func (s *SessionService) Start() {
	if s.wrapGameID == "" {
		return
	}
	s.startOnce.Do(func() {
		go func() {
			res, err := s.play(context.Background(), s.wrapGameID, s.wrapOpts)
			s.done <- SessionOutcome{ExitCode: res.ExitCode, Err: err}
		}()
	})
}

// IsRunning reports whether a session is active for the game or, with a sync
// key (core.SyncKey), for that profile. A game id covers all of its profiles.
func (s *SessionService) IsRunning(gameID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.running[gameID] {
		return true
	}
	for k := range s.running {
		if strings.HasPrefix(k, gameID+".") {
			return true
		}
	}
	return false
}

// Active reports whether any play session (including its syncs before and
// after the game) is running.
func (s *SessionService) Active() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.running) > 0
}

// setStatus records a status under the profile's key and the game id (so a
// game tile shows the session of any of its profiles).
func (s *SessionService) setStatus(st Status) {
	s.mu.Lock()
	if st.Key != "" {
		s.status[st.Key] = st
	}
	s.status[st.GameID] = st
	s.mu.Unlock()
	s.bus.statusChanged(st)
}

// play runs a session. gameID is a game id or name, or a sync key
// (core.SyncKey) to play with one emulator; opts.Emulator picks the profile
// otherwise ("" = the one played last).
func (s *SessionService) play(ctx context.Context, gameID string, opts core.RunOptions) (core.SessionResult, error) {
	ask := noChoice
	if s.askProfile {
		ask = EmulatorChoiceSeconds // a Steam shortcut: start the last one if nobody answers
	}
	return s.playAsking(ctx, gameID, opts, ask)
}

// noChoice is playAsking's askSeconds for "never ask which emulator".
const noChoice = -1

// playAsking is play that, when the game can be played in several ways and none
// was named, first asks which one (askSeconds >= 0). With askSeconds > 0 the
// prompt starts the one played last when it runs out; with 0 it waits for an
// answer, as when the user pressed Play in the launcher.
func (s *SessionService) playAsking(ctx context.Context, gameID string, opts core.RunOptions, askSeconds int) (core.SessionResult, error) {
	r, err := s.open(ctx, gameID, opts, askSeconds)
	if err != nil {
		return core.SessionResult{}, err
	}
	defer r.done()
	res, err := s.c.Play(ctx, r.game.ID, r.opts, r.ui)
	switch {
	case errors.Is(err, core.ErrCancelled):
		r.status(Status{Phase: "done"}.saying(msg.SessionCancelled.Msg()))
	case err != nil:
		r.status(Status{Phase: "error"}.saying(msg.Of(err)))
	case errors.Is(res.PostError, core.ErrOffline):
		r.status(Status{Phase: "done"}.saying(msg.SessionClosedOffline.Msg()))
	case errors.Is(res.PostError, core.ErrCancelled):
		r.status(Status{Phase: "done"}.saying(msg.SessionClosedCancelled.Msg()))
	case res.PostError != nil:
		r.status(Status{Phase: "error"}.saying(msg.SessionUploadFailed.Wrap(res.PostError).Message()))
	case res.PostAction == syncer.ActionConflict:
		r.status(Status{Phase: "done"}.saying(msg.SessionClosedConflict.Msg()))
	default:
		r.status(Status{Phase: "done"}.saying(msg.SessionClosedSynced.Msg()))
	}
	return res, err
}

// sessionRun is a session the service is running: the game, the profile it
// is played with (opts.Emulator is resolved) and the UI the core asks through.
type sessionRun struct {
	game   library.Game
	opts   core.RunOptions
	name   string // the game's name, plus " (<emulator>)" for an emulator profile
	ui     *guiUI
	status func(Status)
	done   func() // ends the service's bookkeeping of the session
}

// open prepares a session of gameID (a game id or name, or a sync key):
// it asks which emulator to use when askSeconds >= 0 and the game can be
// played in several ways (see playAsking), resolves the profile, and marks it
// as running until done is called.
func (s *SessionService) open(ctx context.Context, gameID string, opts core.RunOptions, askSeconds int) (*sessionRun, error) {
	g, err := s.c.Library.Find(gameID)
	if err != nil {
		// Not a game id or name: maybe "<game id>.<emulator id>".
		id, emu := core.ParseKey(gameID)
		if g, err = s.c.Library.Find(id); err == nil && emu != "" && opts.Emulator == "" {
			opts.Emulator = emu
		}
	}
	if err != nil {
		s.setStatus(Status{GameID: gameID, Phase: "error"}.saying(msg.Of(err)))
		return nil, err
	}
	if askSeconds >= 0 && opts.Emulator == "" && len(opts.Command) == 0 {
		if choices, def := s.profileChoices(g); len(choices) > 1 {
			// Ask which emulator to use (a Steam shortcut covers the whole game).
			s.setStatus(Status{GameID: g.ID, Phase: "choose"}.saying(msg.SessionChoose.Msg()))
			ans := s.p.ask(ctx, Prompt{
				Kind: "emulator", GameID: g.ID, GameName: g.Name,
				Choices: choices, Default: def, Seconds: askSeconds,
			})
			if ans == "" || ans == "cancel" {
				s.setStatus(Status{GameID: g.ID, Phase: "done"}.saying(msg.SessionCancelled.Msg()))
				return nil, core.ErrCancelled
			}
			opts.Emulator = ans
		}
	}
	emuID, err := s.c.ResolveProfile(g, opts.Emulator)
	if err != nil {
		s.setStatus(Status{GameID: g.ID, Phase: "error"}.saying(msg.Of(err)))
		return nil, err
	}
	opts.Emulator = emuID
	if emuID == "" {
		opts.Emulator = core.NativeProfile
	}
	key := core.SyncKey(g.ID, emuID)
	suffix, name := "", g.Name
	if emuID != "" {
		suffix = " (" + s.c.ProfileName(emuID) + ")"
		name += suffix
	}
	status := func(st Status) {
		st.GameID, st.Profile, st.Key = g.ID, emuID, key
		s.setStatus(st)
	}

	s.mu.Lock()
	if s.running[key] {
		s.mu.Unlock()
		return nil, msg.SessionAlreadyRunning.Err("Name", name)
	}
	s.running[key] = true
	s.mu.Unlock()
	s.bus.libraryChanged(g.ID)
	done := func() {
		s.mu.Lock()
		delete(s.running, key)
		s.mu.Unlock()
		s.bus.libraryChanged(g.ID)
	}

	ui := &guiUI{gameID: g.ID, profile: emuID, key: key, suffix: suffix, p: s.p, onStatus: s.setStatus, onRunning: func(r bool) {
		if s.OnGameRunning != nil {
			s.OnGameRunning(r)
		}
	}, onCancelable: func(cancel func()) {
		s.mu.Lock()
		if s.cancels == nil {
			s.cancels = map[string]func(){}
		}
		if cancel == nil {
			delete(s.cancels, key)
		} else {
			s.cancels[key] = cancel
		}
		s.mu.Unlock()
	}}
	return &sessionRun{game: g, opts: opts, name: name, ui: ui, status: status, done: done}, nil
}

// Begin starts playing gameID (a game id, or a sync key for an emulator
// profile) with a game that the caller starts itself, as the Android app does
// with an emulator: it syncs before playing, asking through prompts as Play
// does, takes the cloud lock and returns. emulator picks the profile as in
// PlayWith; a game that can be played in several ways and none named asks
// which one. Once the game has closed, End finishes the session. It returns
// core.ErrCancelled when the game shouldn't start.
func (s *SessionService) Begin(gameID, emulator string) (core.SessionStart, error) {
	ctx := context.Background()
	r, err := s.open(ctx, gameID, core.RunOptions{Emulator: emulator}, 0)
	if err != nil {
		return core.SessionStart{}, err
	}
	defer r.done()
	start, err := s.c.StartSession(ctx, r.game.ID, r.opts.Emulator, r.ui)
	switch {
	case errors.Is(err, core.ErrCancelled):
		r.status(Status{Phase: "done"}.saying(msg.SessionCancelled.Msg()))
	case err != nil:
		r.status(Status{Phase: "error"}.saying(msg.Of(err)))
	case start.PreOffline:
		r.status(Status{Phase: "playing", Running: true}.saying(msg.SessionPlayingOffline.Msg("Name", r.name)))
	default:
		r.status(Status{Phase: "playing", Running: true}.saying(msg.SessionPlaying.Msg("Name", r.name)))
	}
	return start, err
}

// End finishes a session started with Begin, once the game has closed: it
// uploads the saves if only this device changed them (a conflict is flagged,
// nothing is downloaded), releases the cloud lock, and returns what to tell
// the user (nil: nothing worth telling). key is a game id or sync key. Offline
// the error wraps core.ErrOffline and the next sync uploads the saves.
func (s *SessionService) End(key string) (*core.Notice, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()
	gameID, emuID := core.ParseKey(key)
	s.setStatus(Status{GameID: gameID, Profile: emuID, Key: key, Phase: "sync"}.saying(msg.SessionUploading.Msg()))
	_, notice, err := s.c.EndSession(ctx, s.c.CloudKeyOf(key))
	s.reportEnd(key, notice, err)
	return notice, err
}

// EndPending finishes a session that ended without its upload (the cloud
// couldn't be reached), once the network is back: a retry in the background.
// It does nothing, returning no notice and no error, when the session was
// finished meanwhile or the game was started again.
func (s *SessionService) EndPending(key string) (*core.Notice, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()
	finished, notice, err := s.c.EndPending(ctx, s.c.CloudKeyOf(key))
	if !finished && notice == nil && err == nil {
		return nil, nil
	}
	s.reportEnd(key, notice, err)
	return notice, err
}

// reportEnd reports how a session's end went, as a status and a library change.
func (s *SessionService) reportEnd(key string, notice *core.Notice, err error) {
	gameID, emuID := core.ParseKey(key)
	phase, said := "done", msg.SessionClosedSynced.Msg()
	switch {
	case errors.Is(err, core.ErrOffline):
		said = msg.SessionClosedOffline.Msg()
	case err != nil:
		phase, said = "error", msg.SessionUploadFailed.Wrap(err).Message()
	case notice != nil && notice.Kind == core.NoticeConflict:
		said = msg.SessionClosedConflict.Msg()
	case notice != nil && notice.Kind == core.NoticeFailed:
		failure := msg.FromSaved(notice.Err, notice.ErrMsg)
		said = msg.SessionUploadFailed.Msg()
		said.Cause, said.Text = &failure, said.Text+": "+failure.Text
		phase = "error"
	}
	st := Status{GameID: gameID, Profile: emuID, Key: key, Phase: phase}.saying(said)
	s.setStatus(st)
	s.bus.libraryChanged(gameID)
}

// EmulatorChoiceSeconds is how long the "which emulator?" prompt waits before it
// starts the one played last.
var EmulatorChoiceSeconds = 5

// profileChoices lists the ways the game can be played on this device (its own
// launch config, and each emulator that has an executable here) and which one
// to start by default: the one played last, else the first.
func (s *SessionService) profileChoices(g library.Game) (choices []ProfileChoice, def string) {
	devID := s.c.DeviceID()
	for _, id := range s.c.Profiles(g) {
		if id == "" {
			native := msg.SessionNoEmulator.Msg()
			choices = append(choices, ProfileChoice{ID: core.NativeProfile, Name: native.Text, NameMsg: native.Ptr()})
			continue
		}
		if emu, err := s.c.Library.Emulator(id); err == nil && emu.Devices[devID].Exe != "" {
			choices = append(choices, ProfileChoice{ID: id, Name: emu.Name})
		}
	}
	if len(choices) == 0 {
		return nil, ""
	}
	def = choices[0].ID
	last := g.Devices[devID].LastEmulator
	if last == "" {
		last = core.NativeProfile
	}
	for _, c := range choices {
		if c.ID == last {
			def = c.ID
		}
	}
	return choices, def
}

// Play starts a game from the launcher. It returns once the game has started
// syncing; progress is reported via events.
//
// gameID is a game id (a game that can be played in several ways on this device
// asks which one, see PlayWith) or a sync key (core.SyncKey) to play with one
// specific emulator.
func (s *SessionService) Play(gameID string) error { return s.PlayWith(gameID, "") }

// PlayWith is Play with an explicit profile: an emulator id, core.NativeProfile
// for the game's own launch config, or "" to decide: a game that can be played
// in several ways on this device asks which one (the one played last is
// highlighted), any other starts at once.
func (s *SessionService) PlayWith(gameID, emulator string) error {
	if s.IsRunning(gameID) {
		return msg.SessionGameRunning.Err()
	}
	go func() {
		_, _ = s.playAsking(context.Background(), gameID, core.RunOptions{Emulator: emulator}, 0)
	}()
	return nil
}

// Cancel stops the sync phase of a play session: before the game starts it
// cancels the launch, after it closes it stops the upload (the game itself is
// never stopped). Returns false if no sync is running for the game.
func (s *SessionService) Cancel(gameID string) bool {
	s.mu.Lock()
	cancel := s.cancels[gameID]
	s.mu.Unlock()
	if cancel == nil {
		return false
	}
	cancel()
	return true
}

// Respond answers a prompt. answer: conflict → "local"/"cloud"/"skip"; lock/syncError → "yes"/"no".
func (s *SessionService) Respond(promptID, answer string) bool {
	return s.p.respond(promptID, answer)
}

// Prompts returns unanswered prompts (used by the UI on start).
func (s *SessionService) Prompts() []Prompt { return s.p.list() }

// WrapperState is the state of wrapper mode for the UI.
type WrapperState struct {
	Active   bool   `json:"active"`
	GameID   string `json:"gameId"`
	GameName string `json:"gameName"`
	// Background is the image behind the wrapper window: the game's hero or
	// wide artwork (BackgroundWide), else its cover; "" when it has none.
	Background     string `json:"background"`
	BackgroundWide bool   `json:"backgroundWide"`
	Status         Status `json:"status"`
}

// wrapperBackground picks the wrapper window's background image from the
// covers dir: the hero artwork (sharpest at full screen), else the wide grid,
// else the cover, skipping any whose file is missing.
func wrapperBackground(dir string, g library.Game) (path string, wide bool) {
	exists := func(name string) bool {
		if name == "" {
			return false
		}
		_, err := os.Stat(filepath.Join(dir, name))
		return err == nil
	}
	for _, kind := range []ArtKind{ArtHero, ArtWide} {
		if name := g.Art[string(kind)]; exists(name) {
			return filepath.Join(dir, name), true
		}
	}
	if exists(g.Cover) {
		return filepath.Join(dir, g.Cover), false
	}
	return "", false
}

// Wrapper returns wrapper-mode info (Active=false in launcher mode).
func (s *SessionService) Wrapper() WrapperState {
	if s.wrapGameID == "" {
		return WrapperState{}
	}
	w := WrapperState{Active: true, GameID: s.wrapGameID}
	if g, err := s.c.Library.Find(s.wrapGameID); err == nil {
		w.GameID, w.GameName = g.ID, g.Name
		w.Background, w.BackgroundWide = wrapperBackground(s.c.Dirs.CoversDir(), g)
	}
	s.mu.Lock()
	w.Status = s.status[w.GameID]
	s.mu.Unlock()
	return w
}
