package services

import (
	"context"
	"errors"
	"path/filepath"
	"runtime"
	"sync"

	"github.com/akfaiz/carryover/internal/core"
	"github.com/akfaiz/carryover/internal/syncer"
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

// IsRunning reports whether a session is active for the game.
func (s *SessionService) IsRunning(gameID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.running[gameID]
}

func (s *SessionService) setStatus(st Status) {
	s.mu.Lock()
	s.status[st.GameID] = st
	s.mu.Unlock()
	s.bus.statusChanged(st)
}

func (s *SessionService) play(ctx context.Context, gameID string, opts core.RunOptions) (core.SessionResult, error) {
	g, err := s.c.Library.Find(gameID)
	if err != nil {
		s.setStatus(Status{GameID: gameID, Phase: "error", Message: err.Error()})
		return core.SessionResult{}, err
	}
	s.mu.Lock()
	if s.running[g.ID] {
		s.mu.Unlock()
		return core.SessionResult{}, errors.New(g.Name + " is already running")
	}
	s.running[g.ID] = true
	s.mu.Unlock()
	s.bus.libraryChanged(g.ID)
	defer func() {
		s.mu.Lock()
		delete(s.running, g.ID)
		s.mu.Unlock()
		s.bus.libraryChanged(g.ID)
	}()

	ui := &guiUI{gameID: g.ID, p: s.p, onStatus: s.setStatus, onRunning: func(r bool) {
		if s.OnGameRunning != nil {
			s.OnGameRunning(r)
		}
	}, onCancelable: func(cancel func()) {
		s.mu.Lock()
		if s.cancels == nil {
			s.cancels = map[string]func(){}
		}
		if cancel == nil {
			delete(s.cancels, g.ID)
		} else {
			s.cancels[g.ID] = cancel
		}
		s.mu.Unlock()
	}}
	res, err := s.c.Play(ctx, g.ID, opts, ui)
	switch {
	case errors.Is(err, core.ErrCancelled):
		s.setStatus(Status{GameID: g.ID, Phase: "done", Message: "Cancelled"})
	case err != nil:
		s.setStatus(Status{GameID: g.ID, Phase: "error", Message: err.Error()})
	case errors.Is(res.PostError, core.ErrCancelled):
		s.setStatus(Status{GameID: g.ID, Phase: "done", Message: "Game closed — upload cancelled; your saves will upload on the next sync"})
	case res.PostError != nil:
		s.setStatus(Status{GameID: g.ID, Phase: "error", Message: "Game closed, but upload failed: " + res.PostError.Error()})
	case res.PostAction == syncer.ActionConflict:
		s.setStatus(Status{GameID: g.ID, Phase: "done", Message: "Game closed — save conflict left unresolved"})
	default:
		s.setStatus(Status{GameID: g.ID, Phase: "done", Message: "Game closed, saves synced"})
	}
	return res, err
}

// Play starts a game from the launcher. It returns once the game has started
// syncing; progress is reported via events.
func (s *SessionService) Play(gameID string) error {
	if s.IsRunning(gameID) {
		return errors.New("game is already running")
	}
	go func() { _, _ = s.play(context.Background(), gameID, core.RunOptions{}) }()
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
	Active    bool   `json:"active"`
	GameID    string `json:"gameId"`
	GameName  string `json:"gameName"`
	CoverPath string `json:"coverPath"`
	Status    Status `json:"status"`
}

// Wrapper returns wrapper-mode info (Active=false in launcher mode).
func (s *SessionService) Wrapper() WrapperState {
	if s.wrapGameID == "" {
		return WrapperState{}
	}
	w := WrapperState{Active: true, GameID: s.wrapGameID}
	if g, err := s.c.Library.Find(s.wrapGameID); err == nil {
		w.GameID, w.GameName = g.ID, g.Name
		if g.Cover != "" {
			w.CoverPath = filepath.Join(s.c.Dirs.CoversDir(), g.Cover)
		}
	}
	s.mu.Lock()
	w.Status = s.status[w.GameID]
	s.mu.Unlock()
	return w
}
