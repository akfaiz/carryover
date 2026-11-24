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

// Update is what the window shows after a status change.
type Update struct {
	Message string
	Failed  bool // the message is an error

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

// ViewModel tracks which sync phase the Cancel button belongs to.
type ViewModel struct {
	phase string
}

// NewViewModel returns a view model for a window that has seen no status yet.
func NewViewModel() *ViewModel { return &ViewModel{} }

// Apply turns a status (and the transfer progress of the same game) into what
// the window shows.
func (vm *ViewModel) Apply(st services.Status, ev syncer.Event) Update {
	u := Update{Message: st.Message, Failed: st.Phase == "error"}
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
