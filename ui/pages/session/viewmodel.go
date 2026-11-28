// Package session is the wrapper window's page (carryover run <game>): the
// small window a Steam shortcut shows while it syncs before the game, plays it
// and syncs after.
//
// ViewModel decides what a status update changes (the message, the progress
// bar, the Cancel button) and is tested without the toolkit; Page draws it.
package session

import (
	"github.com/akfaiz/carryover/internal/cloudsync/syncer"
	"github.com/akfaiz/carryover/services"
)

// Step is where the session is: syncing before the game, playing, uploading
// after it, or over.
type Step int

const (
	StepSync Step = iota
	StepPlay
	StepUpload
	StepDone
)

// Update is what the window shows after a status change.
type Update struct {
	Message string
	Msg     *services.Message // Message to translate
	Failed  bool              // the message is an error

	// Step is the step the session is at (on a failure, the one that failed).
	// Ended is set once nothing runs any more: done, cancelled or failed.
	Step  Step
	Ended bool

	// Sync is set during a sync phase: the progress bar shows Progress.
	Sync     bool
	Progress syncer.Event

	// OfferCancel is set when a new sync phase starts: Cancel is offered again,
	// and says what cancelling does. HideCancel is set outside a sync.
	OfferCancel *CancelOffer
	HideCancel  bool
}

// CancelOffer is a Cancel button for a sync phase.
type CancelOffer struct {
	// BeforePlay: cancelling the sync cancels the launch too.
	BeforePlay bool
}

// ViewModel tracks which sync phase the Cancel button belongs to, and the step
// the session has reached.
type ViewModel struct {
	phase string
	step  Step
}

// NewViewModel returns a view model for a window that has seen no status yet.
func NewViewModel() *ViewModel { return &ViewModel{} }

// Step is the step the session has reached.
func (vm *ViewModel) Step() Step { return vm.step }

// advance moves the step on with a status. A failure or a cancel stays at the
// step it happened in; a session that ends after the game has run is done.
func (vm *ViewModel) advance(st services.Status) {
	switch st.Phase {
	case "sync":
		if st.BeforePlay {
			vm.step = StepSync
		} else {
			vm.step = StepUpload
		}
	case "playing":
		vm.step = StepPlay
	case "done":
		if vm.step >= StepPlay {
			vm.step = StepDone
		}
	}
}

// Apply turns a status (and the transfer progress of the same game) into what
// the window shows.
func (vm *ViewModel) Apply(st services.Status, ev syncer.Event) Update {
	vm.advance(st)
	u := Update{Message: st.Message, Msg: st.Msg, Failed: st.Phase == "error", Step: vm.step}
	u.Ended = st.Phase == "done" || st.Phase == "error"
	if st.Phase != "sync" {
		vm.phase = ""
		u.HideCancel = true
		return u
	}
	u.Sync, u.Progress = true, ev
	if vm.phase != st.Message {
		// A new sync phase (before or after playing): offer Cancel again.
		vm.phase = st.Message
		u.OfferCancel = &CancelOffer{BeforePlay: st.BeforePlay}
	}
	return u
}
