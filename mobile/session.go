package mobile

import (
	"errors"

	"github.com/akfaiz/carryover/internal/core"
)

// BeginOutcome is the result of BeginSession.
type BeginOutcome struct {
	core.SessionStart
	// Emulator is the profile played: an emulator id, "" for the game's own.
	Emulator string `json:"emulator"`
	// Cancelled: the user chose not to play (a conflict or another device's
	// lock was declined, or the sync was cancelled). Don't start the game.
	Cancelled bool `json:"cancelled"`
}

// BeginSession starts playing key (a game id, or a sync key for an emulator
// profile) with a game the app starts itself: it syncs before playing, takes
// the cloud lock and returns a BeginOutcome as JSON; then the app starts the
// game. Questions (a conflict, another device's lock, a failed sync) arrive
// as "prompt" events while it waits; answer them with Respond. emulator picks
// the profile ("" = the one in key, else the one played last). It fails while
// the game's previous session is still open (see SessionOpen). Once the game
// has closed, call EndSession.
func BeginSession(key, emulator string) (_ string, err error) {
	defer wire(&err)
	s, err := get()
	if err != nil {
		return "", err
	}
	start, err := s.sessions.Begin(key, emulator)
	_, emuID := core.ParseKey(start.Key)
	out := BeginOutcome{SessionStart: start, Emulator: emuID}
	if errors.Is(err, core.ErrCancelled) {
		out.Cancelled, err = true, nil
	}
	return encode(out, err)
}

// EndOutcome is the result of EndSession.
type EndOutcome struct {
	// Notice is what to tell the user (nil: nothing worth telling): the saves
	// were uploaded, both sides changed (a conflict, nothing transferred), or
	// the upload failed.
	Notice *core.Notice `json:"notice"`
	// Offline: the cloud couldn't be reached; the saves stay on this device
	// and the next sync uploads them.
	Offline bool `json:"offline"`
}

// EndSession finishes the session of key started with BeginSession, once the
// game has closed: it uploads the saves if only this device changed them,
// never downloads, releases the cloud lock and returns an EndOutcome as JSON.
func EndSession(key string) (_ string, err error) {
	defer wire(&err)
	s, err := get()
	if err != nil {
		return "", err
	}
	return endOutcome(s.sessions.End(key))
}

// EndPendingSession finishes a session PendingSessions listed (its game closed
// but the cloud couldn't be reached then), like EndSession. It does nothing
// when the session was finished meanwhile or its game was started again, so a
// retry in the background never ends a game being played.
func EndPendingSession(key string) (_ string, err error) {
	defer wire(&err)
	s, err := get()
	if err != nil {
		return "", err
	}
	return endOutcome(s.sessions.EndPending(key))
}

func endOutcome(notice *core.Notice, err error) (string, error) {
	out := EndOutcome{Notice: notice}
	if errors.Is(err, core.ErrOffline) {
		out.Offline, err = true, nil
	}
	return encode(out, err)
}

// SessionOpen reports whether key's game is being played in a session that
// BeginSession started and EndSession hasn't finished.
func SessionOpen(key string) bool {
	s, err := get()
	if err != nil {
		return false
	}
	return s.core.SessionOpen(s.core.CloudKeyOf(key))
}

// OpenSessions lists the sessions BeginSession started that EndSession hasn't
// finished ([]core.OpenSession) as JSON: after the app's process was killed
// while a game ran, the app ends those whose game is no longer in front.
func OpenSessions() (_ string, err error) {
	defer wire(&err)
	s, err := get()
	if err != nil {
		return "", err
	}
	open := s.core.OpenSessions()
	if open == nil {
		open = []core.OpenSession{}
	}
	return encode(open, nil)
}

// PendingSessions lists the sessions whose game closed but whose saves
// weren't uploaded (the cloud couldn't be reached: []core.PendingSession) as
// JSON. EndSession on their key finishes them once the network is back.
func PendingSessions() (_ string, err error) {
	defer wire(&err)
	s, err := get()
	if err != nil {
		return "", err
	}
	pending := s.core.PendingSessions()
	if pending == nil {
		pending = []core.PendingSession{}
	}
	return encode(pending, nil)
}

// Respond answers a prompt: for "conflict" "local", "cloud" or "skip"; for
// "lock" and "syncError" "yes" or "no"; for "emulator" a choice's id or
// "cancel". It reports whether the prompt was still waiting.
func Respond(promptID, answer string) bool {
	s, err := get()
	if err != nil {
		return false
	}
	return s.sessions.Respond(promptID, answer)
}

// Prompts lists the prompts waiting for an answer ([]services.Prompt) as JSON,
// for an app that starts while one is open.
func Prompts() (_ string, err error) {
	defer wire(&err)
	s, err := get()
	if err != nil {
		return "", err
	}
	return encode(s.sessions.Prompts(), nil)
}
