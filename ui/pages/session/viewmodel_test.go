package session

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/akfaiz/carryover/internal/cloudsync/syncer"
	"github.com/akfaiz/carryover/services"
)

func syncing(msg string, beforePlay bool) services.Status {
	return services.Status{Phase: "sync", Message: msg, BeforePlay: beforePlay}
}

func TestASyncShowsProgressAndOffersCancel(t *testing.T) {
	vm := NewViewModel()
	ev := syncer.Event{Message: "Downloading", Bytes: 10, TotalBytes: 100}

	u := vm.Apply(syncing("Checking the cloud…", true), ev)

	assert.Equal(t, "Checking the cloud…", u.Message)
	assert.False(t, u.Failed)
	assert.True(t, u.Sync)
	assert.Equal(t, ev, u.Progress)
	require.NotNil(t, u.OfferCancel)
	assert.True(t, u.OfferCancel.BeforePlay, "cancelling this one cancels the launch")
	assert.False(t, u.HideCancel)
}

// Progress ticks of the same phase don't offer Cancel again: it may already
// have been pressed.
func TestTheSamePhaseOffersCancelOnce(t *testing.T) {
	vm := NewViewModel()
	assert.NotNil(t, vm.Apply(syncing("Uploading", false), syncer.Event{}).OfferCancel)
	assert.Nil(t, vm.Apply(syncing("Uploading", false), syncer.Event{Bytes: 5}).OfferCancel)
	assert.Nil(t, vm.Apply(syncing("Uploading", false), syncer.Event{Bytes: 9}).OfferCancel)
}

func TestANewPhaseOffersCancelAgain(t *testing.T) {
	vm := NewViewModel()
	vm.Apply(syncing("Syncing before playing", true), syncer.Event{})
	vm.Apply(services.Status{Phase: "playing", Message: "Playing"}, syncer.Event{})

	u := vm.Apply(syncing("Uploading saves", false), syncer.Event{})
	require.NotNil(t, u.OfferCancel, "the sync after the game is a new phase")
	assert.False(t, u.OfferCancel.BeforePlay)
}

func TestPlayingHidesProgressAndCancel(t *testing.T) {
	vm := NewViewModel()
	vm.Apply(syncing("Syncing", true), syncer.Event{})

	u := vm.Apply(services.Status{Phase: "playing", Message: "Playing Celeste"}, syncer.Event{})

	assert.Equal(t, "Playing Celeste", u.Message)
	assert.False(t, u.Sync, "the progress bar goes")
	assert.True(t, u.HideCancel)
	assert.Nil(t, u.OfferCancel)
}

func TestAnErrorIsShownAsOne(t *testing.T) {
	u := NewViewModel().Apply(services.Status{Phase: "error", Message: "offline"}, syncer.Event{})
	assert.True(t, u.Failed)
	assert.Equal(t, "offline", u.Message)
	assert.True(t, u.HideCancel)

	assert.False(t, NewViewModel().Apply(services.Status{Phase: "done", Message: "ok"}, syncer.Event{}).Failed)
}
