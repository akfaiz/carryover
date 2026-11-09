// Package services exposes PlayRelay to the Wails frontend.
package services

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/wailsapp/wails/v3/pkg/application"

	"github.com/akfaiz/playrelay/internal/library"
	"github.com/akfaiz/playrelay/internal/syncer"
)

// Event names emitted to the frontend.
const (
	EventSync    = "sync:progress"
	EventStatus  = "session:status"
	EventPrompt  = "session:prompt"
	EventLibrary = "library:changed"
)

// Status is a play-session status update.
type Status struct {
	GameID  string `json:"gameId"`
	Phase   string `json:"phase"` // sync, playing, done, error
	Message string `json:"message"`
	Running bool   `json:"running"`
}

// ConflictInfo is shown when local and cloud saves both changed.
type ConflictInfo struct {
	Local  syncer.Summary `json:"local"`
	Remote syncer.Summary `json:"remote"`
}

// Prompt is a question for the user. Answer via SessionService.Respond.
type Prompt struct {
	ID         string        `json:"id"`
	Kind       string        `json:"kind"` // conflict, lock, syncError
	GameID     string        `json:"gameId"`
	GameName   string        `json:"gameName"`
	BeforePlay bool          `json:"beforePlay"`
	Conflict   *ConflictInfo `json:"conflict,omitempty"`
	Lock       *syncer.Lock  `json:"lock,omitempty"`
	Error      string        `json:"error,omitempty"`
}

func init() {
	application.RegisterEvent[syncer.Event](EventSync)
	application.RegisterEvent[Status](EventStatus)
	application.RegisterEvent[Prompt](EventPrompt)
	application.RegisterEvent[string](EventLibrary)
}

func emit(name string, data any) {
	if app := application.Get(); app != nil {
		app.Event.Emit(name, data)
	}
}

// prompter holds outstanding prompts until the frontend answers.
type prompter struct {
	mu      sync.Mutex
	pending map[string]chan string
	prompts map[string]Prompt
	onAsk   func()
}

func newPrompter() *prompter {
	return &prompter{pending: map[string]chan string{}, prompts: map[string]Prompt{}}
}

func (p *prompter) ask(ctx context.Context, pr Prompt) string {
	pr.ID = uuid.NewString()
	ch := make(chan string, 1)
	p.mu.Lock()
	p.pending[pr.ID] = ch
	p.prompts[pr.ID] = pr
	onAsk := p.onAsk
	p.mu.Unlock()
	if onAsk != nil {
		onAsk()
	}
	emit(EventPrompt, pr)
	defer func() {
		p.mu.Lock()
		delete(p.pending, pr.ID)
		delete(p.prompts, pr.ID)
		p.mu.Unlock()
	}()
	select {
	case a := <-ch:
		return a
	case <-ctx.Done():
		return ""
	}
}

func (p *prompter) respond(id, answer string) bool {
	p.mu.Lock()
	ch, ok := p.pending[id]
	p.mu.Unlock()
	if ok {
		ch <- answer
	}
	return ok
}

func (p *prompter) list() []Prompt {
	p.mu.Lock()
	defer p.mu.Unlock()
	out := make([]Prompt, 0, len(p.prompts))
	for _, pr := range p.prompts {
		out = append(out, pr)
	}
	return out
}

// guiUI implements core.SessionUI using frontend prompts.
type guiUI struct {
	gameID    string
	p         *prompter
	onStatus  func(Status)
	onRunning func(bool)
}

func (u *guiUI) Status(phase, message string) {
	u.onStatus(Status{GameID: u.gameID, Phase: phase, Message: message, Running: phase == "playing"})
}

func (u *guiUI) ResolveConflict(ctx context.Context, g library.Game, plan *syncer.Plan, before bool) syncer.Resolution {
	a := u.p.ask(ctx, Prompt{
		Kind: "conflict", GameID: g.ID, GameName: g.Name, BeforePlay: before,
		Conflict: &ConflictInfo{Local: plan.LocalSummary, Remote: plan.RemoteSummary},
	})
	switch a {
	case "local":
		return syncer.ResolveLocal
	case "cloud":
		return syncer.ResolveCloud
	}
	return syncer.ResolveAuto
}

func (u *guiUI) ConfirmLock(ctx context.Context, g library.Game, l *syncer.Lock) bool {
	return u.p.ask(ctx, Prompt{Kind: "lock", GameID: g.ID, GameName: g.Name, Lock: l, BeforePlay: true}) == "yes"
}

func (u *guiUI) ConfirmSyncFailure(ctx context.Context, g library.Game, err error, before bool) bool {
	return u.p.ask(ctx, Prompt{Kind: "syncError", GameID: g.ID, GameName: g.Name, Error: err.Error(), BeforePlay: before}) == "yes"
}

func (u *guiUI) GameRunning(running bool) {
	if u.onRunning != nil {
		u.onRunning(running)
	}
}

func fmtTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Local().Format(time.RFC3339)
}
