// Package services is the API the GUI uses on top of internal/core.
package services

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/akfaiz/playrelay/internal/library"
	"github.com/akfaiz/playrelay/internal/syncer"
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

// Bus delivers service events to subscribers (the GUI). Handlers run on the
// goroutine that raised the event; GUI code must hop to its UI thread.
type Bus struct {
	mu      sync.Mutex
	library []func(gameID string)
	status  []func(Status)
	prompt  []func(Prompt)
	closed  []func(promptID string)
	sync    []func(syncer.Event)
}

// NewBus creates an empty bus.
func NewBus() *Bus { return &Bus{} }

// OnLibrary subscribes to library changes (gameID is "" for global changes).
func (b *Bus) OnLibrary(f func(gameID string)) {
	b.mu.Lock()
	b.library = append(b.library, f)
	b.mu.Unlock()
}

// OnStatus subscribes to play-session status updates.
func (b *Bus) OnStatus(f func(Status)) { b.mu.Lock(); b.status = append(b.status, f); b.mu.Unlock() }

// OnPrompt subscribes to new prompts.
func (b *Bus) OnPrompt(f func(Prompt)) { b.mu.Lock(); b.prompt = append(b.prompt, f); b.mu.Unlock() }

// OnPromptClosed subscribes to prompts withdrawn without an answer (the
// sync that asked was cancelled); the UI should close them.
func (b *Bus) OnPromptClosed(f func(promptID string)) {
	b.mu.Lock()
	b.closed = append(b.closed, f)
	b.mu.Unlock()
}

// OnSync subscribes to sync progress events.
func (b *Bus) OnSync(f func(syncer.Event)) { b.mu.Lock(); b.sync = append(b.sync, f); b.mu.Unlock() }

func snapshot[T any](b *Bus, list *[]T) []T {
	b.mu.Lock()
	defer b.mu.Unlock()
	return append([]T(nil), *list...)
}

func (b *Bus) libraryChanged(gameID string) {
	if b == nil {
		return
	}
	for _, f := range snapshot(b, &b.library) {
		f(gameID)
	}
}

func (b *Bus) statusChanged(st Status) {
	if b == nil {
		return
	}
	for _, f := range snapshot(b, &b.status) {
		f(st)
	}
}

func (b *Bus) promptRaised(p Prompt) {
	if b == nil {
		return
	}
	for _, f := range snapshot(b, &b.prompt) {
		f(p)
	}
}

func (b *Bus) promptClosed(id string) {
	if b == nil {
		return
	}
	for _, f := range snapshot(b, &b.closed) {
		f(id)
	}
}

func (b *Bus) syncProgress(ev syncer.Event) {
	if b == nil {
		return
	}
	for _, f := range snapshot(b, &b.sync) {
		f(ev)
	}
}

// prompter holds outstanding prompts until the UI answers.
type prompter struct {
	mu      sync.Mutex
	pending map[string]chan string
	prompts map[string]Prompt
	onAsk   func()
	bus     *Bus
}

func newPrompter(bus *Bus) *prompter {
	return &prompter{pending: map[string]chan string{}, prompts: map[string]Prompt{}, bus: bus}
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
	p.bus.promptRaised(pr)
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
		p.bus.promptClosed(pr.ID)
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

// guiUI implements core.SessionUI using GUI prompts.
type guiUI struct {
	gameID       string
	p            *prompter
	onStatus     func(Status)
	onRunning    func(bool)
	onCancelable func(cancel func())
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

func (u *guiUI) SyncCancelable(cancel func()) {
	if u.onCancelable != nil {
		u.onCancelable(cancel)
	}
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
